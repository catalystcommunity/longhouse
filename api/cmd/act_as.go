package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	rp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	lk "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
	"github.com/catalystcommunity/longhouse/api/internal/actas"
	"github.com/catalystcommunity/longhouse/api/internal/calendar"
	"github.com/catalystcommunity/longhouse/api/internal/config"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
	log "github.com/sirupsen/logrus"
)

type renewableActAs struct {
	*actas.Verifier
	keys calendar.Keys
	home lk.Transport
}

func (p *renewableActAs) RunKeyRenewal(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		if err := calendar.RenewKeys(ctx, p.home, p.keys); err != nil {
			log.WithError(err).Error("act-as application attestation renewal failed")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func buildActAsProvider() (actas.Provider, error) {
	if config.ActAsKeysFile == "" {
		return nil, nil
	}
	keys, err := calendar.ReadKeys(config.ActAsKeysFile)
	if err != nil {
		return nil, fmt.Errorf("act-as keys: %w", err)
	}
	rpConfig := linkkeys.TCPConfig{Domain: config.LinkkeysIDPDomain, Addr: config.LinkkeysTCPAddr, Fingerprints: strings.Split(config.LinkkeysTCPFingerprints, ","), APIKey: config.LinkkeysPKIAPIKey}
	transport, err := linkkeys.NewPinnedTransport(rpConfig)
	if err != nil {
		return nil, fmt.Errorf("act-as RP: %w", err)
	}
	if strings.TrimSpace(config.LinkkeysPKIAPIKey) == "" {
		return nil, fmt.Errorf("act-as requires a LinkKeys RP API key")
	}
	signers := make([]rp.ApplicationSigner, 0, len(keys.Keys))
	for _, key := range keys.Keys {
		seed, _ := base64.StdEncoding.DecodeString(key.Seed)
		signers = append(signers, rp.ApplicationSigner{KeyID: key.KeyID, Algorithm: "ed25519", PrivateKeyBytes: seed})
	}
	// Renew through the account's home domain. Reuse the configured endpoint
	// only when the RP domain is also the application's home domain.
	homeConfig := linkkeys.TCPConfig{Domain: keys.Application.SubjectDomain, APIKey: "application-signature"}
	if strings.EqualFold(keys.Application.SubjectDomain, config.LinkkeysIDPDomain) {
		homeConfig.Addr = rpConfig.Addr
		homeConfig.Fingerprints = rpConfig.Fingerprints
	}
	home, err := linkkeys.NewPinnedTransport(homeConfig)
	if err != nil {
		return nil, fmt.Errorf("act-as home domain: %w", err)
	}
	return &renewableActAs{keys: keys, home: home, Verifier: &actas.Verifier{Own: lk.ApplicationRef{SubjectDomain: strings.ToLower(keys.Application.SubjectDomain), SubjectUserId: keys.Application.SubjectUserId, ApplicationId: keys.Application.ApplicationId},
		InstanceID: keys.InstanceID, Signers: signers, Keys: actas.NewRPKeySource(transport)}}, nil
}
