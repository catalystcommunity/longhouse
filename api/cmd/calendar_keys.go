package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	cal "github.com/catalystcommunity/catalystcal/clients/go"
	rp "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go"
	bridge "github.com/catalystcommunity/longhouse/api/internal/calendar"
)

func calendarKeys(flags map[string]string) error {
	if flags["help"] == "true" {
		fmt.Println("longhouse calendar-keys --action=generate|enroll|renew --file=PATH\nGenerate: --domain=DOMAIN --user-id=UUID --instance-id=NAME [--application-id=longhouse]\nEnroll/renew: --tcp-addr=HOST:PORT --fingerprints=FP[,FP]\nEnroll also requires --account-api-key-file=PATH (the owner's home-domain API key).")
		return nil
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
	if flags["action"] != "enroll" && flags["action"] != "renew" {
		return errors.New("--action must be generate, enroll, or renew")
	}
	keys, err := bridge.ReadKeys(path)
	if err != nil {
		return err
	}
	credential := "application-signature"
	if flags["action"] == "enroll" {
		b, err := os.ReadFile(flags["account-api-key-file"])
		if err != nil {
			return errors.New("read the account API key from --account-api-key-file")
		}
		credential = strings.TrimSpace(string(b))
		if credential == "" {
			return errors.New("account API key file is empty")
		}
	}
	transport, err := rp.NewPinnedRpcTransport(rp.PinnedRpcTransportOptions{TCPAddress: flags["tcp-addr"], Fingerprints: strings.Split(flags["fingerprints"], ","), APIKey: credential})
	if err != nil {
		return err
	}
	if flags["action"] == "enroll" {
		return bridge.Enroll(context.Background(), transport, keys)
	}
	return bridge.RenewKeys(context.Background(), transport, keys)
}
