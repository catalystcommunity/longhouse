package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/catalystcommunity/longhouse/webapp/internal/api"
	"github.com/catalystcommunity/longhouse/webapp/internal/session"
)

// fakeSessionAPI records what the views called and lets tests preset
// what each method returns. Returning the zero value with no error is
// usually fine for "render the page" smoke tests.
type fakeSessionAPI struct {
	calls []string

	listMembers      func(houseID string) ([]api.Member, error)
	updateMember     func(houseID, memberID string, body api.UpdateMemberRequest) (*api.Member, error)
	listRoles        func(houseID string) ([]api.Role, error)
	createRole       func(houseID string, body api.CreateRoleRequest) (*api.Role, error)
	deleteRole       func(houseID, roleID string) error
	listMemberRoles  func(houseID, memberID string) ([]api.Role, error)
	grantRole        func(houseID, memberID, roleID string) error
	revokeRole       func(houseID, memberID, roleID string) error
	listTrustedDomains   func(houseID string) ([]api.TrustedDomain, error)
	createTrustedDomain  func(houseID string, body api.CreateTrustedDomainRequest) (*api.TrustedDomain, error)
	deleteTrustedDomain  func(houseID, tdID string) error
	listGroups        func(houseID string) ([]api.Group, error)
	createGroup       func(houseID string, body api.CreateGroupRequest) (*api.Group, error)
	deleteGroup       func(houseID, groupID string) error
	listGroupMembers  func(houseID, groupID string) ([]api.Member, error)
	addGroupMember    func(houseID, groupID, memberID string) error
	removeGroupMember func(houseID, groupID, memberID string) error
	listAuditsForMember func(houseID, memberID string) ([]api.MemberAudit, error)

	listEvents   func(houseID string) ([]api.Event, error)
	createEvent  func(houseID string, body api.CreateEventRequest) (*api.Event, error)
	getEvent     func(houseID, eventID string) (*api.Event, error)
	listComments     func(houseID, targetType, targetID string) ([]api.Comment, error)
	createComment    func(houseID, targetType, targetID string, body api.CreateCommentRequest) (*api.Comment, error)
	deleteCommentFn  func(houseID, commentID string) error
	listSkills       func(houseID string) ([]api.Skill, error)
	createSkill      func(houseID string, body api.CreateSkillRequest) (*api.Skill, error)
	deleteSkill      func(houseID, skillID string) error
	listMemberSkills func(houseID, memberID string) ([]api.Skill, error)
	addMemberSkill   func(houseID, memberID, skillID string) error
	removeMemberSk   func(houseID, memberID, skillID string) error
	listProjects     func(houseID string) ([]api.Project, error)
	createProject    func(houseID string, body api.CreateProjectRequest) (*api.Project, error)
	getProject       func(houseID, projectID string) (*api.Project, error)
	listProjectTasks func(houseID, projectID string) ([]api.Task, error)
	addProjectTask   func(houseID, projectID, taskID string, position int) error
	listTasks        func(houseID string) ([]api.Task, error)
	createTask       func(houseID string, body api.CreateTaskRequest) (*api.Task, error)
	getTask          func(houseID, taskID string) (*api.Task, error)
	updateTask       func(houseID, taskID string, body api.UpdateTaskRequest) (*api.Task, error)

	listShares  func(houseID string) ([]api.Share, error)
	createShare func(houseID string, body api.CreateShareRequest) (*api.Share, error)
	deleteShare func(houseID, shareID string) error
}

func (f *fakeSessionAPI) record(name string) { f.calls = append(f.calls, name) }

