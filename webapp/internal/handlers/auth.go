package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"

	"github.com/catalystcommunity/longhouse/webapp/internal/api"
	"github.com/catalystcommunity/longhouse/webapp/internal/session"
	log "github.com/sirupsen/logrus"
)

// login starts the OAuth-ish handshake: generate a fresh nonce, ask the PKI
// sidecar to sign an AuthRequest bound to our callback URL, stash the nonce in
// a short-lived cookie, and redirect the user to the IDP.
func (d *Deps) login(w http.ResponseWriter, r *http.Request) {
	if d.IDPURL == "" || d.IDPDomain == "" || d.CallbackURL == "" {
		renderLogin(w, http.StatusInternalServerError, "Login is not configured on this server.")
		return
	}

	nonce, err := newNonce()
	if err != nil {
		renderLogin(w, http.StatusInternalServerError, "Could not start login.")
		return
	}

	signedRequest, err := d.PKI.SignRequest(d.CallbackURL, nonce)
	if err != nil {
		log.WithError(err).Error("PKI sign-request failed")
		renderLogin(w, http.StatusBadGateway, "Could not reach identity service.")
		return
	}

	if err := d.Sessions.SetLoginState(w, session.LoginState{Nonce: nonce}); err != nil {
		log.WithError(err).Error("Failed to set login state cookie")
		renderLogin(w, http.StatusInternalServerError, "Could not start login.")
		return
	}

	// The IDP reads relying_party + callback_url + nonce from the verified
	// CBOR inside signed_request, so we don't pass them as URL parameters.
	// (Older linkkeys versions required them; the trust-the-CBOR change
	// landed in linkkeys >= 0.5.6.)
	q := url.Values{}
	q.Set("signed_request", signedRequest)
	http.Redirect(w, r, d.IDPURL+"/auth/authorize?"+q.Encode(), http.StatusFound)
}

// callback receives the encrypted_token from the IDP, decrypts+verifies via
// the PKI sidecar, sanity-checks the nonce + audience + domain, and issues
// the session cookie.
func (d *Deps) callback(w http.ResponseWriter, r *http.Request) {
	encrypted := r.URL.Query().Get("encrypted_token")
	if encrypted == "" {
		renderLogin(w, http.StatusBadRequest, "Missing token.")
		return
	}

	state, err := d.Sessions.ConsumeLoginState(w, r)
	if err != nil {
		renderLogin(w, http.StatusBadRequest, "Login session expired. Please try again.")
		return
	}

	signedAssertion, err := d.PKI.DecryptToken(encrypted)
	if err != nil {
		log.WithError(err).Error("PKI decrypt-token failed")
		renderLogin(w, http.StatusBadGateway, "Could not decrypt token.")
		return
	}

	assertion, err := d.PKI.VerifyAssertion(signedAssertion, d.IDPDomain)
	if err != nil {
		log.WithError(err).Error("PKI verify-assertion failed")
		renderLogin(w, http.StatusUnauthorized, "Identity could not be verified.")
		return
	}

	if assertion.Nonce != state.Nonce {
		renderLogin(w, http.StatusUnauthorized, "Login nonce mismatch.")
		return
	}
	if assertion.Domain != d.IDPDomain {
		renderLogin(w, http.StatusUnauthorized, "Assertion from unexpected domain.")
		return
	}
	// In our deployment topology the RP's identity domain equals the IDP's
	// identity domain (single linkkeys instance serving both roles), so
	// the assertion's audience should match d.IDPDomain. If a future
	// deployment splits IDP and RP across different identity domains,
	// this check needs a separate config knob.
	if assertion.Audience != "" && assertion.Audience != d.IDPDomain {
		renderLogin(w, http.StatusUnauthorized, "Assertion audience mismatch.")
		return
	}

	// Exchange the verified assertion for an api bearer token. The webapp
	// is just one client of the api; the token authenticates subsequent
	// /api/* calls regardless of who's calling.
	loginResp, err := d.API.Login(signedAssertion, nil)
	if err != nil {
		var multi *api.MultiHouseError
		if errors.As(err, &multi) {
			// Stash the verified assertion + show a picker. The user
			// re-submits with their chosen house_id at /auth/pick-house.
			if perr := d.Sessions.SetPendingHousePick(w, session.PendingHousePick{
				SignedAssertion: signedAssertion,
			}); perr != nil {
				log.WithError(perr).Error("Failed to set pending-house-pick cookie")
				renderLogin(w, http.StatusInternalServerError, "Could not start house selection.")
				return
			}
			renderHousePicker(w, multi.Houses, "")
			return
		}
		log.WithError(err).Error("api login failed")
		renderLogin(w, http.StatusBadGateway, "Could not exchange identity for a session.")
		return
	}

	if err := d.Sessions.SetIdentity(w, session.Identity{
		Domain:      assertion.Domain,
		UserID:      assertion.UserID,
		DisplayName: assertion.DisplayName,
		MemberID:    loginResp.MemberID,
		HouseID:     loginResp.HouseID,
		APIToken:    loginResp.Token,
		Roles:       loginResp.Roles,
	}); err != nil {
		log.WithError(err).Error("Failed to set session cookie")
		renderLogin(w, http.StatusInternalServerError, "Could not complete login.")
		return
	}

	http.Redirect(w, r, "/app/dashboard", http.StatusFound)
}

func (d *Deps) logout(w http.ResponseWriter, r *http.Request) {
	d.Sessions.ClearIdentity(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// pickHouse handles the picker form: pulls the pending signed assertion
// from the cookie, re-calls api.Login with the chosen house_id, sets the
// session, and redirects to the dashboard.
func (d *Deps) pickHouse(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	houseID := r.FormValue("house_id")
	if houseID == "" {
		renderLogin(w, http.StatusBadRequest, "Pick a house to continue.")
		return
	}
	pending, err := d.Sessions.ConsumePendingHousePick(w, r)
	if err != nil {
		renderLogin(w, http.StatusBadRequest, "House selection expired. Please sign in again.")
		return
	}
	loginResp, err := d.API.Login(pending.SignedAssertion, &houseID)
	if err != nil {
		log.WithError(err).Error("api login (with house) failed")
		renderLogin(w, http.StatusBadGateway, "Could not log in to that house.")
		return
	}

	// We don't have the original linkkeys assertion here (only the signed
	// blob), so identity domain/user_id come from the LoginResponse-related
	// fields on the api side. The webapp's session keeps the same fields
	// as before; for picker flows we only have what /auth/login returned.
	if err := d.Sessions.SetIdentity(w, session.Identity{
		MemberID: loginResp.MemberID,
		HouseID:  loginResp.HouseID,
		APIToken: loginResp.Token,
		Roles:    loginResp.Roles,
	}); err != nil {
		log.WithError(err).Error("Failed to set session cookie after picker")
		renderLogin(w, http.StatusInternalServerError, "Could not complete login.")
		return
	}
	http.Redirect(w, r, "/app/dashboard", http.StatusFound)
}

// renderHousePicker draws the house-picker template with the supplied
// list. errMsg is rendered above the picker if non-empty.
func renderHousePicker(w http.ResponseWriter, houses []api.HouseChoice, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusConflict)
	if err := standalones.ExecuteTemplate(w, "house_picker.html", map[string]any{
		"Title":  "Pick a house",
		"Houses": houses,
		"Error":  errMsg,
	}); err != nil {
		log.WithError(err).Error("Failed to render house picker")
	}
}

func newNonce() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
