package audit

import (
	"context"
	"testing"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/store/postgres/models"
)

type capture struct{ entry *models.AuditEntry }

func (c *capture) RecordAuditEntry(_ context.Context, e *models.AuditEntry) error {
	c.entry = e
	return nil
}
func TestDelegatedMutationRecordsUserAndApplication(t *testing.T) {
	ctx, _ := WithDraft(context.Background())
	Annotate(ctx, func(d *Draft) {
		d.HouseID = "house"
		d.Detail = map[string]any{"other": "retained", "act_as_grantee": "spoof"}
	})
	sink := &capture{}
	id := &auth.Identity{Domain: "home.test", UserID: "user", Houses: []auth.HouseRoles{{House: "house", Member: "member"}}, Delegation: &auth.Delegation{GrantID: "grant", Grantee: "owner@app.test/helper", Scopes: []string{"tasks.write"}}}
	if err := Emit(ctx, sink, "task", "CreateTask", id, models.AuditOutcomeOK); err != nil {
		t.Fatal(err)
	}
	e := sink.entry
	if e.ActorDomain != "home.test" || e.ActorUserID != "user" || e.ActorMemberID == nil || *e.ActorMemberID != "member" || e.Detail["act_as_grantee"] != id.Delegation.Grantee || e.Detail["act_as_grant_id"] != "grant" || e.Detail["other"] != "retained" {
		t.Fatalf("wrong audit: %+v", e)
	}
}