func (f *fakeSessionAPI) ListMembers(h string) ([]api.Member, error) {
	f.record("ListMembers")
	if f.listMembers != nil {
		return f.listMembers(h)
	}
	return nil, nil
}
func (f *fakeSessionAPI) UpdateMember(h, m string, b api.UpdateMemberRequest) (*api.Member, error) {
	f.record("UpdateMember")
	if f.updateMember != nil {
		return f.updateMember(h, m, b)
	}
	return &api.Member{MemberID: m, HouseID: h}, nil
}
func (f *fakeSessionAPI) ListRoles(h string) ([]api.Role, error) {
	f.record("ListRoles")
	if f.listRoles != nil {
		return f.listRoles(h)
	}
	return nil, nil
}
func (f *fakeSessionAPI) CreateRole(h string, b api.CreateRoleRequest) (*api.Role, error) {
	f.record("CreateRole")
	if f.createRole != nil {
		return f.createRole(h, b)
	}
	return &api.Role{RoleID: "r1", HouseID: h, Name: b.Name}, nil
}
func (f *fakeSessionAPI) DeleteRole(h, r string) error {
	f.record("DeleteRole")
	if f.deleteRole != nil {
		return f.deleteRole(h, r)
	}
	return nil
}
func (f *fakeSessionAPI) ListMemberRoles(h, m string) ([]api.Role, error) {
	f.record("ListMemberRoles")
	if f.listMemberRoles != nil {
		return f.listMemberRoles(h, m)
	}
	return nil, nil
}
func (f *fakeSessionAPI) GrantRole(h, m, r string) error {
	f.record("GrantRole")
	if f.grantRole != nil {
		return f.grantRole(h, m, r)
	}
	return nil
}
func (f *fakeSessionAPI) RevokeRole(h, m, r string) error {
	f.record("RevokeRole")
	if f.revokeRole != nil {
		return f.revokeRole(h, m, r)
	}
	return nil
}
func (f *fakeSessionAPI) ListGroups(h string) ([]api.Group, error) {
	f.record("ListGroups")
	if f.listGroups != nil {
		return f.listGroups(h)
	}
	return nil, nil
}
func (f *fakeSessionAPI) CreateGroup(h string, b api.CreateGroupRequest) (*api.Group, error) {
	f.record("CreateGroup")
	if f.createGroup != nil {
		return f.createGroup(h, b)
	}
	return &api.Group{GroupID: "g-1", HouseID: h, Name: b.Name}, nil
}
func (f *fakeSessionAPI) DeleteGroup(h, g string) error {
	f.record("DeleteGroup")
	if f.deleteGroup != nil {
		return f.deleteGroup(h, g)
	}
	return nil
}
func (f *fakeSessionAPI) ListGroupMembers(h, g string) ([]api.Member, error) {
	f.record("ListGroupMembers")
	if f.listGroupMembers != nil {
		return f.listGroupMembers(h, g)
	}
	return nil, nil
}
func (f *fakeSessionAPI) AddGroupMember(h, g, m string) error {
	f.record("AddGroupMember")
	if f.addGroupMember != nil {
		return f.addGroupMember(h, g, m)
	}
	return nil
}
func (f *fakeSessionAPI) RemoveGroupMember(h, g, m string) error {
	f.record("RemoveGroupMember")
	if f.removeGroupMember != nil {
		return f.removeGroupMember(h, g, m)
	}
	return nil
}
func (f *fakeSessionAPI) ListAuditsForMember(h, m string) ([]api.MemberAudit, error) {
	f.record("ListAuditsForMember")
	if f.listAuditsForMember != nil {
		return f.listAuditsForMember(h, m)
	}
	return nil, nil
}
func (f *fakeSessionAPI) ListEvents(h string) ([]api.Event, error) {
	f.record("ListEvents")
	if f.listEvents != nil {
		return f.listEvents(h)
	}
	return nil, nil
}
func (f *fakeSessionAPI) CreateEvent(h string, b api.CreateEventRequest) (*api.Event, error) {
	f.record("CreateEvent")
	if f.createEvent != nil {
		return f.createEvent(h, b)
	}
	return &api.Event{EventID: "e-1", HouseID: h, Title: b.Title}, nil
}
func (f *fakeSessionAPI) GetEvent(h, eventID string) (*api.Event, error) {
	f.record("GetEvent")
	if f.getEvent != nil {
		return f.getEvent(h, eventID)
	}
	return &api.Event{EventID: eventID, HouseID: h, Title: "Demo"}, nil
}
func (f *fakeSessionAPI) ListComments(h, targetType, targetID string) ([]api.Comment, error) {
	f.record("ListComments")
	if f.listComments != nil {
		return f.listComments(h, targetType, targetID)
	}
	return nil, nil
}
func (f *fakeSessionAPI) CreateComment(h, targetType, targetID string, b api.CreateCommentRequest) (*api.Comment, error) {
	f.record("CreateComment")
	if f.createComment != nil {
		return f.createComment(h, targetType, targetID, b)
	}
	return &api.Comment{CommentID: "c-1", HouseID: h, TargetType: targetType, TargetID: targetID, Body: b.Body}, nil
}
func (f *fakeSessionAPI) DeleteComment(h, commentID string) error {
	f.record("DeleteComment")
	if f.deleteCommentFn != nil {
		return f.deleteCommentFn(h, commentID)
	}
	return nil
}
func (f *fakeSessionAPI) ListTrustedDomains(h string) ([]api.TrustedDomain, error) {
	f.record("ListTrustedDomains")
	if f.listTrustedDomains != nil {
		return f.listTrustedDomains(h)
	}
	return nil, nil
}
func (f *fakeSessionAPI) CreateTrustedDomain(h string, b api.CreateTrustedDomainRequest) (*api.TrustedDomain, error) {
	f.record("CreateTrustedDomain")
	if f.createTrustedDomain != nil {
		return f.createTrustedDomain(h, b)
	}
	return &api.TrustedDomain{TrustedDomainID: "td-1", HouseID: h, Domain: b.Domain}, nil
}
func (f *fakeSessionAPI) DeleteTrustedDomain(h, td string) error {
	f.record("DeleteTrustedDomain")
	if f.deleteTrustedDomain != nil {
		return f.deleteTrustedDomain(h, td)
	}
	return nil
}
func (f *fakeSessionAPI) ListSkills(h string) ([]api.Skill, error) {
	f.record("ListSkills")
	if f.listSkills != nil {
		return f.listSkills(h)
	}
	return nil, nil
}
func (f *fakeSessionAPI) CreateSkill(h string, b api.CreateSkillRequest) (*api.Skill, error) {
	f.record("CreateSkill")
	if f.createSkill != nil {
		return f.createSkill(h, b)
	}
	return &api.Skill{SkillID: "s1", HouseID: h, Name: b.Name}, nil
}
func (f *fakeSessionAPI) DeleteSkill(h, s string) error {
	f.record("DeleteSkill")
	if f.deleteSkill != nil {
		return f.deleteSkill(h, s)
	}
	return nil
}
func (f *fakeSessionAPI) ListMemberSkills(h, m string) ([]api.Skill, error) {
	f.record("ListMemberSkills")
	if f.listMemberSkills != nil {
		return f.listMemberSkills(h, m)
	}
	return nil, nil
}
func (f *fakeSessionAPI) AddMemberSkill(h, m, s string) error {
	f.record("AddMemberSkill")
	if f.addMemberSkill != nil {
		return f.addMemberSkill(h, m, s)
	}
	return nil
}
func (f *fakeSessionAPI) RemoveMemberSkill(h, m, s string) error {
	f.record("RemoveMemberSkill")
	if f.removeMemberSk != nil {
		return f.removeMemberSk(h, m, s)
	}
	return nil
}
func (f *fakeSessionAPI) ListProjects(h string) ([]api.Project, error) {
	f.record("ListProjects")
	if f.listProjects != nil {
		return f.listProjects(h)
	}
	return nil, nil
}
func (f *fakeSessionAPI) CreateProject(h string, b api.CreateProjectRequest) (*api.Project, error) {
	f.record("CreateProject")
	if f.createProject != nil {
		return f.createProject(h, b)
	}
	return &api.Project{ProjectID: "p1", HouseID: h, Name: b.Name}, nil
}
func (f *fakeSessionAPI) GetProject(h, p string) (*api.Project, error) {
	f.record("GetProject")
	if f.getProject != nil {
		return f.getProject(h, p)
	}
	return &api.Project{ProjectID: p, HouseID: h, Name: "Demo"}, nil
}
func (f *fakeSessionAPI) ListProjectTasks(h, p string) ([]api.Task, error) {
	f.record("ListProjectTasks")
	if f.listProjectTasks != nil {
		return f.listProjectTasks(h, p)
	}
	return nil, nil
}
func (f *fakeSessionAPI) AddProjectTask(h, p, taskID string, pos int) error {
	f.record("AddProjectTask")
	if f.addProjectTask != nil {
		return f.addProjectTask(h, p, taskID, pos)
	}
	return nil
}
func (f *fakeSessionAPI) ListTasks(h string) ([]api.Task, error) {
	f.record("ListTasks")
	if f.listTasks != nil {
		return f.listTasks(h)
	}
	return nil, nil
}
func (f *fakeSessionAPI) CreateTask(h string, b api.CreateTaskRequest) (*api.Task, error) {
	f.record("CreateTask")
	if f.createTask != nil {
		return f.createTask(h, b)
	}
	return &api.Task{TaskID: "t1", HouseID: h, Title: b.Title}, nil
}
func (f *fakeSessionAPI) GetTask(h, taskID string) (*api.Task, error) {
	f.record("GetTask")
	if f.getTask != nil {
		return f.getTask(h, taskID)
	}
	return &api.Task{TaskID: taskID, HouseID: h, Title: "Demo"}, nil
}
func (f *fakeSessionAPI) UpdateTask(h, taskID string, b api.UpdateTaskRequest) (*api.Task, error) {
	f.record("UpdateTask")
	if f.updateTask != nil {
		return f.updateTask(h, taskID, b)
	}
	return &api.Task{TaskID: taskID, HouseID: h}, nil
}

