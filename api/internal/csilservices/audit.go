package csilservices

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/catalystcommunity/longhouse/api/internal/csilrpc"
	"github.com/catalystcommunity/longhouse/api/internal/store"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// AuditService serves the admin-only, per-house audit log query. Recording is
// done by the dispatcher interceptor + auth security events, not here.
type AuditService struct {
	Store store.Store
}

func (s *AuditService) Register(d *csilrpc.Dispatcher) {
	d.RegisterTyped("audit", "QueryAudit", csilrpc.Route(s.QueryAudit, csil.DecodeAuditQueryAuditRequest, csil.EncodeAuditQueryAuditResponse))
}

func (s *AuditService) QueryAudit(ctx context.Context, q csil.AuditQuery) (csil.AuditPage, error) {
	if q.HouseId == "" {
		return csil.AuditPage{}, csilrpc.BadRequest("house_id is required")
	}
	if _, _, err := requireRoleForHouse(ctx, string(q.HouseId), models.RoleAdmin); err != nil {
		return csil.AuditPage{}, err
	}

	f := models.AuditFilter{
		ResourceType: q.ResourceType,
		Action:       q.Action,
	}
	if q.ActorMemberId != nil {
		v := string(*q.ActorMemberId)
		f.ActorMemberID = &v
	}
	if q.Since != nil {
		if t, err := time.Parse(time.RFC3339, string(*q.Since)); err == nil {
			f.Since = &t
		}
	}
	if q.Until != nil {
		if t, err := time.Parse(time.RFC3339, string(*q.Until)); err == nil {
			f.Until = &t
		}
	}
	limit := 100
	if q.Limit != nil && *q.Limit > 0 {
		limit = int(*q.Limit)
	}
	f.Limit = limit
	if q.Cursor != nil {
		if cAt, cID, ok := decodeAuditCursor(*q.Cursor); ok {
			f.CursorCreatedAt = &cAt
			f.CursorAuditID = &cID
		}
	}

	rows, err := s.Store.ListAuditEntries(ctx, string(q.HouseId), f)
	if err != nil {
		return csil.AuditPage{}, csilrpc.Internal("internal error")
	}
	entries := make([]csil.AuditEntry, 0, len(rows))
	for i := range rows {
		entries = append(entries, auditEntryToCSIL(&rows[i]))
	}
	page := csil.AuditPage{Entries: entries}
	// A full page implies there may be more; hand back a keyset cursor.
	if len(rows) == limit && limit > 0 {
		last := rows[len(rows)-1]
		cur := encodeAuditCursor(last.CreatedAt, last.AuditID)
		page.NextCursor = &cur
	}
	return page, nil
}

func auditEntryToCSIL(e *models.AuditEntry) csil.AuditEntry {
	out := csil.AuditEntry{
		AuditId:      csil.AuditID(e.AuditID),
		ActorDomain:  e.ActorDomain,
		ActorUserId:  e.ActorUserID,
		ServiceName:  e.Service,
		Method:       e.Method,
		Action:       e.Action,
		ResourceType: e.ResourceType,
		ResourceId:   e.ResourceID,
		Outcome:      e.Outcome,
		Before:       jsonMapToStrPtr(e.Before),
		After:        jsonMapToStrPtr(e.After),
		Detail:       jsonMapToStrPtr(e.Detail),
		CreatedAt:    ts(e.CreatedAt),
	}
	if e.HouseID != nil {
		h := csil.HouseID(*e.HouseID)
		out.HouseId = &h
	}
	if e.ActorMemberID != nil {
		m := csil.MemberID(*e.ActorMemberID)
		out.ActorMemberId = &m
	}
	return out
}

// jsonMapToStrPtr renders a jsonb snapshot back to a JSON text field (CSIL
// models jsonb as JSON-encoded text). Returns nil for an absent snapshot.
func jsonMapToStrPtr(m models.JSONMap) *string {
	if m == nil {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

// Audit cursors are opaque base64("<RFC3339Nano created_at>|<audit_id>"), the
// keyset of the last row of a page, used to fetch strictly-older rows.
func encodeAuditCursor(t time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeAuditCursor(s string) (time.Time, string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, "", false
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", false
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", false
	}
	return t, parts[1], true
}
