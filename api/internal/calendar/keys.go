package calendar

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	rp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	lk "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
	log "github.com/sirupsen/logrus"
)

func GenerateKeys(path string, application cal.ApplicationRef, instance string) error {
	if application.SubjectDomain == "" || application.SubjectUserId == "" || application.ApplicationId == "" || instance == "" {
		return errors.New("domain, user-id, application-id and instance-id are required")
	}
	keys := Keys{Application: application, InstanceID: instance}
	for i := 0; i < 3; i++ {
		_, seed, _, err := rp.NewSigningKeyPair()
		if err != nil {
			return err
		}
		keys.Keys = append(keys.Keys, Key{KeyID: fmt.Sprintf("sign-%d", i+1), Seed: base64.StdEncoding.EncodeToString(seed[:])})
	}
	b, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err = file.Write(append(b, '\n')); err != nil {
		return err
	}
	return file.Sync()
}
func ReadKeys(path string) (Keys, error) {
	var keys Keys
	b, err := os.ReadFile(path)
	if err != nil {
		return keys, err
	}
	if err = json.Unmarshal(b, &keys); err != nil {
		return keys, errors.New("invalid calendar application keys file")
	}
	if len(keys.Keys) != 3 || keys.Application.SubjectUserId == "" || keys.Application.SubjectDomain == "" || keys.Application.ApplicationId == "" || keys.InstanceID == "" {
		return keys, errors.New("calendar application requires an identity, instance and three keys")
	}
	seen := map[string]bool{}
	seeds := map[string]bool{}
	for _, key := range keys.Keys {
		seed, err := base64.StdEncoding.DecodeString(key.Seed)
		if err != nil || len(seed) != ed25519.SeedSize || key.KeyID == "" || seen[key.KeyID] || seeds[key.Seed] {
			return keys, errors.New("invalid or duplicate calendar application key")
		}
		seen[key.KeyID] = true
		seeds[key.Seed] = true
	}
	return keys, nil
}
func keySigner(key Key) (rp.ApplicationSigner, ed25519.PublicKey, error) {
	seed, err := base64.StdEncoding.DecodeString(key.Seed)
	if err != nil || len(seed) != ed25519.SeedSize {
		return rp.ApplicationSigner{}, nil, errors.New("invalid key seed")
	}
	private := ed25519.NewKeyFromSeed(seed)
	return rp.ApplicationSigner{KeyID: key.KeyID, Algorithm: "ed25519", PrivateKeyBytes: seed}, private.Public().(ed25519.PublicKey), nil
}