func (f *fakeSessionAPI) ListShares(h string) ([]api.Share, error) {
	f.record("ListShares")
	if f.listShares != nil {
		return f.listShares(h)
	}
	return nil, nil
}
func (f *fakeSessionAPI) CreateShare(h string, b api.CreateShareRequest) (*api.Share, error) {
	f.record("CreateShare")
	if f.createShare != nil {
		return f.createShare(h, b)
	}
	return &api.Share{HouseID: h, ResourceType: b.ResourceType, ResourceID: b.ResourceID}, nil
}
func (f *fakeSessionAPI) DeleteShare(h, sid string) error {
	f.record("DeleteShare")
	if f.deleteShare != nil {
		return f.deleteShare(h, sid)
	}
	return nil
}

// ----- Test scaffolding -----

// viewDeps returns a *Deps wired with a session manager + the supplied
// fake api client + a permissive test config.
func viewDeps(fake *fakeSessionAPI) *Deps {
	return &Deps{
		Sessions:      session.New("test-secret", false),
		PKI:           &fakePKI{},
		API:           &fakeAPI{},
		NewSessionAPI: func(string) SessionAPI { return fake },
	}
}

// signedIn issues a session cookie carrying the given identity, by way of
// SetIdentity into a temporary recorder, and pulling the cookie back out.
func signedIn(t *testing.T, d *Deps, id session.Identity) []*http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	if err := d.Sessions.SetIdentity(rec, id); err != nil {
		t.Fatalf("SetIdentity: %v", err)
	}
	return rec.Result().Cookies()
}

