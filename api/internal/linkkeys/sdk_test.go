package linkkeys

import (
	"context"
	"testing"

	lk "github.com/catalystcommunity/linkkeys/sdks/regular-rp/go/generated"
	"github.com/catalystcommunity/longhouse/api/internal/transport"
)

func TestPinnedSDKTransportUsesExistingTLSIdentity(t *testing.T) {
	_, cert, parsed := newTestCert(t)
	pin, err := spkiFingerprint(parsed)
	if err != nil {
		t.Fatal(err)
	}
	address := startTestRPCServer(t, cert, func(req *transport.RpcRequest) transport.HandlerOutcome {
		if req.Service != "Account" || req.Op != "get-my-info" || req.Auth == nil || *req.Auth != "test-api-key" {
			t.Error("wrong service or credential")
		}
		return transport.Reply("GetMyInfoResponse", lk.EncodeGetMyInfoResponse(lk.GetMyInfoResponse{User: lk.AdminUser{Id: "app-account", DisplayName: "Longhouse App", IsActive: true}}))
	})
	for _, badPin := range []bool{false, true} {
		selected := pin
		if badPin {
			selected = "0000000000000000000000000000000000000000000000000000000000000000"
		}
		client, err := NewPinnedTransport(TCPConfig{Addr: address, Fingerprints: []string{"", selected}, APIKey: "test-api-key"})
		if err != nil {
			t.Fatal(err)
		}
		info, err := lk.NewAccountClient(client).GetMyInfo(context.Background(), lk.EmptyRequest{})
		if badPin {
			if err == nil {
				t.Fatal("incorrect TLS pin accepted")
			}
			continue
		}
		if err != nil || info.User.Id != "app-account" {
			t.Fatalf("account lookup: %v", err)
		}
	}
	if _, err := NewPinnedTransport(TCPConfig{Insecure: true}); err == nil {
		t.Fatal("insecure application transport accepted")
	}
}