// Enroll sends possession proofs through the owner's authenticated account.
// The caller supplies the home-domain transport, not a remote RP proxy.
func Enroll(ctx context.Context, transport lk.Transport, keys Keys) error {
	client := lk.NewApplicationKeysClient(transport)
	signed := []lk.SignedApplicationKeyAddition{}
	for _, key := range keys.Keys {
		signer, pub, err := keySigner(key)
		if err != nil {
			return err
		}
		challenge, err := client.StartKeyChallenge(ctx, lk.StartApplicationKeyChallengeRequest{SubjectUserId: keys.Application.SubjectUserId, ApplicationId: keys.Application.ApplicationId, InstanceId: keys.InstanceID, Purpose: "add", KeyUsage: "sign", Algorithm: "ed25519", PublicKey: pub})
		if err != nil {
			return err
		}
		if challenge.Challenge == nil {
			return errors.New("home domain did not return a signing challenge")
		}
		now := time.Now().UTC()
		addition, err := rp.SignAddition(lk.ApplicationKeyAddition{SubjectUserId: keys.Application.SubjectUserId, SubjectDomain: keys.Application.SubjectDomain, ApplicationId: keys.Application.ApplicationId, InstanceId: keys.InstanceID, KeyId: key.KeyID, KeyUsage: "sign", Algorithm: "ed25519", PublicKey: pub, Fingerprint: rp.Fingerprint(pub), RequestedKeyLifetimeSeconds: 365 * 24 * 60 * 60, ChallengeId: challenge.ChallengeId, Challenge: *challenge.Challenge, RequestedAt: now.Format(time.RFC3339), ExpiresAt: challenge.ExpiresAt}, nil, &signer)
		if err != nil {
			return err
		}
		signed = append(signed, addition)
	}
	response, err := lk.NewAccountClient(transport).EnrollApplicationInstance(ctx, lk.EnrollApplicationInstanceRequest{ApplicationId: keys.Application.ApplicationId, InstanceId: keys.InstanceID, Keys: signed})
	if err != nil {
		return err
	}
	if response.SubjectDomain != keys.Application.SubjectDomain || response.SubjectUserId != keys.Application.SubjectUserId || response.ApplicationId != keys.Application.ApplicationId || response.InstanceId != keys.InstanceID || len(response.Attestations) != 3 {
		return errors.New("home domain returned another application identity")
	}
	return nil
}
func RenewKeys(ctx context.Context, transport lk.Transport, keys Keys) error {
	client := lk.NewApplicationKeysClient(transport)
	for _, key := range keys.Keys {
		signer, pub, err := keySigner(key)
		if err != nil {
			return err
		}
		challenge, err := client.StartKeyChallenge(ctx, lk.StartApplicationKeyChallengeRequest{SubjectUserId: keys.Application.SubjectUserId, ApplicationId: keys.Application.ApplicationId, InstanceId: keys.InstanceID, Purpose: "renew", KeyUsage: "sign", Algorithm: "ed25519", PublicKey: pub})
		if err != nil {
			return err
		}
		if challenge.Challenge == nil {
			return errors.New("home domain did not return a signing challenge")
		}
		request, err := rp.SignRenewal(lk.ApplicationKeyRenewal{SubjectUserId: keys.Application.SubjectUserId, SubjectDomain: keys.Application.SubjectDomain, ApplicationId: keys.Application.ApplicationId, InstanceId: keys.InstanceID, KeyId: key.KeyID, ChallengeId: challenge.ChallengeId, Challenge: *challenge.Challenge, RequestedAt: time.Now().UTC().Format(time.RFC3339), ExpiresAt: challenge.ExpiresAt}, nil, &signer)
		if err != nil {
			return err
		}
		if _, err = client.RenewAttestation(ctx, lk.RenewApplicationKeyAttestationRequest{Request: request}); err != nil {
			return err
		}
	}
	return nil
}
func (c *Client) RunKeyRenewal(ctx context.Context) {
	tick := time.NewTicker(6 * time.Hour)
	defer tick.Stop()
	for {
		if err := RenewKeys(ctx, c.Home, c.Keys); err != nil {
			log.WithError(err).Error("calendar application attestation renewal failed")
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

// EnsureKeys enrolls a missing instance. It never resets an existing instance.
func EnsureKeys(ctx context.Context, transport lk.Transport, keys Keys) error {
	response, err := lk.NewApplicationKeysClient(transport).GetApplicationKeys(ctx, lk.GetApplicationKeysRequest{SubjectUserId: keys.Application.SubjectUserId, ApplicationId: keys.Application.ApplicationId, InstanceId: keys.InstanceID})
	if err != nil {
		// Released LinkKeys carriers use status 1 or 2 for this service's 404.
		var remote *rp.ServerError
		if !errors.As(err, &remote) || (remote.Status != 1 && remote.Status != 2) || remote.Message != "unknown application instance" {
			return err
		}
		if err := Enroll(ctx, transport, keys); err != nil {
			return err
		}
	} else {
		if err := checkApplicationKeys(response, keys, false); err != nil {
			return err
		}
		if err := RenewKeys(ctx, transport, keys); err != nil {
			return err
		}
	}
	response, err = lk.NewApplicationKeysClient(transport).GetApplicationKeys(ctx, lk.GetApplicationKeysRequest{SubjectUserId: keys.Application.SubjectUserId, ApplicationId: keys.Application.ApplicationId, InstanceId: keys.InstanceID})
	if err != nil {
		return err
	}
	return checkApplicationKeys(response, keys, true)
}

func checkApplicationKeys(response lk.GetApplicationKeysResponse, keys Keys, current bool) error {
	if response.SubjectDomain != keys.Application.SubjectDomain || response.SubjectUserId != keys.Application.SubjectUserId || response.ApplicationId != keys.Application.ApplicationId || response.InstanceId != keys.InstanceID {
		return errors.New("home domain returned another application identity")
	}
	for _, key := range keys.Keys {
		_, pub, err := keySigner(key)
		if err != nil {
			return err
		}
		for _, revocation := range response.Revocations {
			if revocation.TargetKeyId == key.KeyID {
				return errors.New("configured application key is revoked")
			}
		}
		matched := false
		for _, signed := range response.Keys {
			attestation, err := lk.DecodeApplicationKeyAttestation(signed.Attestation)
			if err != nil {
				return errors.New("invalid application attestation")
			}
			if attestation.KeyId != key.KeyID {
				continue
			}
			if attestation.SubjectDomain != keys.Application.SubjectDomain || attestation.SubjectUserId != keys.Application.SubjectUserId || attestation.ApplicationId != keys.Application.ApplicationId || attestation.InstanceId != keys.InstanceID || attestation.Fingerprint != rp.Fingerprint(pub) || !bytes.Equal(attestation.PublicKey, pub) || attestation.KeyUsage != "sign" || attestation.Algorithm != "ed25519" {
				return errors.New("existing application keys differ; refusing a trust reset")
			}
			if current {
				for _, raw := range []string{attestation.KeyExpiresAt, attestation.AttestationExpiresAt} {
					expires, err := time.Parse(time.RFC3339, raw)
					if err != nil || !expires.After(time.Now()) {
						return errors.New("application key attestation is expired")
					}
				}
			}
			matched = true
		}
		if !matched {
			return errors.New("existing application key is missing; refusing a trust reset")
		}
	}
	return nil
}