func authedReq(t *testing.T, method, path string, body url.Values, cookies []*http.Cookie) *http.Request {
	t.Helper()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(body.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return req
}

// ----- Smoke tests -----

func TestViewMembers_RendersOK(t *testing.T) {
	fake := &fakeSessionAPI{
		listMembers: func(h string) ([]api.Member, error) {
			return []api.Member{{MemberID: "m1", HouseID: h, LinkkeysUserID: "u1"}}, nil
		},
	}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{
		MemberID: "m1", HouseID: "h1", APIToken: "tok",
	})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/members", nil, cookies))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	called := strings.Join(fake.calls, ",")
	for _, want := range []string{"ListMembers", "ListRoles", "ListMemberRoles"} {
		if !strings.Contains(called, want) {
			t.Errorf("expected %s in calls; got %s", want, called)
		}
	}
}

func TestViewRoles_AdminGate(t *testing.T) {
	d := viewDeps(&fakeSessionAPI{})
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/admin/roles", nil, cookies))

	if rec.Code != http.StatusForbidden {
		t.Errorf("non-admin should be 403, got %d", rec.Code)
	}
}

func TestViewRoles_AdminAllowed(t *testing.T) {
	fake := &fakeSessionAPI{}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"admin"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/admin/roles", nil, cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(fake.calls) == 0 || fake.calls[0] != "ListRoles" {
		t.Errorf("ListRoles not called: %+v", fake.calls)
	}
}

