//go:build e2e

// End-to-end RBAC/privacy check against a LIVE api on localhost:6080 and the
// dev postgres. Not part of the normal suite — run explicitly:
//
//	set -a; . ./.env.dev; set +a
//	LH_HOUSE_ID=... LH_ADMIN_MEMBER=... LH_BOB_MEMBER=... LH_GROUP_ID=... \
//	  go test -tags=e2e -run TestRBACE2E -v ./internal/csilservices/
//
// It mints real bearer tokens (admin + a non-admin "Bob") with the server's
// JWT secret and exercises: private-by-containment, "see it through any
// project you can access", member grants, group grants, the umbrella
// guardrail, and hidden_count. Uses the real csil types + the server's CBOR
// enc mode, so the wire format matches exactly.
package csilservices

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/catalystcommunity/longhouse/api/internal/auth"
	"github.com/catalystcommunity/longhouse/api/internal/csil"
	"github.com/fxamacker/cbor/v2"
)

const e2eBase = "http://localhost:6080/api/csil"

func env(t *testing.T, k string) string {
	v := os.Getenv(k)
	if v == "" {
		t.Fatalf("missing required env %s", k)
	}
	return v
}

func e2eToken(t *testing.T, secret []byte, houseID, memberID string, roles []string) string {
	tok, err := auth.Mint(secret, auth.Identity{
		Domain: "todandlorna.com",
		UserID: "e2e-" + memberID,
		Houses: []auth.HouseRoles{{House: houseID, Member: memberID, Roles: roles}},
	}, time.Hour)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	return tok
}

var e2eEnc, _ = cbor.CoreDetEncOptions().EncMode()

