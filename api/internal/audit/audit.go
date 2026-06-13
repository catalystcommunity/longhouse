// Package audit records mutations and security events to the partitioned
// audit_log. The write spine is the CSIL dispatcher: it seeds a *Draft into the
// request context before the handler runs, the handler enriches it via
// Annotate (resource id, house, before/after snapshot), and the dispatcher
// calls Emit after the handler returns. Handlers that forget to annotate still
// get a coarse entry for any mutation method, so the log never silently misses
// a write. Reads (Get*/List*/Me) are skipped.
//
// This package depends only on models and auth, so the dispatcher (csilrpc) can
// import it without a cycle.
package audit

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
	log "github.com/sirupsen/logrus"
)

// Recorder is the sink for audit entries. *store.PostgresStore (and the Store
// interface) satisfy it structurally.
type Recorder interface {
	RecordAuditEntry(ctx context.Context, e *models.AuditEntry) error
}

// Draft is the per-request, handler-enriched audit detail. The dispatcher seeds
// an empty one; handlers fill it in via Annotate.
type Draft struct {
	HouseID      string
	ResourceType string
	ResourceID   string
	Action       string // overrides the method-derived action when set
	Before       any
	After        any
	Detail       map[string]any
	annotated    bool
}

type ctxKey struct{}

// WithDraft returns a context carrying a fresh empty Draft and the Draft
// pointer (so the dispatcher can read it back after the handler runs).
func WithDraft(ctx context.Context) (context.Context, *Draft) {
	d := &Draft{}
	return context.WithValue(ctx, ctxKey{}, d), d
}

// FromContext returns the Draft seeded by the dispatcher, or nil.
func FromContext(ctx context.Context) *Draft {
	d, _ := ctx.Value(ctxKey{}).(*Draft)
	return d
}

// Annotate lets a handler enrich the audit entry for the current request. A
// no-op if no Draft is in context (e.g. unit tests calling handlers directly).
func Annotate(ctx context.Context, fn func(*Draft)) {
	if d := FromContext(ctx); d != nil {
		fn(d)
		d.annotated = true
	}
}

// Emit builds and records the audit entry for a completed call. Best-effort:
// it returns an error for the caller to log but should never block the
// response. It skips reads and unannotated non-mutations.
func Emit(ctx context.Context, rec Recorder, service, method string, id *auth.Identity, outcome string) error {
	if rec == nil {
		return nil
	}
	d := FromContext(ctx)
	action := ""
	if d != nil && d.Action != "" {
		action = d.Action
	} else {
		action = mutationAction(method)
	}
	// Nothing to record: a read that the handler didn't explicitly annotate.
	if action == "" && (d == nil || !d.annotated) {
		return nil
	}
	if action == "" {
		action = models.AuditActionUpdate // annotated mutation with no verb hint
	}

	e := &models.AuditEntry{
		Service: service,
		Method:  method,
		Action:  action,
		Outcome: outcome,
	}
	if id != nil {
		e.ActorDomain = id.Domain
		e.ActorUserID = id.UserID
	}
	if d != nil {
		if d.HouseID != "" {
			e.HouseID = strPtr(d.HouseID)
			if id != nil {
				if hr := id.House(d.HouseID); hr != nil && hr.Member != "" {
					e.ActorMemberID = strPtr(hr.Member)
				}
			}
		}
		if d.ResourceType != "" {
			e.ResourceType = strPtr(d.ResourceType)
		}
		if d.ResourceID != "" {
			e.ResourceID = strPtr(d.ResourceID)
		}
		e.Before = toJSONMap(d.Before)
		e.After = toJSONMap(d.After)
		if len(d.Detail) > 0 {
			e.Detail = models.JSONMap(d.Detail)
		}
	}
	return rec.RecordAuditEntry(ctx, e)
}

// mutationAction maps a CSIL method name to an audit action, or "" for reads.
// Get*/List* and the bare "Me" are reads. Everything else is a write whose verb
// is inferred from the prefix.
func mutationAction(method string) string {
	switch {
	case method == "Me":
		return ""
	case strings.HasPrefix(method, "Get"), strings.HasPrefix(method, "List"), strings.HasPrefix(method, "Query"):
		return ""
	case strings.HasPrefix(method, "Create"):
		return models.AuditActionCreate
	case strings.HasPrefix(method, "Delete"), strings.HasPrefix(method, "Remove"),
		strings.HasPrefix(method, "Unassign"), strings.HasPrefix(method, "Revoke"):
		return models.AuditActionDelete
	case strings.HasPrefix(method, "Restore"):
		return models.AuditActionRestore
	case strings.HasPrefix(method, "Purge"):
		return models.AuditActionPurge
	case method == "Refresh":
		return models.AuditActionRefresh
	default:
		// Update/Set/Put/Add/Assign/Archive/Grant/etc.
		return models.AuditActionUpdate
	}
}

// toJSONMap round-trips an arbitrary value (typically a model row) into a
// JSONMap for the before/after snapshot. Returns nil for nil/unmarshalable
// input rather than failing the audit write.
func toJSONMap(v any) models.JSONMap {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		log.WithError(err).Warn("audit: snapshot marshal failed; dropping before/after")
		return nil
	}
	var m models.JSONMap
	if err := json.Unmarshal(b, &m); err != nil {
		// Not a JSON object (e.g. an array) — wrap it so it still records.
		return models.JSONMap{"value": json.RawMessage(b)}
	}
	return m
}

func strPtr(s string) *string { return &s }
