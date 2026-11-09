package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	lk "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
	bridge "github.com/catalystcommunity/longhouse/api/internal/calendar"
	"github.com/catalystcommunity/longhouse/api/internal/linkkeys"
)

func calendarKeys(flags map[string]string) error {
	if flags["help"] == "true" {
		fmt.Println("longhouse application-keys --action=generate|ensure|enroll|renew --file=PATH\nGenerate: --domain=DOMAIN --user-id=UUID --instance-id=NAME [--application-id=longhouse]\nEnroll/renew: [--tcp-addr=HOST:PORT] [--fingerprints=FP[,FP]] (defaults to home-domain DNS)\nEnsure/enroll require --account-api-key-file=PATH (the owner's home-domain API key).\nInspect: --action=inspect --domain=DOMAIN --account-api-key-file=PATH (prints public account metadata).")
		return nil
	}
	if flags["action"] == "inspect" {
		credential, err := readAccountAPIKey(flags["account-api-key-file"])
		if err != nil {
			return err
		}
		transport, err := linkkeys.NewPinnedTransport(linkkeys.TCPConfig{Domain: flags["domain"], Addr: flags["tcp-addr"], Fingerprints: strings.Split(flags["fingerprints"], ","), APIKey: credential})
		if err != nil {
			return err
		}
		info, err := lk.NewAccountClient(transport).GetMyInfo(context.Background(), lk.EmptyRequest{})
		if err != nil {
			return fmt.Errorf("read application account: %w", err)
		}
		// Print only public account metadata, never claims or credentials.
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"user_id": info.User.Id, "username": info.User.Username, "display_name": info.User.DisplayName, "active": info.User.IsActive})
	}
	path := flags["file"]
	if path == "" {
		return errors.New("--file is required")
	}
	if flags["action"] == "generate" {
		app := flags["application-id"]
		if app == "" {
			app = "longhouse"
		}
		return bridge.GenerateKeys(path, cal.ApplicationRef{SubjectDomain: flags["domain"], SubjectUserId: flags["user-id"], ApplicationId: app}, flags["instance-id"])
	}
	if flags["action"] != "enroll" && flags["action"] != "ensure" && flags["action"] != "renew" {
		return errors.New("--action must be inspect, generate, ensure, enroll, or renew")
	}
	keys, err := bridge.ReadKeys(path)
	if err != nil {
		return err
	}
	credential := "application-signature"
	if flags["action"] == "enroll" || flags["action"] == "ensure" {
		credential, err = readAccountAPIKey(flags["account-api-key-file"])
		if err != nil {
			return err
		}
	}
	transport, err := linkkeys.NewPinnedTransport(linkkeys.TCPConfig{Domain: keys.Application.SubjectDomain, Addr: flags["tcp-addr"], Fingerprints: strings.Split(flags["fingerprints"], ","), APIKey: credential})
	if err != nil {
		return err
	}
	if flags["action"] == "ensure" {
		return bridge.EnsureKeys(context.Background(), transport, keys)
	}
	if flags["action"] == "enroll" {
		return bridge.Enroll(context.Background(), transport, keys)
	}
	return bridge.RenewKeys(context.Background(), transport, keys)
}

func readAccountAPIKey(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("read the account API key from --account-api-key-file")
	}
	credential := strings.TrimSpace(string(b))
	if credential == "" {
		return "", errors.New("account API key file is empty")
	}
	return credential, nil
}
