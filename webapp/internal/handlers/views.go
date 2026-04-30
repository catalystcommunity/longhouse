package handlers

import (
	"net/http"
	"strconv"

	"github.com/catalystcommunity/longhouse/webapp/internal/api"
	log "github.com/sirupsen/logrus"
)

// Each view handler does roughly the same thing: pull the caller's identity
// out of the request context (set by requireAuth), build an api client
// scoped to their bearer token, do the call(s), render a template or
// redirect. Errors render a generic page rather than detailed messages.

// ----- Members -----

func (d *Deps) viewMembers(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	c := d.sessionAPI(id)
	members, err := c.ListMembers(id.HouseID)
	if err != nil {
		renderError(w, "loading members", err)
		return
	}
	roles, _ := c.ListRoles(id.HouseID)
	memberRoles := map[string][]api.Role{}
	for _, m := range members {
		mr, _ := c.ListMemberRoles(id.HouseID, m.MemberID)
		memberRoles[m.MemberID] = mr
	}
	renderTemplate(w, "members.html", map[string]any{
		"Title":       "Members",
		"Identity":    id,
		"Members":     members,
		"Roles":       roles,
		"MemberRoles": memberRoles,
	})
}

func (d *Deps) viewUpdateMember(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	memberID := r.PathValue("member_id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	displayName := r.FormValue("display_name")
	if _, err := d.sessionAPI(id).UpdateMember(id.HouseID, memberID, api.UpdateMemberRequest{DisplayName: &displayName}); err != nil {
		renderError(w, "updating member", err)
		return
	}
	http.Redirect(w, r, "/app/members", http.StatusSeeOther)
}

func (d *Deps) viewMemberAudits(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	memberID := r.PathValue("member_id")
	audits, err := d.sessionAPI(id).ListAuditsForMember(id.HouseID, memberID)
	if err != nil {
		renderError(w, "loading member audits", err)
		return
	}
	renderTemplate(w, "member_audits.html", map[string]any{
		"Title":    "Member audit log",
		"Identity": id,
		"MemberID": memberID,
		"Audits":   audits,
	})
}

// ----- Roles -----

func (d *Deps) viewRoles(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	roles, err := d.sessionAPI(id).ListRoles(id.HouseID)
	if err != nil {
		renderError(w, "loading roles", err)
		return
	}
	renderTemplate(w, "roles.html", map[string]any{
		"Title":    "Roles",
		"Identity": id,
		"Roles":    roles,
	})
}

func (d *Deps) viewCreateRole(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if _, err := d.sessionAPI(id).CreateRole(id.HouseID, api.CreateRoleRequest{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
	}); err != nil {
		renderError(w, "creating role", err)
		return
	}
	http.Redirect(w, r, "/app/admin/roles", http.StatusSeeOther)
}

func (d *Deps) viewDeleteRole(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := d.sessionAPI(id).DeleteRole(id.HouseID, r.PathValue("role_id")); err != nil {
		renderError(w, "deleting role", err)
		return
	}
	http.Redirect(w, r, "/app/admin/roles", http.StatusSeeOther)
}

func (d *Deps) viewGrantRole(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := d.sessionAPI(id).GrantRole(id.HouseID, r.PathValue("member_id"), r.PathValue("role_id")); err != nil {
		renderError(w, "granting role", err)
		return
	}
	http.Redirect(w, r, "/app/members", http.StatusSeeOther)
}

func (d *Deps) viewRevokeRole(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := d.sessionAPI(id).RevokeRole(id.HouseID, r.PathValue("member_id"), r.PathValue("role_id")); err != nil {
		renderError(w, "revoking role", err)
		return
	}
	http.Redirect(w, r, "/app/members", http.StatusSeeOther)
}

// ----- Events -----

func (d *Deps) viewEvents(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	events, err := d.sessionAPI(id).ListEvents(id.HouseID)
	if err != nil {
		renderError(w, "loading events", err)
		return
	}
	renderTemplate(w, "events.html", map[string]any{
		"Title":    "Events",
		"Identity": id,
		"Events":   events,
	})
}

func (d *Deps) viewCreateEvent(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if _, err := d.sessionAPI(id).CreateEvent(id.HouseID, api.CreateEventRequest{
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Location:    r.FormValue("location"),
		StartsAt:    r.FormValue("starts_at"),
		EndsAt:      r.FormValue("ends_at"),
		AllDay:      r.FormValue("all_day") == "on",
	}); err != nil {
		renderError(w, "creating event", err)
		return
	}
	http.Redirect(w, r, "/app/events", http.StatusSeeOther)
}

func (d *Deps) viewEvent(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	c := d.sessionAPI(id)
	eventID := r.PathValue("event_id")
	e, err := c.GetEvent(id.HouseID, eventID)
	if err != nil {
		renderError(w, "loading event", err)
		return
	}
	comments, _ := c.ListComments(id.HouseID, "event", eventID)
	renderTemplate(w, "event.html", map[string]any{
		"Title":    e.Title,
		"Identity": id,
		"Event":    e,
		"Comments": comments,
	})
}

// ----- Groups (admin) -----

func (d *Deps) viewGroups(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	c := d.sessionAPI(id)
	groups, err := c.ListGroups(id.HouseID)
	if err != nil {
		renderError(w, "loading groups", err)
		return
	}
	groupMembers := map[string][]api.Member{}
	for _, g := range groups {
		gm, _ := c.ListGroupMembers(id.HouseID, g.GroupID)
		groupMembers[g.GroupID] = gm
	}
	allMembers, _ := c.ListMembers(id.HouseID)
	renderTemplate(w, "groups.html", map[string]any{
		"Title":        "Groups",
		"Identity":     id,
		"Groups":       groups,
		"GroupMembers": groupMembers,
		"AllMembers":   allMembers,
	})
}

func (d *Deps) viewCreateGroup(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if _, err := d.sessionAPI(id).CreateGroup(id.HouseID, api.CreateGroupRequest{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
	}); err != nil {
		renderError(w, "creating group", err)
		return
	}
	http.Redirect(w, r, "/app/admin/groups", http.StatusSeeOther)
}

func (d *Deps) viewDeleteGroup(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := d.sessionAPI(id).DeleteGroup(id.HouseID, r.PathValue("group_id")); err != nil {
		renderError(w, "deleting group", err)
		return
	}
	http.Redirect(w, r, "/app/admin/groups", http.StatusSeeOther)
}

func (d *Deps) viewAddGroupMember(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	memberID := r.FormValue("member_id")
	if err := d.sessionAPI(id).AddGroupMember(id.HouseID, r.PathValue("group_id"), memberID); err != nil {
		renderError(w, "adding group member", err)
		return
	}
	http.Redirect(w, r, "/app/admin/groups", http.StatusSeeOther)
}

func (d *Deps) viewRemoveGroupMember(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := d.sessionAPI(id).RemoveGroupMember(id.HouseID, r.PathValue("group_id"), r.PathValue("member_id")); err != nil {
		renderError(w, "removing group member", err)
		return
	}
	http.Redirect(w, r, "/app/admin/groups", http.StatusSeeOther)
}

// ----- Trusted domains (admin) -----

func (d *Deps) viewTrustedDomains(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	tds, err := d.sessionAPI(id).ListTrustedDomains(id.HouseID)
	if err != nil {
		renderError(w, "loading trusted domains", err)
		return
	}
	renderTemplate(w, "trusted_domains.html", map[string]any{
		"Title":    "Trusted Domains",
		"Identity": id,
		"Domains":  tds,
	})
}

func (d *Deps) viewCreateTrustedDomain(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if _, err := d.sessionAPI(id).CreateTrustedDomain(id.HouseID, api.CreateTrustedDomainRequest{
		Domain: r.FormValue("domain"),
	}); err != nil {
		renderError(w, "adding trusted domain", err)
		return
	}
	http.Redirect(w, r, "/app/admin/trusted-domains", http.StatusSeeOther)
}

func (d *Deps) viewDeleteTrustedDomain(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := d.sessionAPI(id).DeleteTrustedDomain(id.HouseID, r.PathValue("trusted_domain_id")); err != nil {
		renderError(w, "removing trusted domain", err)
		return
	}
	http.Redirect(w, r, "/app/admin/trusted-domains", http.StatusSeeOther)
}

// ----- Shares (admin) -----

func (d *Deps) viewShares(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	shares, err := d.sessionAPI(id).ListShares(id.HouseID)
	if err != nil {
		renderError(w, "loading shares", err)
		return
	}
	renderTemplate(w, "shares.html", map[string]any{
		"Title":    "Shares",
		"Identity": id,
		"Shares":   shares,
	})
}

func (d *Deps) viewCreateShare(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if _, err := d.sessionAPI(id).CreateShare(id.HouseID, api.CreateShareRequest{
		LinkkeysDomain: r.FormValue("linkkeys_domain"),
		LinkkeysUserID: r.FormValue("linkkeys_user_id"),
		ResourceType:   r.FormValue("resource_type"),
		ResourceID:     r.FormValue("resource_id"),
		ExpiresAt:      r.FormValue("expires_at"),
	}); err != nil {
		renderError(w, "creating share", err)
		return
	}
	http.Redirect(w, r, "/app/admin/shares", http.StatusSeeOther)
}

func (d *Deps) viewDeleteShare(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := d.sessionAPI(id).DeleteShare(id.HouseID, r.PathValue("share_id")); err != nil {
		renderError(w, "removing share", err)
		return
	}
	http.Redirect(w, r, "/app/admin/shares", http.StatusSeeOther)
}

// ----- Skills -----

func (d *Deps) viewSkills(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	c := d.sessionAPI(id)
	all, err := c.ListSkills(id.HouseID)
	if err != nil {
		renderError(w, "loading skills", err)
		return
	}
	mine, _ := c.ListMemberSkills(id.HouseID, id.MemberID)
	mineSet := map[string]bool{}
	for _, s := range mine {
		mineSet[s.SkillID] = true
	}
	renderTemplate(w, "skills.html", map[string]any{
		"Title":      "Skills",
		"Identity":   id,
		"Skills":     all,
		"OwnSkillID": mineSet,
		"IsAdmin":    id.HasRole("admin"),
	})
}

func (d *Deps) viewCreateSkill(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if _, err := d.sessionAPI(id).CreateSkill(id.HouseID, api.CreateSkillRequest{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
	}); err != nil {
		renderError(w, "creating skill", err)
		return
	}
	http.Redirect(w, r, "/app/skills", http.StatusSeeOther)
}

func (d *Deps) viewDeleteSkill(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := d.sessionAPI(id).DeleteSkill(id.HouseID, r.PathValue("skill_id")); err != nil {
		renderError(w, "deleting skill", err)
		return
	}
	http.Redirect(w, r, "/app/skills", http.StatusSeeOther)
}

func (d *Deps) viewAddOwnSkill(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := d.sessionAPI(id).AddMemberSkill(id.HouseID, id.MemberID, r.PathValue("skill_id")); err != nil {
		renderError(w, "adding skill", err)
		return
	}
	http.Redirect(w, r, "/app/skills", http.StatusSeeOther)
}

func (d *Deps) viewRemoveOwnSkill(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := d.sessionAPI(id).RemoveMemberSkill(id.HouseID, id.MemberID, r.PathValue("skill_id")); err != nil {
		renderError(w, "removing skill", err)
		return
	}
	http.Redirect(w, r, "/app/skills", http.StatusSeeOther)
}

// ----- Projects -----

func (d *Deps) viewProjects(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	projects, err := d.sessionAPI(id).ListProjects(id.HouseID)
	if err != nil {
		renderError(w, "loading projects", err)
		return
	}
	renderTemplate(w, "projects.html", map[string]any{
		"Title":    "Projects",
		"Identity": id,
		"Projects": projects,
	})
}

func (d *Deps) viewCreateProject(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	p, err := d.sessionAPI(id).CreateProject(id.HouseID, api.CreateProjectRequest{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
	})
	if err != nil {
		renderError(w, "creating project", err)
		return
	}
	http.Redirect(w, r, "/app/projects/"+p.ProjectID, http.StatusSeeOther)
}

func (d *Deps) viewProject(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	c := d.sessionAPI(id)
	projectID := r.PathValue("project_id")
	p, err := c.GetProject(id.HouseID, projectID)
	if err != nil {
		renderError(w, "loading project", err)
		return
	}
	tasks, _ := c.ListProjectTasks(id.HouseID, projectID)
	renderTemplate(w, "project.html", map[string]any{
		"Title":    p.Name,
		"Identity": id,
		"Project":  p,
		"Tasks":    tasks,
	})
}

// ----- Tasks -----

func (d *Deps) viewTasks(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	c := d.sessionAPI(id)
	tasks, err := c.ListTasks(id.HouseID)
	if err != nil {
		renderError(w, "loading tasks", err)
		return
	}
	members, _ := c.ListMembers(id.HouseID)
	skills, _ := c.ListSkills(id.HouseID)
	renderTemplate(w, "tasks.html", map[string]any{
		"Title":      "Tasks",
		"Identity":   id,
		"Tasks":      tasks,
		"Members":    members,
		"Skills":     skills,
		"ParentTask": tasks, // parent picker uses the same list
	})
}

func (d *Deps) viewCreateTask(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	req := api.CreateTaskRequest{
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
	}
	if v := r.FormValue("assigned_to_member_id"); v != "" {
		req.AssignedToMemberID = &v
	}
	if v := r.FormValue("assigned_to_skill_id"); v != "" {
		req.AssignedToSkillID = &v
	}
	if v := r.FormValue("parent_task_id"); v != "" {
		req.ParentTaskID = &v
	}
	if v := r.FormValue("recurrence_freq"); v != "" {
		req.RecurrenceFreq = &v
		if iv := r.FormValue("recurrence_interval"); iv != "" {
			if n, err := strconv.Atoi(iv); err == nil {
				req.RecurrenceInterval = n
			}
		}
		// Multi-select weekdays arrive as repeated form keys; r.Form
		// preserves them. Skips parse failures silently.
		if days, ok := r.Form["recurrence_by_weekday"]; ok {
			for _, d := range days {
				if n, err := strconv.Atoi(d); err == nil {
					req.RecurrenceByWeekday = append(req.RecurrenceByWeekday, n)
				}
			}
		}
		if v := r.FormValue("next_recurrence_at"); v != "" {
			req.NextRecurrenceAt = v
		}
	}
	t, err := d.sessionAPI(id).CreateTask(id.HouseID, req)
	if err != nil {
		renderError(w, "creating task", err)
		return
	}
	// Optional project linking after task creation.
	if pid := r.FormValue("project_id"); pid != "" {
		pos := 0
		if v := r.FormValue("position"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				pos = n
			}
		}
		_ = d.sessionAPI(id).AddProjectTask(id.HouseID, pid, t.TaskID, pos)
	}
	http.Redirect(w, r, "/app/tasks", http.StatusSeeOther)
}

