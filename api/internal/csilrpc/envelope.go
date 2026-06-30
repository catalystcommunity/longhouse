package csilrpc

// This file adds the canonical CSIL-RPC v1 carrier alongside the legacy
// path-routed ServeHTTP in dispatcher.go. Both share the same registry and
// per-method handlers; only the wire layer differs.
//
//	POST /csil/v1/rpc
//	Content-Type: application/cbor
//	body  = CsilRpcRequest { v, service, op, payload: 24(CBOR(req)), ? id, ? auth }
//	reply = CsilRpcResponse { v, status, payload: 24(CBOR(res)), ? id, ? variant, ? error }
//
// Routing moves into the envelope: (service, op) self-route, so the HTTP path is
// not semantic. The envelope's `status` carries the transport outcome (registry
// in the conventions doc); a typed reply rides at status 0 with `variant` naming
// the chosen output arm. Application errors are NOT a transport status — a
// handler's *Error becomes the declared `ServiceError` arm at status 0. The HTTP
// status is always 200; the envelope status is the real outcome (matching the
// linkkeys carrier).

import (
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"unicode"

	"github.com/catalystcommunity/longhouse/api/internal/audit"
	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/transport"
	log "github.com/sirupsen/logrus"
)

// RPCMountPath is the canonical default carrier mount for CSIL-RPC v1.
const RPCMountPath = "/csil/v1/rpc"

// ServeRPC implements http.Handler for the canonical CSIL-RPC carrier. Only POST
// is accepted. Every reply is a CBOR CsilRpcResponse at HTTP 200; failures are
// reported via the envelope's status (transport) or a ServiceError arm
// (application), never via the HTTP status.
func (d *Dispatcher) ServeRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		d.writeRPC(w, nil, transport.NewRpcResponseTransportError(
			transport.StatusUnknownServiceOrOp, "only POST is supported on "+RPCMountPath))
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBytes))
	if err != nil {
		d.writeRPC(w, nil, transport.NewRpcResponseTransportError(
			transport.StatusMalformedEnvelope, "could not read request body: "+err.Error()))
		return
	}

	req, err := transport.DecodeRpcRequest(body)
	if err != nil {
		d.writeRPC(w, nil, transport.NewRpcResponseTransportError(
			transport.StatusMalformedEnvelope, err.Error()))
		return
	}
	id := req.ID // echoed on every response when present

	// Resolve (service, op) → typed handler. The envelope carries the canonical
	// CSIL names: a lower-case service segment matching the registry keys, and a
	// kebab-case op; the typed registry is keyed by the PascalCase method.
	method := methodFromOp(req.Op)
	handler := d.typedRegistry[req.Service][method]
	if handler == nil {
		d.writeRPC(w, id, transport.NewRpcResponseTransportError(
			transport.StatusUnknownServiceOrOp, "unknown service/op: "+req.Service+"/"+req.Op))
		return
	}

	ctx := r.Context()
	authenticated := !d.publicMethods[req.Service+"."+method]
	if authenticated {
		identity, authErr := d.verifyBearerRPC(r, &req)
		if authErr != nil {
			d.writeRPC(w, id, transport.NewRpcResponseTransportError(
				transport.StatusUnauthenticated, authErr.Message))
			return
		}
		ctx = auth.WithIdentity(ctx, identity)
		// Seed an audit draft for the handler to enrich; recorded after the call.
		ctx, _ = audit.WithDraft(ctx)
	}

	// The typed route returns the success-arm variant + the response payload
	// already encoded by the generated codec.
	variant, payload, callErr := handler(ctx, req.Payload)
	if authenticated {
		d.emitAudit(ctx, req.Service, method, callErr)
	}

	if callErr != nil {
		var serr *Error
		if errors.As(callErr, &serr) {
			// Application error → the declared ServiceError output arm at status 0.
			// Preserves the {code, message} contract the client already consumes.
			d.writeRPC(w, id, transport.NewRpcResponseOk("ServiceError",
				csil.EncodeServiceError(csil.ServiceError{
					Code:    uint64(serr.Code),
					Message: serr.Message,
				})))
			return
		}
		// Unstructured error → a real transport-level internal failure. Log the
		// detail; never leak it to the caller.
		log.WithFields(log.Fields{"service": req.Service, "op": req.Op}).
			WithError(callErr).Error("csilrpc: handler returned unstructured error")
		d.writeRPC(w, id, transport.NewRpcResponseTransportError(
			transport.StatusInternal, "internal error"))
		return
	}

	d.writeRPC(w, id, transport.NewRpcResponseOk(variant, payload))
}

// writeRPC encodes and writes a CsilRpcResponse envelope at HTTP 200. The
// envelope's status field is the real outcome; the HTTP layer only signals that
// an envelope is present.
func (d *Dispatcher) writeRPC(w http.ResponseWriter, id *uint64, resp transport.RpcResponse) {
	buf, err := resp.WithID(id).Encode()
	if err != nil {
		// Should never happen — the envelope codec is total over our inputs.
		log.WithError(err).Error("csilrpc: failed to encode CsilRpcResponse envelope")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/cbor")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf)
}

// verifyBearerRPC verifies the caller's JWT for the envelope carrier. The bearer
// stays transport-level: it is read from the Authorization header (the simplest
// correct move per the conventions, which allow auth outside the envelope), and
// as a convenience also accepted from the envelope's per-request `auth` field for
// callers that prefer in-band credentials. Returns an *Error on any failure so
// the caller can map it to a transport Unauthenticated status.
func (d *Dispatcher) verifyBearerRPC(r *http.Request, req *transport.RpcRequest) (*auth.Identity, *Error) {
	if d.jwtSecret == nil {
		return nil, internal("auth not configured on this server")
	}
	token := ""
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		token = strings.TrimPrefix(h, "Bearer ")
	} else if req.Auth != nil {
		// Accept a bare token or a "Bearer <token>" form, for symmetry with the header.
		token = strings.TrimPrefix(*req.Auth, "Bearer ")
	}
	if token == "" {
		return nil, unauthorized("missing bearer token")
	}
	id, err := auth.Verify(d.jwtSecret, token)
	if err != nil {
		return nil, unauthorized("invalid token: " + err.Error())
	}
	return id, nil
}

// methodFromOp converts a canonical kebab-case CSIL op ("list-tasks") to the
// PascalCase method name the registry is keyed by ("ListTasks"). It is the
// inverse of the generated client's method→op derivation. longhouse ops contain
// no digit or acronym runs, so the round trip is exact.
func methodFromOp(op string) string {
	var b strings.Builder
	b.Grow(len(op))
	upper := true
	for _, c := range op {
		if c == '-' {
			upper = true
			continue
		}
		if upper {
			b.WriteRune(unicode.ToUpper(c))
			upper = false
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// successVariant returns the CSIL output-arm name to put in a status-0 reply's
// `variant`. Every longhouse op is `Req -> Res / ServiceError`, so a status-0
// reply has multiple arms and MUST name the chosen one; the client only needs it
// to tell the success arm from the "ServiceError" arm. Slices (the `[* T]` arms)
// report their element type name; named structs report their own.
func successVariant(resp any) string {
	t := reflect.TypeOf(resp)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil {
		return "EmptyResponse"
	}
	if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		if name := t.Elem().Name(); name != "" {
			return name
		}
	}
	if name := t.Name(); name != "" {
		return name
	}
	return "Response"
}