// call POSTs req (CBOR) to service/method with the bearer token, decoding a
// 200 body into out. Returns (httpStatus, serviceErrMessage).
func call(t *testing.T, token, service, method string, req any, out any) (int, string) {
	t.Helper()
	var body []byte
	if req != nil {
		b, err := e2eEnc.Marshal(req)
		if err != nil {
			t.Fatalf("marshal %s/%s: %v", service, method, err)
		}
		body = b
	}
	httpReq, _ := http.NewRequest(http.MethodPost, e2eBase+"/"+service+"/"+method, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/cbor")
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("http %s/%s: %v", service, method, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var se struct {
			Code    string `cbor:"code"`
			Message string `cbor:"message"`
		}
		_ = cbor.Unmarshal(raw, &se)
		return resp.StatusCode, se.Message
	}
	if out != nil {
		if err := cbor.Unmarshal(raw, out); err != nil {
			t.Fatalf("decode %s/%s response: %v", service, method, err)
		}
	}
	return resp.StatusCode, ""
}

func mustOK(t *testing.T, label string, status int, msg string) {
	t.Helper()
	if status != http.StatusOK {
		t.Fatalf("%s: expected 200, got %d (%s)", label, status, msg)
	}
}

func taskIDs(tl csil.TaskList) map[string]bool {
	m := map[string]bool{}
	for _, x := range tl.Tasks {
		m[string(x.TaskId)] = true
	}
	return m
}
func projIDs(pl csil.ProjectList) map[string]bool {
	m := map[string]bool{}
	for _, x := range pl.Projects {
		m[string(x.ProjectId)] = true
	}
	return m
}

func acc(s string) *csil.AccessLevel { v := csil.AccessLevel(s); return &v }

func TestRBACE2E(t *testing.T) {
	secret := []byte(env(t, "LONGHOUSE_JWT_SECRET"))
	houseID := env(t, "LH_HOUSE_ID")
	adminMember := env(t, "LH_ADMIN_MEMBER")
	bobMember := env(t, "LH_BOB_MEMBER")
	groupID := env(t, "LH_GROUP_ID")

	admin := e2eToken(t, secret, houseID, adminMember, []string{"admin", "member"})
	bob := e2eToken(t, secret, houseID, bobMember, []string{"member"})

	stamp := time.Now().UTC().Format("150405.000")

	// --- Admin creates a PUBLIC and a PRIVATE project --------------------
	var pub, priv, priv2 csil.Project
	st, msg := call(t, admin, "project", "CreateProject",
		csil.Project{HouseId: csil.HouseID(houseID), Name: "E2E Public " + stamp, Visibility: acc("read")}, &pub)
	mustOK(t, "create pub", st, msg)
	st, msg = call(t, admin, "project", "CreateProject",
		csil.Project{HouseId: csil.HouseID(houseID), Name: "E2E Private " + stamp, Visibility: acc("none")}, &priv)
	mustOK(t, "create priv", st, msg)
	st, msg = call(t, admin, "project", "CreateProject",
		csil.Project{HouseId: csil.HouseID(houseID), Name: "E2E Private2 " + stamp, Visibility: acc("none")}, &priv2)
	mustOK(t, "create priv2", st, msg)
	t.Logf("projects: pub=%s priv=%s priv2=%s", pub.ProjectId, priv.ProjectId, priv2.ProjectId)

	// Confirm created visibility persisted.
	if got := accString(priv.Visibility); got != "none" {
		t.Fatalf("priv project visibility: want none got %q", got)
	}

	// --- Admin creates tasks and places them ----------------------------
	mkTask := func(title string) csil.Task {
		var tk csil.Task
		st, msg := call(t, admin, "task", "CreateTask",
			csil.Task{HouseId: csil.HouseID(houseID), Title: title, Assignees: []csil.MemberID{}}, &tk)
		mustOK(t, "create "+title, st, msg)
		return tk
	}
	addToProject := func(p csil.Project, tk csil.Task) {
		st, msg := call(t, admin, "project", "AddProjectTask",
			csil.ProjectTaskOrderRequest{ProjectId: p.ProjectId, TaskId: tk.TaskId, Position: 0}, nil)
		mustOK(t, "addProjectTask", st, msg)
	}

	tPrivOnly := mkTask("E2E priv-only " + stamp) // only in PRIV
	tPubOnly := mkTask("E2E pub-only " + stamp)   // only in PUB
	tBoth := mkTask("E2E both " + stamp)          // in PUB and PRIV
	tFree := mkTask("E2E free " + stamp)          // no project at all

	addToProject(priv, tPrivOnly)
	addToProject(pub, tPubOnly)
	addToProject(pub, tBoth)
	addToProject(priv, tBoth)

	// === Assertion 1: admin (owner/admin) sees everything ===============
	var adminTasks csil.TaskList
	st, msg = call(t, admin, "task", "ListTasks", csil.HouseScopedListRequest{HouseId: csil.HouseID(houseID), Limit: u64(500)}, &adminTasks)
	mustOK(t, "admin listTasks", st, msg)
	at := taskIDs(adminTasks)
	for _, tk := range []csil.Task{tPrivOnly, tPubOnly, tBoth, tFree} {
		if !at[string(tk.TaskId)] {
			t.Errorf("admin should see task %s", tk.TaskId)
		}
	}
	t.Logf("PASS: admin sees all own tasks (hidden_count=%d)", adminTasks.HiddenCount)

	// === Assertion 2: Bob (non-admin, non-owner, no grants) =============
	// priv-only hidden; pub-only and both visible (via PUB); free visible.
	var bobTasks csil.TaskList
	st, msg = call(t, bob, "task", "ListTasks", csil.HouseScopedListRequest{HouseId: csil.HouseID(houseID), Limit: u64(500)}, &bobTasks)
	mustOK(t, "bob listTasks", st, msg)
	bt := taskIDs(bobTasks)
	if bt[string(tPrivOnly.TaskId)] {
		t.Errorf("FAIL: Bob must NOT see priv-only task %s", tPrivOnly.TaskId)
	} else {
		t.Logf("PASS: Bob cannot see priv-only task")
	}
	if !bt[string(tPubOnly.TaskId)] {
		t.Errorf("FAIL: Bob should see pub-only task")
	}
	if !bt[string(tBoth.TaskId)] {
		t.Errorf("FAIL: Bob should see 'both' task via the public project (see-through-any-project)")
	} else {
		t.Logf("PASS: Bob sees the task shared via the public project even though it's also in a private one")
	}
	if bobTasks.HiddenCount < 1 {
		t.Errorf("FAIL: Bob's hidden_count should be >= 1, got %d", bobTasks.HiddenCount)
	} else {
		t.Logf("PASS: Bob's listTasks reports hidden_count=%d", bobTasks.HiddenCount)
	}

	// === Assertion 3: project visibility filtering ======================
	var bobProjects csil.ProjectList
	st, msg = call(t, bob, "project", "ListProjects", csil.HouseScopedListRequest{HouseId: csil.HouseID(houseID), Limit: u64(500)}, &bobProjects)
	mustOK(t, "bob listProjects", st, msg)
	bp := projIDs(bobProjects)
	if bp[string(priv.ProjectId)] {
		t.Errorf("FAIL: Bob must NOT see private project")
	}
	if !bp[string(pub.ProjectId)] {
		t.Errorf("FAIL: Bob should see public project")
	}
	if bobProjects.HiddenCount < 2 {
		t.Errorf("FAIL: Bob should have >=2 hidden projects (priv, priv2), got %d", bobProjects.HiddenCount)
	} else {
		t.Logf("PASS: Bob sees public project, %d hidden", bobProjects.HiddenCount)
	}

	// === Assertion 4: direct member grant opens access ==================
	st, msg = call(t, admin, "project", "PutProjectGrant",
		csil.PutProjectGrantRequest{ProjectId: priv.ProjectId, GranteeType: csil.GranteeType("member"), GranteeId: bobMember, AccessLevel: csil.AccessLevel("read")}, nil)
	mustOK(t, "putProjectGrant member", st, msg)
	st, msg = call(t, bob, "task", "ListTasks", csil.HouseScopedListRequest{HouseId: csil.HouseID(houseID), Limit: u64(500)}, &bobTasks)
	mustOK(t, "bob listTasks after grant", st, msg)
	if !taskIDs(bobTasks)[string(tPrivOnly.TaskId)] {
		t.Errorf("FAIL: after read grant on priv, Bob should see priv-only task")
	} else {
		t.Logf("PASS: direct member read-grant on the private project reveals its task to Bob")
	}

	// === Assertion 5: GROUP grant opens access ==========================
	// Add Bob to the group, grant the group read on priv2, expect Bob to see it.
	st, msg = call(t, admin, "group", "AddGroupMember",
		csil.GroupMemberRef{GroupId: csil.GroupID(groupID), MemberId: csil.MemberID(bobMember)}, nil)
	mustOK(t, "addGroupMember", st, msg)
	st, msg = call(t, admin, "project", "PutProjectGrant",
		csil.PutProjectGrantRequest{ProjectId: priv2.ProjectId, GranteeType: csil.GranteeType("group"), GranteeId: groupID, AccessLevel: csil.AccessLevel("read")}, nil)
	mustOK(t, "putProjectGrant group", st, msg)
	st, msg = call(t, bob, "project", "ListProjects", csil.HouseScopedListRequest{HouseId: csil.HouseID(houseID), Limit: u64(500)}, &bobProjects)
	mustOK(t, "bob listProjects after group grant", st, msg)
	if !projIDs(bobProjects)[string(priv2.ProjectId)] {
		t.Errorf("FAIL: after group read-grant, Bob should see priv2 via group membership")
	} else {
		t.Logf("PASS: group read-grant reveals priv2 to Bob through his group membership")
	}

	// === Assertion 6: umbrella guardrail ================================
	// A free-floating task (no project, no parent) cannot be made private.
	st, msg = call(t, admin, "task", "SetTaskVisibility",
		csil.SetTaskVisibilityRequest{TaskId: tFree.TaskId, Visibility: csil.AccessLevel("none")}, nil)
	if st == http.StatusOK {
		t.Errorf("FAIL: setting a free-floating task to none should be rejected, got 200")
	} else {
		t.Logf("PASS: free-floating task can't be made private (status %d: %s)", st, msg)
	}

	// === Assertion 7: Bob (read on priv) cannot mutate governance =======
	// Bob has read on priv now; he must NOT be able to add a grant (full only).
	st, msg = call(t, bob, "project", "PutProjectGrant",
		csil.PutProjectGrantRequest{ProjectId: priv.ProjectId, GranteeType: csil.GranteeType("member"), GranteeId: bobMember, AccessLevel: csil.AccessLevel("full")}, nil)
	if st == http.StatusOK {
		t.Errorf("FAIL: Bob with read should not be able to self-escalate via PutProjectGrant, got 200")
	} else {
		t.Logf("PASS: Bob (read) cannot manage grants (status %d: %s)", st, msg)
	}

	// --- Cleanup: delete the projects we created (cascades grants/links).
	for _, p := range []csil.Project{pub, priv, priv2} {
		call(t, admin, "project", "DeleteProject", p.ProjectId, nil)
	}
	for _, tk := range []csil.Task{tPrivOnly, tPubOnly, tBoth, tFree} {
		call(t, admin, "task", "DeleteTask", tk.TaskId, nil)
	}
	call(t, admin, "group", "RemoveGroupMember",
		csil.GroupMemberRef{GroupId: csil.GroupID(groupID), MemberId: csil.MemberID(bobMember)}, nil)
}

func u64(v uint64) *uint64 { return &v }

func accString(p *csil.AccessLevel) string {
	if p == nil {
		return ""
	}
	return string(*p)
}
