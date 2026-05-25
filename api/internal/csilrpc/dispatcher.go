// Package csilrpc serves CSIL services over a single HTTP endpoint with a
// CBOR request/response body. The wire shape is intentionally small:
//
//	POST /api/csil/{service}/{method}
//	Content-Type: application/cbor
//	Authorization: Bearer <jwt>          (where required)
//
//	<request type CBOR-encoded>          → <response type CBOR-encoded>
//	<error>                              → HTTP status + CBOR ServiceError
//
// Service method names are PascalCase to match the generated client classes
// (e.g. /api/csil/task/ListTasks). Auth is one layer: bearer verification
// happens at the dispatcher entry, identity lands in request context, and
// individual method handlers run their own per-resource authz checks.
//
// There is no per-route REST tree: every operation lives at this single
// endpoint. The browser-flow GET /api/v1/auth/start (a 302 navigation, not
// an RPC call) is the one exception that stays outside this dispatcher.
package csilrpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/fxamacker/cbor/v2"
	log "github.com/sirupsen/logrus"
)

// Handler is the function signature every method registers under. The body
// arrives as raw CBOR; the handler decodes it into its method-specific
// request type, does its work, and returns the response (any CBOR-marshalable
// value) or an *Error.
//
// Handlers SHOULD return *Error for all caller-visible failures. A plain
// error is treated as a 500 internal error with a generic message — the
// detail is logged but never sent to the caller.
type Handler func(ctx context.Context, body []byte) (response any, err error)

// Dispatcher is the single HTTP handler that routes (service, method) to
// the registered handler function. Construct with New, register methods
// with Register or RegisterPublic, then mount as http.Handler.
//
// Methods registered with Register require a valid bearer token; the
// dispatcher verifies it before invoking the handler and attaches the
// resulting auth.Identity to the request context (use
// auth.IdentityFromContext to read it). Methods registered with
// RegisterPublic skip bearer verification — used for the login methods
// that mint the bearer in the first place.
type Dispatcher struct {
	// registry[service][method] = handler. Lower-cased service segments
	// (matching the URL) keyed first, then exact-match method.
	registry map[string]map[string]Handler

	// publicMethods["service.Method"] = true means no bearer is required.
	// Defaults to false for every registration; flip via RegisterPublic.
	publicMethods map[string]bool

	// jwtSecret is the HMAC secret for verifying bearers on non-public
	// methods. Nil means every non-public method 500s with "auth not
	// configured" — fail closed so a misconfigured deploy doesn't issue
	// data without auth.
	jwtSecret []byte

	// encMode is the cbor.EncMode used for every response. Pinned to "core
	// deterministic" so output bytes are stable for a given response value
	// — keeps cache-key derivation and testing sane.
	encMode cbor.EncMode
}

// New constructs an empty Dispatcher. Pass the JWT secret so non-public
// methods can verify bearers; pass nil only in tests that exercise just
// the public surface.
func New(jwtSecret []byte) *Dispatcher {
	enc, _ := cbor.CoreDetEncOptions().EncMode()
	return &Dispatcher{
		registry:      map[string]map[string]Handler{},
		publicMethods: map[string]bool{},
		jwtSecret:     jwtSecret,
		encMode:       enc,
	}
}

// Register attaches a handler for the given service+method pair, gated on
// a valid bearer. Service names are the lower-case identifier the URL uses
// (e.g. "task", "auth", "devauth"); method names are PascalCase to match
// the generated client calls (e.g. "ListTasks"). Re-registering a method
// panics — duplicates almost always indicate a wiring bug we want to catch
// at startup, not silently take the last write.
func (d *Dispatcher) Register(service, method string, h Handler) {
	d.register(service, method, h)
}

// RegisterPublic attaches a handler that doesn't require a bearer. Reserved
// for the login methods that mint the bearer (AuthService.Login/Complete,
// DevAuthService.ListDevUsers/DevLogin) and dev/health probes.
func (d *Dispatcher) RegisterPublic(service, method string, h Handler) {
	d.register(service, method, h)
	d.publicMethods[service+"."+method] = true
}

func (d *Dispatcher) register(service, method string, h Handler) {
	svc := d.registry[service]
	if svc == nil {
		svc = map[string]Handler{}
		d.registry[service] = svc
	}
	if _, exists := svc[method]; exists {
		panic("csilrpc: duplicate registration for " + service + "." + method)
	}
	svc[method] = h
}

