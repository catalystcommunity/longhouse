package csilrpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/transport"
)

func TestDelegatedDispatcher(t *testing.T) {
	for _, tc := range []struct {
		name, scope, service, op, method string
		verify                           bool
		verifyErr                        error
		want                             int
	}{
		{"allowed", "tasks.write", "task", "create-task", "CreateTask", true, nil, 0},
		{"wrong scope", "tasks.read", "task", "create-task", "CreateTask", true, nil, 403},
		{"no verifier", "tasks.write", "task", "create-task", "CreateTask", false, nil, 401},
		{"revoked", "tasks.write", "task", "create-task", "CreateTask", true, errors.New("revoked"), 401},
		{"no refresh escalation", "longhouse.full", "auth", "refresh", "Refresh", true, nil, 403},
		{"no CLI escalation", "longhouse.full", "auth", "approve-cli-login", "ApproveCliLogin", true, nil, 403},
		{"new operation denied", "longhouse.full", "task", "future-op", "FutureOp", true, nil, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secret := []byte("test")
			d := New(secret)
			called := false
			d.RegisterTyped(tc.service, tc.method, Route(func(context.Context, csil.EmptyRequest) (csil.EmptyResponse, error) {
				called = true
				return csil.EmptyResponse{}, nil
			}, csil.DecodeEmptyRequest, csil.EncodeEmptyResponse))
			if tc.verify {
				d.SetDelegationVerifier(func(context.Context, *auth.Identity) error {
					if tc.verifyErr != nil {
						return Unauthorized("revoked")
					}
					return nil
				})
			}
			token, err := auth.Mint(secret, auth.Identity{Delegation: &auth.Delegation{GrantID: "grant", Grantee: "app", Scopes: []string{tc.scope}}}, time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			req := transport.NewRpcRequest(tc.service, tc.op, csil.EncodeEmptyRequest(csil.EmptyRequest{}))
			// Exercise credentials in the envelope, too.
			req.Auth = &token
			resp := serveRPC(t, d, req, "")
			if tc.want == 0 {
				if !called || resp.Variant == nil || *resp.Variant == "ServiceError" {
					t.Fatalf("request failed: %+v", resp)
				}
				return
			}
			if called {
				t.Fatal("unauthorized handler ran")
			}
			failure, err := csil.DecodeServiceError(resp.Payload)
			if err != nil || int(failure.Code) != tc.want {
				t.Fatalf("got %+v, %v", failure, err)
			}
		})
	}
}