func TestViewCreateRole_RedirectsAfterCreate(t *testing.T) {
	fake := &fakeSessionAPI{}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"admin"}})

	form := url.Values{}
	form.Set("name", "moderator")
	form.Set("description", "trusted")
	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/admin/roles", form, cookies))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "/app/admin/roles" {
		t.Errorf("redirect: %q", rec.Header().Get("Location"))
	}
	if len(fake.calls) == 0 || fake.calls[0] != "CreateRole" {
		t.Errorf("CreateRole not called: %+v", fake.calls)
	}
}

func TestViewSkills_MemberSelfAdd(t *testing.T) {
	fake := &fakeSessionAPI{}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/skills/skill-1/add", url.Values{}, cookies))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	wantedPrefix := "AddMemberSkill"
	found := false
	for _, c := range fake.calls {
		if c == wantedPrefix {
			found = true
		}
	}
	if !found {
		t.Errorf("AddMemberSkill not called: %+v", fake.calls)
	}
}

func TestViewProjects_CreateRedirectsToDetail(t *testing.T) {
	fake := &fakeSessionAPI{
		createProject: func(h string, b api.CreateProjectRequest) (*api.Project, error) {
			return &api.Project{ProjectID: "p-42", HouseID: h, Name: b.Name}, nil
		},
	}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	form := url.Values{}
	form.Set("name", "Spring cleanup")
	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/projects", form, cookies))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status: %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/app/projects/p-42" {
		t.Errorf("redirect: %q", rec.Header().Get("Location"))
	}
}

func TestViewTasks_CreateLinksToProject(t *testing.T) {
	fake := &fakeSessionAPI{
		createTask: func(h string, b api.CreateTaskRequest) (*api.Task, error) {
			return &api.Task{TaskID: "t-7", HouseID: h, Title: b.Title}, nil
		},
	}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	form := url.Values{}
	form.Set("title", "Cut wood")
	form.Set("project_id", "p-1")
	form.Set("position", "3")
	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/tasks", form, cookies))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	called := strings.Join(fake.calls, ",")
	if !strings.Contains(called, "CreateTask") || !strings.Contains(called, "AddProjectTask") {
		t.Errorf("expected CreateTask+AddProjectTask; got %s", called)
	}
}

func TestViewTrustedDomains_AdminGate(t *testing.T) {
	d := viewDeps(&fakeSessionAPI{})
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/admin/trusted-domains", nil, cookies))
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-admin should be 403, got %d", rec.Code)
	}
}

func TestViewTrustedDomains_AdminCanAddAndRemove(t *testing.T) {
	fake := &fakeSessionAPI{}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"admin"}})

	form := url.Values{}
	form.Set("domain", "todandlorna.com")
	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/admin/trusted-domains", form, cookies))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d, body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/admin/trusted-domains/td-1/delete", url.Values{}, cookies))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d, body=%s", rec.Code, rec.Body.String())
	}

	called := strings.Join(fake.calls, ",")
	if !strings.Contains(called, "CreateTrustedDomain") || !strings.Contains(called, "DeleteTrustedDomain") {
		t.Errorf("expected create+delete calls; got %s", called)
	}
}