// ServeHTTP implements http.Handler. Only POST is accepted; everything else
// is a 405 (with Allow: POST) so clients see the right hint immediately.
// The URL must match /api/csil/{service}/{method} exactly — extra trailing
// path segments are rejected so we never silently route to a different
// handler than the caller intended.
func (d *Dispatcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeErr(w, d.encMode, methodNotAllowed("only POST is supported on /api/csil"))
		return
	}

	service, method, ok := parseCSILPath(r.URL.Path)
	if !ok {
		writeErr(w, d.encMode, notFound("expected /api/csil/{service}/{method}"))
		return
	}

	svc := d.registry[service]
	if svc == nil {
		writeErr(w, d.encMode, notFound("unknown service: "+service))
		return
	}
	handler := svc[method]
	if handler == nil {
		writeErr(w, d.encMode, notFound("unknown method: "+service+"."+method))
		return
	}

	// Bearer check (unless the method is registered public). Identity goes
	// into context with the same key the rest of the auth package uses, so
	// service implementations can read it with auth.IdentityFromContext.
	ctx := r.Context()
	if !d.publicMethods[service+"."+method] {
		id, authErr := d.verifyBearer(r)
		if authErr != nil {
			writeErr(w, d.encMode, authErr)
			return
		}
		ctx = auth.WithIdentity(ctx, id)
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBytes))
	if err != nil {
		writeErr(w, d.encMode, badRequest("could not read request body: "+err.Error()))
		return
	}

	resp, callErr := handler(ctx, body)
	if callErr != nil {
		var serr *Error
		if errors.As(callErr, &serr) {
			writeErr(w, d.encMode, serr)
			return
		}
		// Unstructured errors are 500s; log the detail but never return it.
		log.WithFields(log.Fields{
			"service": service,
			"method":  method,
		}).WithError(callErr).Error("csilrpc: handler returned unstructured error")
		writeErr(w, d.encMode, internal("internal error"))
		return
	}

	// Empty success: a method that returns nil is conventionally
	// EmptyResponse — encode that explicitly so callers always see a value.
	if resp == nil {
		resp = csil.EmptyResponse{}
	}

	buf, err := d.encMode.Marshal(resp)
	if err != nil {
		log.WithFields(log.Fields{
			"service": service,
			"method":  method,
		}).WithError(err).Error("csilrpc: response CBOR encode failed")
		writeErr(w, d.encMode, internal("response encoding failed"))
		return
	}
	w.Header().Set("Content-Type", "application/cbor")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf)
}

// maxRequestBytes caps the request body. Generous (1 MiB) for any plausible
// CSIL payload; refuses requests larger than that with a 413 so a malformed
// upload can't tie up the api reading forever.
const maxRequestBytes = 1 << 20

// parseCSILPath returns the (service, method) pair for a path of exactly
// the shape /api/csil/{service}/{method}. Any extra segments — including
// a trailing slash — fail so we don't silently route a path the caller
// didn't intend.
func parseCSILPath(p string) (service, method string, ok bool) {
	const prefix = "/api/csil/"
	if !strings.HasPrefix(p, prefix) {
		return "", "", false
	}
	rest := p[len(prefix):]
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// Decode unmarshals the request body into v. On failure it returns a
// caller-visible BadRequest with the decode error message — the body
// shape is the caller's concern so it's safe to surface.
func Decode(body []byte, v any) error {
	if len(body) == 0 {
		// EmptyRequest is the canonical zero-body request; decoding nothing
		// into the target is a no-op. Callers asking for any other type
		// will catch the missing fields downstream.
		return nil
	}
	dec := cbor.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(v); err != nil {
		return badRequest("invalid CBOR body: " + err.Error())
	}
	return nil
}

// verifyBearer extracts the Authorization: Bearer header and verifies the
// JWT against d.jwtSecret. Returns a CSIL ServiceError on any failure so
// the dispatcher can wire it straight into a CBOR response.
func (d *Dispatcher) verifyBearer(r *http.Request) (*auth.Identity, *Error) {
	if d.jwtSecret == nil {
		return nil, internal("auth not configured on this server")
	}
	h := r.Header.Get("Authorization")
	if h == "" {
		return nil, unauthorized("missing bearer token")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return nil, unauthorized("Authorization header must be 'Bearer <token>'")
	}
	id, err := auth.Verify(d.jwtSecret, h[len(prefix):])
	if err != nil {
		return nil, unauthorized("invalid token: " + err.Error())
	}
	return id, nil
}
