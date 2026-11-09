package calendar

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	rp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	lk "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
)

func applicationKeysFixture(t *testing.T) (Keys, lk.GetApplicationKeysResponse) {
	t.Helper()
	keys := Keys{Application: cal.ApplicationRef{SubjectDomain: "home.test", SubjectUserId: "account", ApplicationId: "longhouse"}, InstanceID: "instance"}
	response := lk.GetApplicationKeysResponse{SubjectDomain: "home.test", SubjectUserId: "account", ApplicationId: "longhouse", InstanceId: "instance"}
	for i := range 3 {
		seed := make([]byte, 32)
		seed[0] = byte(i + 1)
		key := Key{KeyID: string(rune('a' + i)), Seed: base64.StdEncoding.EncodeToString(seed)}
		keys.Keys = append(keys.Keys, key)
		_, pub, err := keySigner(key)
		if err != nil {
			t.Fatal(err)
		}
		expires := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
		attestation := lk.ApplicationKeyAttestation{SubjectDomain: "home.test", SubjectUserId: "account", ApplicationId: "longhouse", InstanceId: "instance", KeyId: key.KeyID, KeyUsage: "sign", Algorithm: "ed25519", PublicKey: pub, Fingerprint: rp.Fingerprint(pub), KeyExpiresAt: expires, AttestationExpiresAt: expires}
		response.Keys = append(response.Keys, lk.SignedApplicationKeyAttestation{Attestation: lk.EncodeApplicationKeyAttestation(attestation)})
	}
	return keys, response
}

func TestEnsureKeysRefusesTrustResetAndLookupFailure(t *testing.T) {
	for _, kind := range []string{"changed-key", "missing-key", "wrong-account", "revoked", "outage", "unrelated-not-found"} {
		t.Run(kind, func(t *testing.T) {
			keys, response := applicationKeysFixture(t)
			var lookupErr error
			switch kind {
			case "changed-key":
				keys.Keys[0].Seed = keys.Keys[1].Seed
			case "missing-key":
				response.Keys = response.Keys[1:]
			case "wrong-account":
				response.SubjectUserId = "other"
			case "revoked":
				response.Revocations = []lk.ApplicationKeyRevocation{{TargetKeyId: keys.Keys[0].KeyID}}
			case "outage":
				lookupErr = errors.New("network unavailable")
			case "unrelated-not-found":
				lookupErr = &rp.ServerError{Status: 1, Message: "unknown service"}
			}
			calls := 0
			transport := callFunc(func(_ context.Context, _, op string, _ []byte) ([]byte, error) {
				calls++
				if op != "get-application-keys" {
					t.Fatal("unexpected key mutation:", op)
				}
				if lookupErr != nil {
					return nil, lookupErr
				}
				return lk.EncodeGetApplicationKeysResponse(response), nil
			})
			if err := EnsureKeys(context.Background(), transport, keys); err == nil || calls != 1 {
				t.Fatalf("unsafe ensure: %v, %d calls", err, calls)
			}
		})
	}
}

func TestEnsureKeysRenewsExistingKeysWithoutEnrollment(t *testing.T) {
	keys, response := applicationKeysFixture(t)
	renewed := 0
	transport := callFunc(func(_ context.Context, _, op string, _ []byte) ([]byte, error) {
		switch op {
		case "get-application-keys":
			return lk.EncodeGetApplicationKeysResponse(response), nil
		case "start-key-challenge":
			challenge := []byte("test-challenge")
			return lk.EncodeStartApplicationKeyChallengeResponse(lk.StartApplicationKeyChallengeResponse{ChallengeId: "challenge", Challenge: &challenge, ExpiresAt: time.Now().Add(time.Minute).UTC().Format(time.RFC3339)}), nil
		case "renew-attestation":
			renewed++
			return lk.EncodeRenewApplicationKeyAttestationResponse(lk.RenewApplicationKeyAttestationResponse{}), nil
		default:
			t.Fatal("unexpected enrollment:", op)
			return nil, errors.New("unexpected operation")
		}
	})
	if err := EnsureKeys(context.Background(), transport, keys); err != nil {
		t.Fatal(err)
	}
	if renewed != 3 {
		t.Fatalf("renewed %d keys", renewed)
	}
}

func TestEnsureKeysEnrollsOnlyKnownMissingInstance(t *testing.T) {
	for _, status := range []int64{1, 2} {
		keys, response := applicationKeysFixture(t)
		exists := false
		enrolled := 0
		transport := callFunc(func(_ context.Context, _, op string, _ []byte) ([]byte, error) {
			switch op {
			case "get-application-keys":
				if !exists {
					return nil, &rp.ServerError{Status: status, Message: "unknown application instance"}
				}
				return lk.EncodeGetApplicationKeysResponse(response), nil
			case "start-key-challenge":
				challenge := []byte("test-challenge")
				return lk.EncodeStartApplicationKeyChallengeResponse(lk.StartApplicationKeyChallengeResponse{ChallengeId: "challenge", Challenge: &challenge, ExpiresAt: time.Now().Add(time.Minute).UTC().Format(time.RFC3339)}), nil
			case "enroll-application-instance":
				exists = true
				enrolled++
				return lk.EncodeEnrollApplicationInstanceResponse(lk.EnrollApplicationInstanceResponse{SubjectDomain: response.SubjectDomain, SubjectUserId: response.SubjectUserId, ApplicationId: response.ApplicationId, InstanceId: response.InstanceId, Attestations: response.Keys}), nil
			default:
				t.Fatal("unexpected operation:", op)
				return nil, errors.New("unexpected operation")
			}
		})
		if err := EnsureKeys(context.Background(), transport, keys); err != nil {
			t.Fatal(err)
		}
		if enrolled != 1 {
			t.Fatalf("enrolled %d times", enrolled)
		}
	}
}
