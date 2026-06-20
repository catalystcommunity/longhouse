package csilrpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/transport"
	"github.com/fxamacker/cbor/v2"
)

// serveRPC drives one envelope through ServeRPC and returns the decoded reply.
func serveRPC(t *testing.T, d *Dispatcher, req transport.RpcRequest, bearer string) transport.RpcResponse {
	t.Helper()
	frame, err := req.Encode()
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, RPCMountPath, strings.NewReader(string(frame)))
	r.Header.Set("Content-Type", "application/cbor")
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	d.ServeRPC(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status: got %d want 200 (envelope carries the real status)", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/cbor" {
		t.Fatalf("Content-Type: got %q want application/cbor", ct)
	}
	resp, err := transport.DecodeRpcResponse(w.Body.Bytes())
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func TestServeRPC_SuccessRoundTrip(t *testing.T) {
	d := New(nil)
	d.RegisterPublic("widget", "GetWidget", func(_ context.Context, body []byte) (any, error) {
		var in csil.HouseID
		if err := Decode(body, &in); err != nil {
			return nil, err
		}
		// Echo the request through to prove the payload survives tag-24 wrapping.
		return csil.BoolResponse{Value: string(in) == "h-1"}, nil
	})

	payload, err := cbor.Marshal(csil.HouseID("h-1"))
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	resp := serveRPC(t, d, transport.NewRpcRequest("widget", "get-widget", payload).WithID(7), "")

	if !resp.Status.IsOk() {
		t.Fatalf("status: got %v want ok", resp.Status.Name())
	}
	if resp.ID == nil || *resp.ID != 7 {
		t.Fatalf("id not echoed: got %v want 7", resp.ID)
	}
	if resp.Variant == nil || *resp.Variant != "BoolResponse" {
		t.Fatalf("variant: got %v want BoolResponse", resp.Variant)
	}
	var out csil.BoolResponse
	if err := cbor.Unmarshal(resp.Payload, &out); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if !out.Value {
		t.Fatalf("payload round-trip failed: got result=false")
	}
}

func TestServeRPC_ApplicationErrorIsServiceErrorArm(t *testing.T) {
	d := New(nil)
	d.RegisterPublic("widget", "GetWidget", func(_ context.Context, _ []byte) (any, error) {
		return nil, NotFound("no such widget")
	})

	resp := serveRPC(t, d, transport.NewRpcRequest("widget", "get-widget", []byte{}), "")

	// Application errors ride at status 0 with the ServiceError variant — never a
	// transport status.
	if !resp.Status.IsOk() {
		t.Fatalf("status: got %v want ok (application error is not a transport status)", resp.Status.Name())
	}
	if resp.Variant == nil || *resp.Variant != "ServiceError" {
		t.Fatalf("variant: got %v want ServiceError", resp.Variant)
	}
	var se csil.ServiceError
	if err := cbor.Unmarshal(resp.Payload, &se); err != nil {
		t.Fatalf("decode ServiceError: %v", err)
	}
	if se.Code != uint64(http.StatusNotFound) || se.Message != "no such widget" {
		t.Fatalf("ServiceError: got {%d, %q} want {404, \"no such widget\"}", se.Code, se.Message)
	}
}

func TestServeRPC_UnknownServiceOrOp(t *testing.T) {
	d := New(nil)
	resp := serveRPC(t, d, transport.NewRpcRequest("nope", "do-thing", []byte{}), "")
	if resp.Status.Code() != transport.StatusUnknownServiceOrOp.Code() {
		t.Fatalf("status: got %v want unknown-service-or-op", resp.Status.Name())
	}
}

func TestServeRPC_MalformedEnvelope(t *testing.T) {
	d := New(nil)
	r := httptest.NewRequest(http.MethodPost, RPCMountPath, strings.NewReader("not cbor"))
	w := httptest.NewRecorder()
	d.ServeRPC(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status: got %d want 200", w.Code)
	}
	resp, err := transport.DecodeRpcResponse(w.Body.Bytes())
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status.Code() != transport.StatusMalformedEnvelope.Code() {
		t.Fatalf("status: got %v want malformed-envelope", resp.Status.Name())
	}
}

func TestServeRPC_UnauthenticatedWhenNoBearer(t *testing.T) {
	d := New([]byte("secret"))
	d.Register("widget", "GetWidget", func(_ context.Context, _ []byte) (any, error) {
		return csil.EmptyResponse{}, nil
	})
	resp := serveRPC(t, d, transport.NewRpcRequest("widget", "get-widget", []byte{}), "")
	if resp.Status.Code() != transport.StatusUnauthenticated.Code() {
		t.Fatalf("status: got %v want unauthenticated", resp.Status.Name())
	}
}

func TestMethodFromOp(t *testing.T) {
	cases := map[string]string{
		"me":                        "Me",
		"login":                     "Login",
		"list-tasks":                "ListTasks",
		"dev-login":                 "DevLogin",
		"list-dev-users":            "ListDevUsers",
		"get-member-by-identity":    "GetMemberByIdentity",
		"set-project-task-position": "SetProjectTaskPosition",
	}
	for op, want := range cases {
		if got := methodFromOp(op); got != want {
			t.Errorf("methodFromOp(%q) = %q, want %q", op, got, want)
		}
	}
}