func TestViewShares_AdminGate(t *testing.T) {
	d := viewDeps(&fakeSessionAPI{})
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/admin/shares", nil, cookies))
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-admin should be 403, got %d", rec.Code)
	}
}

func TestViewShares_AdminListsRendersAndShowsShareDetails(t *testing.T) {
	fake := &fakeSessionAPI{
		listShares: func(string) ([]api.Share, error) {
			return []api.Share{
				{ShareID: "s1", LinkkeysDomain: "guest.example", LinkkeysUserID: "ext-1",
					ResourceType: "task", ResourceID: "t1", AccessLevel: "read"},
			}, nil
		},
	}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"admin"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/admin/shares", nil, cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"guest.example/ext-1", "task", "t1", "Revoke"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in body:\n%s", want, body)
		}
	}
}

func TestViewShares_AdminCanCreateAndDelete(t *testing.T) {
	fake := &fakeSessionAPI{}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"admin"}})

	form := url.Values{}
	form.Set("linkkeys_domain", "guest.example")
	form.Set("linkkeys_user_id", "ext-9")
	form.Set("resource_type", "event")
	form.Set("resource_id", "e1")

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/admin/shares", form, cookies))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d, body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/admin/shares/s1/delete", url.Values{}, cookies))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d, body=%s", rec.Code, rec.Body.String())
	}

	called := strings.Join(fake.calls, ",")
	if !strings.Contains(called, "CreateShare") || !strings.Contains(called, "DeleteShare") {
		t.Errorf("expected create+delete calls; got %s", called)
	}
}

func TestViewTask_LoadsComments(t *testing.T) {
	fake := &fakeSessionAPI{
		listComments: func(h, tt, ti string) ([]api.Comment, error) {
			return []api.Comment{{CommentID: "c1", Body: "hello", MemberID: "m9"}}, nil
		},
	}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/tasks/t-1", nil, cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hello") {
		t.Errorf("comment body not rendered; body=%s", rec.Body.String())
	}
	called := strings.Join(fake.calls, ",")
	if !strings.Contains(called, "ListComments") {
		t.Errorf("ListComments not called: %s", called)
	}
}

func TestViewCreateComment_RedirectsBackToTask(t *testing.T) {
	fake := &fakeSessionAPI{}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	form := url.Values{}
	form.Set("body", "ack")
	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/comments/task/t-7", form, cookies))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "/app/tasks/t-7" {
		t.Errorf("redirect: %q", rec.Header().Get("Location"))
	}
}

func TestViewEvents_ListAndCreate(t *testing.T) {
	fake := &fakeSessionAPI{
		listEvents: func(h string) ([]api.Event, error) {
			return []api.Event{{EventID: "e1", Title: "Standup"}}, nil
		},
	}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/events", nil, cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Standup") {
		t.Errorf("event title not rendered; body=%s", rec.Body.String())
	}

	form := url.Values{}
	form.Set("title", "Sprint review")
	form.Set("starts_at", "2026-05-01T09:00:00Z")
	rec = httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/events", form, cookies))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d, body=%s", rec.Code, rec.Body.String())
	}
	called := strings.Join(fake.calls, ",")
	if !strings.Contains(called, "CreateEvent") {
		t.Errorf("CreateEvent not called: %s", called)
	}
}

func TestViewEvent_LoadsCommentsToo(t *testing.T) {
	fake := &fakeSessionAPI{
		listComments: func(h, tt, ti string) ([]api.Comment, error) {
			if tt != "event" {
				t.Errorf("expected target_type=event, got %q", tt)
			}
			return []api.Comment{{CommentID: "c1", Body: "see you there", MemberID: "m9"}}, nil
		},
	}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/events/e-7", nil, cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "see you there") {
		t.Errorf("comment body not rendered; body=%s", rec.Body.String())
	}
}

