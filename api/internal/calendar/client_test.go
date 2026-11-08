package calendar

import (
	"context"
	"crypto/ed25519"
	"errors"
	wire "github.com/catalystcommunity/longhouse/api/internal/transport"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	rp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	lk "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
)

type callFunc func(context.Context, string, string, []byte) ([]byte, error)

func (f callFunc) Call(ctx context.Context, s, o string, b []byte) ([]byte, error) {
	return f(ctx, s, o, b)
}

func TestOccurrencesSplitAndDeduplicate(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	calls := 0
	transport := callFunc(func(_ context.Context, s, o string, b []byte) ([]byte, error) {
		if s != "EventService" || o != "list-events" {
			t.Fatal("wrong operation")
		}
		req, err := cal.DecodeListEventsRequest(b)
		if err != nil {
			t.Fatal(err)
		}
		if len(req.CalendarIds) != 1 || req.CalendarIds[0] != "house-calendar" || req.IncludeInvitations == nil || *req.IncludeInvitations {
			t.Fatal("house scope was not explicit")
		}
		a, _ := time.Parse(time.RFC3339, string(req.RangeStart))
		z, _ := time.Parse(time.RFC3339, string(req.RangeEnd))
		if z.Sub(a) > 400*24*time.Hour {
			t.Fatal("range exceeds catalystcal limit")
		}
		calls++
		if z.Sub(a) > 200*24*time.Hour {
			return cal.EncodeOccurrenceList(cal.OccurrenceList{Truncated: true}), nil
		}
		// One long event overlaps every subrange. It must occur once in the result.
		return cal.EncodeOccurrenceList(cal.OccurrenceList{Occurrences: []cal.Occurrence{{Access: "owner", Event: cal.Event{EventId: "span", CalendarId: "house-calendar"}, Start: cal.Timestamp(start.Format(time.RFC3339)), End: cal.Timestamp(start.AddDate(2, 0, 0).Format(time.RFC3339))}}}), nil
	})
	result, err := Occurrences(context.Background(), transport, "house-calendar", start, start.AddDate(2, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || calls < 4 {
		t.Fatalf("got %d rows after %d calls", len(result), calls)
	}
}
func TestOccurrencesRefuseForeignCalendarAndDenseInstant(t *testing.T) {
	start := time.Now().Truncate(time.Second)
	for _, tt := range []struct {
		name     string
		response cal.OccurrenceList
	}{
		{"foreign", cal.OccurrenceList{Occurrences: []cal.Occurrence{{Access: "owner", Event: cal.Event{EventId: "secret", CalendarId: "other"}}}}},
		{"dense", cal.OccurrenceList{Truncated: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			transport := callFunc(func(context.Context, string, string, []byte) ([]byte, error) {
				calls++
				return cal.EncodeOccurrenceList(tt.response), nil
			})
			_, err := Occurrences(context.Background(), transport, "house", start, start.Add(time.Second))
			if err == nil {
				t.Fatal("unsafe or incomplete calendar read succeeded")
			}
			if calls != 1 {
				t.Fatalf("dense read did not stop: %d", calls)
			}
		})
	}
}
func TestKeysArePrivateAndEnrollmentProvesEachKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	app := cal.ApplicationRef{SubjectDomain: "example.test", SubjectUserId: "owner", ApplicationId: "longhouse"}
	if err := GenerateKeys(path, app, "test"); err != nil {
		t.Fatal(err)
	}
	if err := GenerateKeys(path, app, "test"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing keys were not protected: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("private keys are readable by others")
	}
	keys, err := ReadKeys(path)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	challenges := 0
	transport := callFunc(func(_ context.Context, s, o string, b []byte) ([]byte, error) {
		switch o {
		case "start-key-challenge":
			req, err := lk.DecodeStartApplicationKeyChallengeRequest(b)
			if err != nil {
				t.Fatal(err)
			}
			if req.SubjectUserId != "owner" || req.Purpose != "add" || req.KeyUsage != "sign" || req.Algorithm != "ed25519" {
				t.Fatalf("wrong challenge %+v", req)
			}
			challenges++
			challenge := []byte("unique challenge for key " + string(req.PublicKey))
			return lk.EncodeStartApplicationKeyChallengeResponse(lk.StartApplicationKeyChallengeResponse{ChallengeId: "challenge", Challenge: &challenge, ExpiresAt: time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339)}), nil
		case "enroll-application-instance":
			if s != "Account" {
				t.Fatal("enrollment is not account-authenticated")
			}
			req, err := lk.DecodeEnrollApplicationInstanceRequest(b)
			if err != nil {
				t.Fatal(err)
			}
			if len(req.Keys) != 3 {
				t.Fatal("enrollment did not include three keys")
			}
			for _, signed := range req.Keys {
				addition, err := lk.DecodeApplicationKeyAddition(signed.Addition)
				if err != nil {
					t.Fatal(err)
				}
				if seen[addition.KeyId] {
					t.Fatal("duplicate signing key")
				}
				seen[addition.KeyId] = true
				if addition.SubjectDomain != "example.test" || addition.SubjectUserId != "owner" || addition.ApplicationId != "longhouse" || addition.InstanceId != "test" {
					t.Fatal("proof is not bound to application identity")
				}
				if signed.PossessionProof == nil || !ed25519.Verify(ed25519.PublicKey(addition.PublicKey), rp.PossessionSignatureInput(signed.Addition), *signed.PossessionProof) {
					t.Fatal("invalid possession proof")
				}
			}
			return lk.EncodeEnrollApplicationInstanceResponse(lk.EnrollApplicationInstanceResponse{SubjectDomain: "example.test", SubjectUserId: "owner", ApplicationId: "longhouse", InstanceId: "test", Attestations: make([]lk.SignedApplicationKeyAttestation, 3)}), nil
		default:
			t.Fatalf("unexpected op %s", o)
			return nil, nil
		}
	})
	if err = Enroll(context.Background(), transport, keys); err != nil {
		t.Fatal(err)
	}
	if challenges != 3 || len(seen) != 3 {
		t.Fatal("not every key proved possession")
	}
}

