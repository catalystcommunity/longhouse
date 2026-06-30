// Package csilrpc serves CSIL services over the canonical CSIL-RPC v1 carrier:
// a single endpoint that takes a CBOR CsilRpcRequest envelope and returns a
// CsilRpcResponse envelope (see envelope.go). Routing lives in the envelope
// (service/op), so the HTTP path is not semantic.
//
// Each operation is dispatched through a typed route (RegisterTyped) whose
// payload (de)serialization is owned by the generated csil codec; the dispatcher
// adds one layer of cross-cutting concerns — bearer verification, audit, and the
// declared ServiceError arm — around the typed service method. The browser-flow
// GET /api/v1/auth/start (a 302 navigation, not an RPC call) is the one exception
// that stays outside this dispatcher.
package csilrpc

import (
	"context"
	"errors"
	"net/http"

	"github.com/catalystcommunity/longhouse/api/internal/audit"
	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

// Dispatcher routes (service, op) to a registered typed handler. Construct with
// New, register methods with RegisterTyped or RegisterTypedPublic, then mount
// ServeRPC as an http.HandlerFunc.
//
// Methods registered with RegisterTyped require a valid bearer token; the
// dispatcher verifies it before invoking the handler and attaches the resulting
// auth.Identity to the request context (read it with auth.IdentityFromContext).
// Methods registered with RegisterTypedPublic skip bearer verification — used for
// the login methods that mint the bearer in the first place.
type Dispatcher struct {
	// typedRegistry[service][method] = typed handler over the generated codec.
	// Service segments are the lower-case CSIL service name; methods are the
	// PascalCase op.
	typedRegistry map[string]map[string]TypedHandler

	// publicMethods["service.Method"] = true means no bearer is required.
	// Defaults to false for every registration; flip via RegisterTypedPublic.
	publicMethods map[string]bool

	// jwtSecret is the HMAC secret for verifying bearers on non-public
	// methods. Nil means every non-public method fails closed with "auth not
	// configured" so a misconfigured deploy doesn't issue data without auth.
	jwtSecret []byte

	// auditRecorder, when set, receives an audit entry for every authenticated
	// mutation (and denied/failed attempt). Nil disables audit recording.
	// The auth/devauth services are skipped here — they emit their own
	// security events with per-house fan-out.
	auditRecorder audit.Recorder
}

// SetAuditRecorder wires the audit sink. Call once at startup. Safe to leave
// unset (audit recording is then a no-op) — used by tests that exercise just
// the dispatch surface.
func (d *Dispatcher) SetAuditRecorder(rec audit.Recorder) { d.auditRecorder = rec }

// New constructs an empty Dispatcher. Pass the JWT secret so non-public
// methods can verify bearers; pass nil only in tests that exercise just
// the public surface.
func New(jwtSecret []byte) *Dispatcher {
	return &Dispatcher{
		typedRegistry: map[string]map[string]TypedHandler{},
		publicMethods: map[string]bool{},
		jwtSecret:     jwtSecret,
	}
}

// RegisterTyped attaches a typed handler (generated codec + interface method) for
// the given service+method pair, gated on a valid bearer. Service names are the
// lower-case identifier (e.g. "task", "auth"); method names are PascalCase to
// match the generated client (e.g. "ListTasks"). Re-registering a method panics —
// duplicates almost always indicate a wiring bug we want to catch at startup.
func (d *Dispatcher) RegisterTyped(service, method string, h TypedHandler) {
	d.registerTyped(service, method, h)
}

// RegisterTypedPublic is RegisterTyped without bearer gating — reserved for the
// login methods that mint the bearer (AuthService.Login/Complete,
// DevAuthService.ListDevUsers/DevLogin) and dev/health probes.
func (d *Dispatcher) RegisterTypedPublic(service, method string, h TypedHandler) {
	d.registerTyped(service, method, h)
	d.publicMethods[service+"."+method] = true
}

func (d *Dispatcher) registerTyped(service, method string, h TypedHandler) {
	svc := d.typedRegistry[service]
	if svc == nil {
		svc = map[string]TypedHandler{}
		d.typedRegistry[service] = svc
	}
	if _, exists := svc[method]; exists {
		panic("csilrpc: duplicate typed registration for " + service + "." + method)
	}
	svc[method] = h
}

// emitAudit records an audit entry for a completed authenticated call. The
// auth/devauth services are skipped — they emit their own security events with
// per-house fan-out and unattributable-failure handling. Best-effort: a failed
// audit write is logged loudly but never affects the response the caller sees.
func (d *Dispatcher) emitAudit(ctx context.Context, service, method string, callErr error) {
	if d.auditRecorder == nil || service == "auth" || service == "devauth" {
		return
	}
	outcome := models.AuditOutcomeOK
	if callErr != nil {
		outcome = models.AuditOutcomeError
		var serr *Error
		if errors.As(callErr, &serr) && (serr.Code == http.StatusUnauthorized || serr.Code == http.StatusForbidden) {
			outcome = models.AuditOutcomeDenied
		}
	}
	if err := audit.Emit(ctx, d.auditRecorder, service, method, auth.IdentityFromContext(ctx), outcome); err != nil {
		log.WithFields(log.Fields{"service": service, "method": method}).
			WithError(err).Error("audit: record failed")
	}
}

// maxRequestBytes caps the request body. Generous (1 MiB) for any plausible
// CSIL payload; refuses larger requests so a malformed upload can't tie up the
// api reading forever.
const maxRequestBytes = 1 << 20