func (d *Deps) viewTask(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	c := d.sessionAPI(id)
	taskID := r.PathValue("task_id")
	t, err := c.GetTask(id.HouseID, taskID)
	if err != nil {
		renderError(w, "loading task", err)
		return
	}
	comments, _ := c.ListComments(id.HouseID, "task", taskID)
	renderTemplate(w, "task.html", map[string]any{
		"Title":    t.Title,
		"Identity": id,
		"Task":     t,
		"Comments": comments,
	})
}

func (d *Deps) viewCreateComment(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	targetType := r.PathValue("target_type")
	targetID := r.PathValue("target_id")
	if _, err := d.sessionAPI(id).CreateComment(id.HouseID, targetType, targetID, api.CreateCommentRequest{
		Body: r.FormValue("body"),
	}); err != nil {
		renderError(w, "posting comment", err)
		return
	}
	// Send the user back to whichever resource they were commenting on.
	switch targetType {
	case "task":
		http.Redirect(w, r, "/app/tasks/"+targetID, http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
	}
}

func (d *Deps) viewDeleteComment(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	commentID := r.PathValue("comment_id")
	returnTo := r.FormValue("return_to")
	if returnTo == "" {
		returnTo = "/app/dashboard"
	}
	if err := d.sessionAPI(id).DeleteComment(id.HouseID, commentID); err != nil {
		renderError(w, "deleting comment", err)
		return
	}
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func (d *Deps) viewUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := identityFrom(r.Context())
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	req := api.UpdateTaskRequest{}
	if v := r.FormValue("title"); v != "" {
		req.Title = &v
	}
	if v := r.FormValue("description"); v != "" {
		req.Description = &v
	}
	if v := r.FormValue("status"); v != "" {
		req.Status = &v
	}
	taskID := r.PathValue("task_id")
	if _, err := d.sessionAPI(id).UpdateTask(id.HouseID, taskID, req); err != nil {
		renderError(w, "updating task", err)
		return
	}
	http.Redirect(w, r, "/app/tasks/"+taskID, http.StatusSeeOther)
}

// renderError logs the error and renders a single generic error page so we
// don't leak api error bodies into the UI. Operators see the detail in
// logs; users see a one-liner.
func renderError(w http.ResponseWriter, what string, err error) {
	log.WithError(err).Errorf("view: %s failed", what)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	_ = standalones.ExecuteTemplate(w, "error.html", map[string]any{
		"Title":   "Error",
		"Message": "Something went wrong while " + what + ".",
	})
}