func TestViewGroups_AdminGate(t *testing.T) {
	d := viewDeps(&fakeSessionAPI{})
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/admin/groups", nil, cookies))
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-admin should be 403, got %d", rec.Code)
	}
}

func TestViewGroups_AdminCreateAndAddMember(t *testing.T) {
	fake := &fakeSessionAPI{
		listGroups: func(h string) ([]api.Group, error) {
			return []api.Group{{GroupID: "g1", Name: "team"}}, nil
		},
	}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"admin"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/admin/groups", nil, cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d, body=%s", rec.Code, rec.Body.String())
	}

	form := url.Values{}
	form.Set("name", "new-group")
	rec = httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/admin/groups", form, cookies))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d, body=%s", rec.Code, rec.Body.String())
	}

	form = url.Values{}
	form.Set("member_id", "m-x")
	rec = httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/admin/groups/g1/members", form, cookies))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("add member: %d, body=%s", rec.Code, rec.Body.String())
	}

	called := strings.Join(fake.calls, ",")
	for _, want := range []string{"ListGroups", "CreateGroup", "AddGroupMember"} {
		if !strings.Contains(called, want) {
			t.Errorf("expected %s in %s", want, called)
		}
	}
}

func TestViewCreateTask_PropagatesRecurrence(t *testing.T) {
	var captured api.CreateTaskRequest
	fake := &fakeSessionAPI{
		createTask: func(h string, b api.CreateTaskRequest) (*api.Task, error) {
			captured = b
			return &api.Task{TaskID: "t-1", HouseID: h}, nil
		},
	}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	form := url.Values{}
	form.Set("title", "Take out trash")
	form.Set("recurrence_freq", "weekly")
	form.Set("recurrence_interval", "2")
	form.Add("recurrence_by_weekday", "1")
	form.Add("recurrence_by_weekday", "4")
	form.Set("next_recurrence_at", "2026-05-04T08:00:00Z")
	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodPost, "/app/tasks", form, cookies))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}

	if captured.RecurrenceFreq == nil || *captured.RecurrenceFreq != "weekly" {
		t.Errorf("freq: %+v", captured.RecurrenceFreq)
	}
	if captured.RecurrenceInterval != 2 {
		t.Errorf("interval: %d, want 2", captured.RecurrenceInterval)
	}
	if len(captured.RecurrenceByWeekday) != 2 {
		t.Errorf("by_weekday: %+v", captured.RecurrenceByWeekday)
	}
	if captured.NextRecurrenceAt != "2026-05-04T08:00:00Z" {
		t.Errorf("next_recurrence_at: %q", captured.NextRecurrenceAt)
	}
}

func TestViewMemberAudits_AdminGate(t *testing.T) {
	d := viewDeps(&fakeSessionAPI{})
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"member"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/admin/members/m1/audits", nil, cookies))
	if rec.Code != http.StatusForbidden {
		t.Errorf("non-admin should be 403, got %d", rec.Code)
	}
}

func TestViewMemberAudits_AdminRendersRows(t *testing.T) {
	fake := &fakeSessionAPI{
		listAuditsForMember: func(h, m string) ([]api.MemberAudit, error) {
			return []api.MemberAudit{
				{AuditID: "a1", Action: "role_granted", CreatedAt: "2026-04-29T12:00:00Z"},
			}, nil
		},
	}
	d := viewDeps(fake)
	cookies := signedIn(t, d, session.Identity{MemberID: "m1", HouseID: "h1", Roles: []string{"admin"}})

	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, authedReq(t, http.MethodGet, "/app/admin/members/m9/audits", nil, cookies))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "role_granted") {
		t.Errorf("expected action in rendered audit row; body=%s", rec.Body.String())
	}
}

func TestRequireAuth_RedirectsAnonymous(t *testing.T) {
	d := viewDeps(&fakeSessionAPI{})
	rec := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/members", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login" {
		t.Errorf("expected anon redirect to /login, got %d → %q", rec.Code, rec.Header().Get("Location"))
	}
}
