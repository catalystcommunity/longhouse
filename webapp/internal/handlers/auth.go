package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/url"

	"github.com/catalystcommunity/longhouse/webapp/internal/session"
	log "github.com/sirupsen/logrus"
)

// login starts the OAuth-ish handshake: generate a fresh nonce, ask the PKI
// sidecar to sign an AuthRequest bound to our callback URL, stash the nonce in
// a short-lived cookie, and redirect the user to the IDP.
func (d *Deps) login(w http.ResponseWriter, r *http.Request) {
	if d.IDPURL == "" || d.RPDomain == "" || d.CallbackURL == "" {
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

	q := url.Values{}
	q.Set("callback_url", d.CallbackURL)
	q.Set("nonce", nonce)
	q.Set("relying_party", d.RPDomain)
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
	if assertion.Audience != "" && assertion.Audience != d.RPDomain {
		renderLogin(w, http.StatusUnauthorized, "Assertion audience mismatch.")
		return
	}

	if err := d.Sessions.SetIdentity(w, session.Identity{
		Domain:      assertion.Domain,
		UserID:      assertion.UserID,
		DisplayName: assertion.DisplayName,
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

func newNonce() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