func TestBeginRequestsOnlyCalendarReadAndEventWrite(t *testing.T) {
	sentinel := errors.New("stop before signing")
	app := cal.ApplicationRef{SubjectDomain: "example.test", SubjectUserId: "account", ApplicationId: "longhouse"}
	c := Client{Keys: Keys{Application: app}, App: callFunc(func(_ context.Context, service, op string, b []byte) ([]byte, error) {
		if service != "AuthService" || op != "get-act-as-scope-set" {
			t.Fatalf("unexpected scope operation: %s/%s", service, op)
		}
		request, err := cal.DecodeScopeSetRequest(b)
		if err != nil {
			t.Fatal(err)
		}
		if request.GranteeDomain != app.SubjectDomain || request.GranteeUserId != app.SubjectUserId || request.GranteeApplicationId != app.ApplicationId {
			t.Fatal("wrong grantee")
		}
		if len(request.Scopes) != 2 || request.Scopes[0] != "calendars.read" || request.Scopes[1] != "events.write" {
			t.Fatalf("unexpected scopes: %v", request.Scopes)
		}
		return nil, sentinel
	})}
	if _, err := c.Begin(context.Background(), "nonce"); !errors.Is(err, sentinel) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemberReusesBearerButChecksGrantBeforeEveryWrite(t *testing.T) {
	audience := lk.ApplicationRef{SubjectDomain: "example.test", SubjectUserId: "calendar", ApplicationId: "catalystcal"}
	app := cal.ApplicationRef{SubjectDomain: "example.test", SubjectUserId: "account", ApplicationId: "longhouse"}
	exchanges, refreshes := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		request, err := wire.DecodeRpcRequest(b)
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if request.Op != "exchange-act-as-credential" {
			t.Errorf("unexpected op %s", request.Op)
			w.WriteHeader(400)
			return
		}
		exchanges++
		response := wire.NewRpcResponseOk("LoginResponse", cal.EncodeLoginResponse(cal.LoginResponse{Token: "test-bearer", ExpiresAt: cal.Timestamp(time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)), Via: "act_as_grant"}))
		out, _ := response.Encode()
		_, _ = w.Write(out)
	}))
	defer server.Close()
	var refusal error
	seed := make([]byte, ed25519.SeedSize)
	c := &Client{Config: Config{URL: server.URL, SubjectDomain: "example.test"}, Keys: Keys{Application: app},
		signer: rp.NewApplicationGranteeSigner("instance", rp.ApplicationSigner{KeyID: "key", Algorithm: "ed25519", PrivateKeyBytes: seed}),
		App: callFunc(func(_ context.Context, s, o string, b []byte) ([]byte, error) {
			return cal.EncodeApplicationIdentityResponse(cal.ApplicationIdentityResponse{Audience: cal.ApplicationRef{SubjectDomain: audience.SubjectDomain, SubjectUserId: audience.SubjectUserId, ApplicationId: audience.ApplicationId}}), nil
		}),
	}
	c.RP = callFunc(func(_ context.Context, s, o string, b []byte) ([]byte, error) {
		refreshes++
		if refusal != nil {
			return nil, refusal
		}
		req, err := lk.DecodeRpActAsRefreshRequest(b)
		if err != nil {
			return nil, err
		}
		refresh, err := lk.DecodeActAsRefreshRequest(req.Request.Request)
		if err != nil {
			return nil, err
		}
		grant := lk.ActAsGrant{GrantId: refresh.GrantId, UserId: "member", SubjectDomain: "example.test", Grantee: c.grantee(), Audience: audience, ApprovedScope: []string{"calendars.read", "events.write"}, IssuedAt: time.Now().UTC().Format(time.RFC3339), ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}
		return lk.EncodeRefreshActAsGrantResponse(lk.RefreshActAsGrantResponse{Grant: lk.SignedActAsGrant{Grant: lk.EncodeActAsGrant(grant)}}), nil
	})
	for i := 0; i < 3; i++ {
		if _, err := c.Member(context.Background(), "example.test", "member", "grant"); err != nil {
			t.Fatal(err)
		}
	}
	if exchanges != 1 || refreshes != 3 {
		t.Fatalf("exchanges=%d refreshes=%d", exchanges, refreshes)
	}
	refusal = &rp.ServerError{Status: 5, Message: "Too many authentication attempts. Wait and try again"}
	if _, err := c.Member(context.Background(), "example.test", "member", "grant"); err == nil || errors.Is(err, ErrGrantUnavailable) {
		t.Fatalf("rate limit treated as permanent denial: %v", err)
	}
	refusal = nil
	if _, err := c.Member(context.Background(), "example.test", "member", "grant"); err != nil {
		t.Fatal(err)
	}
	if exchanges != 1 {
		t.Fatal("rate limit discarded a valid bearer")
	}
	refusal = &rp.ServerError{Status: 4, Message: "the grant is revoked"}
	if _, err := c.Member(context.Background(), "example.test", "member", "grant"); !errors.Is(err, ErrGrantUnavailable) {
		t.Fatalf("revoked grant used cached bearer: %v", err)
	}
}
