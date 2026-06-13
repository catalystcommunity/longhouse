package csilservices

import (
	"context"

	"github.com/catalystcommunity/longhouse/api/internal/audit"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

// annotateAudit sets the request's audit draft for a non-delete admin action
// (restore/purge). The dispatcher records it after the handler returns.
func annotateAudit(ctx context.Context, houseID, action, resourceType, resourceID string, detail map[string]any) {
	audit.Annotate(ctx, func(d *audit.Draft) {
		d.HouseID = houseID
		d.Action = action
		d.ResourceType = resourceType
		d.ResourceID = resourceID
		d.Detail = detail
	})
}

// annotateDelete enriches the request's audit draft for a soft-delete so the
// log answers "who deleted this <resource>" and so a restore can find the
// batch by deleted_op_id and reconstruct mutated fields from the before
// snapshot. The dispatcher records the entry after the handler returns.
//
// before should be the resource row as it existed pre-delete (for milestones
// etc. whose row carries no house_id, pass the owning house explicitly).
func annotateDelete(ctx context.Context, houseID, resourceType, resourceID, opID string, before any) {
	annotateDeleteWithDetail(ctx, houseID, resourceType, resourceID, opID, before, nil)
}

// annotateDeleteWithDetail is annotateDelete with extra detail fields merged in
// (e.g. the "this & future" event sweep records the root's pre-mute recurrence
// so a future restore can re-arm it).
func annotateDeleteWithDetail(ctx context.Context, houseID, resourceType, resourceID, opID string, before any, extra map[string]any) {
	audit.Annotate(ctx, func(d *audit.Draft) {
		d.HouseID = houseID
		d.ResourceType = resourceType
		d.ResourceID = resourceID
		d.Action = models.AuditActionDelete
		d.Before = before
		det := map[string]any{"deleted_op_id": opID}
		for k, v := range extra {
			det[k] = v
		}
		d.Detail = det
	})
}
