package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"

	longhouseclient "github.com/catalystcommunity/longhouse/clients/go"
)

const structuredHelpVersion = "1"

type cliClientFactory func(longhouseclient.Transport) any

type cliOperation struct {
	ID           string
	Service      string
	Method       string
	Operation    string
	Path         []string
	Aliases      [][]string
	Summary      string
	RequestType  reflect.Type
	ResponseType reflect.Type
	Factory      cliClientFactory
	Positionals  []string
	Destructive  bool
	Mutation     bool
	HiddenFields map[string]bool
}

type cliFieldHelp struct {
	Name        string          `json:"name"`
	JSONPath    string          `json:"json_path,omitempty"`
	Type        string          `json:"type"`
	Required    bool            `json:"required"`
	Default     json.RawMessage `json:"default,omitempty"`
	Description string          `json:"description,omitempty"`
	Enum        []string        `json:"enum,omitempty"`
}

type cliCommandHelp struct {
	Path         []string       `json:"path"`
	Aliases      [][]string     `json:"aliases,omitempty"`
	Summary      string         `json:"summary"`
	Usage        []string       `json:"usage,omitempty"`
	Arguments    []string       `json:"arguments,omitempty"`
	Options      []cliFieldHelp `json:"options,omitempty"`
	Service      string         `json:"service,omitempty"`
	Operation    string         `json:"operation,omitempty"`
	RequestType  string         `json:"request_type,omitempty"`
	ResponseType string         `json:"response_type,omitempty"`
	OutputFields []cliFieldHelp `json:"output_fields,omitempty"`
	Examples     []string       `json:"examples,omitempty"`
	Environment  []string       `json:"environment,omitempty"`
	Related      [][]string     `json:"related,omitempty"`
	Effect       string         `json:"effect,omitempty"`
	Mutation     bool           `json:"mutation"`
	Destructive  bool           `json:"destructive"`
}

type cliHelpDocument struct {
	SchemaVersion string           `json:"schema_version"`
	Topic         []string         `json:"topic,omitempty"`
	Commands      []cliCommandHelp `json:"commands"`
}

type cliClientFamily struct {
	service string
	new     cliClientFactory
}

var cliClientFamilies = []cliClientFamily{
	{"AuthService", func(t longhouseclient.Transport) any { return longhouseclient.NewAuthClient(t) }},
	{"DevAuthService", func(t longhouseclient.Transport) any { return longhouseclient.NewDevAuthClient(t) }},
	{"HouseService", func(t longhouseclient.Transport) any { return longhouseclient.NewHouseClient(t) }},
	{"MemberService", func(t longhouseclient.Transport) any { return longhouseclient.NewMemberClient(t) }},
	{"TrustedDomainService", func(t longhouseclient.Transport) any { return longhouseclient.NewTrustedDomainClient(t) }},
	{"RoleService", func(t longhouseclient.Transport) any { return longhouseclient.NewRoleClient(t) }},
	{"SkillService", func(t longhouseclient.Transport) any { return longhouseclient.NewSkillClient(t) }},
	{"GroupService", func(t longhouseclient.Transport) any { return longhouseclient.NewGroupClient(t) }},
	{"ProjectService", func(t longhouseclient.Transport) any { return longhouseclient.NewProjectClient(t) }},
	{"EventService", func(t longhouseclient.Transport) any { return longhouseclient.NewEventClient(t) }},
	{"TaskService", func(t longhouseclient.Transport) any { return longhouseclient.NewTaskClient(t) }},
	{"DependencyService", func(t longhouseclient.Transport) any { return longhouseclient.NewDependencyClient(t) }},
	{"CommentService", func(t longhouseclient.Transport) any { return longhouseclient.NewCommentClient(t) }},
	{"NotificationService", func(t longhouseclient.Transport) any { return longhouseclient.NewNotificationClient(t) }},
	{"ShareService", func(t longhouseclient.Transport) any { return longhouseclient.NewShareClient(t) }},
	{"MemberAuditService", func(t longhouseclient.Transport) any { return longhouseclient.NewMemberAuditClient(t) }},
	{"SettingsService", func(t longhouseclient.Transport) any { return longhouseclient.NewSettingsClient(t) }},
	{"BugService", func(t longhouseclient.Transport) any { return longhouseclient.NewBugClient(t) }},
	{"AuditService", func(t longhouseclient.Transport) any { return longhouseclient.NewAuditClient(t) }},
	{"TrashService", func(t longhouseclient.Transport) any { return longhouseclient.NewTrashClient(t) }},
}

var cliPathOverrides = map[string]string{
	"AuthService.ListSessions":                 "auth session list",
	"AuthService.RevokeSession":                "auth session revoke",
	"HouseService.CreateHouse":                 "house create",
	"HouseService.GetHouse":                    "house view",
	"HouseService.UpdateHouse":                 "house edit",
	"HouseService.DeleteHouse":                 "house delete",
	"HouseService.ListHouses":                  "house list",
	"MemberService.CreateMember":               "member add",
	"MemberService.GetMember":                  "member view",
	"MemberService.GetMemberByIdentity":        "member identity view",
	"MemberService.UpdateMember":               "member edit",
	"MemberService.DeactivateMember":           "member remove",
	"MemberService.ReactivateMember":           "member activate",
	"MemberService.ListMembers":                "member list",
	"TrustedDomainService.AddTrustedDomain":    "admin domain add",
	"TrustedDomainService.RemoveTrustedDomain": "admin domain remove",
	"TrustedDomainService.ListTrustedDomains":  "admin domain list",
	"TrustedDomainService.IsDomainTrusted":     "admin domain check",
	"RoleService.CreateRole":                   "role create",
	"RoleService.UpdateRole":                   "role edit",
	"RoleService.DeleteRole":                   "role delete",
	"RoleService.ListRoles":                    "role list",
	"RoleService.GrantRole":                    "member role add",
	"RoleService.RevokeRole":                   "member role remove",
	"RoleService.ListMemberRoles":              "member role list",
	"SkillService.CreateSkill":                 "skill create",
	"SkillService.UpdateSkill":                 "skill edit",
	"SkillService.DeleteSkill":                 "skill delete",
	"SkillService.ListSkills":                  "skill list",
	"SkillService.AddMemberSkill":              "member skill add",
	"SkillService.RemoveMemberSkill":           "member skill remove",
	"SkillService.ListMemberSkills":            "member skill list",
	"SkillService.AddGroupSkill":               "group skill add",
	"SkillService.RemoveGroupSkill":            "group skill remove",
	"SkillService.ListGroupSkills":             "group skill list",
	"GroupService.CreateGroup":                 "group create",
	"GroupService.UpdateGroup":                 "group edit",
	"GroupService.DeleteGroup":                 "group delete",
	"GroupService.ListGroups":                  "group list",
	"GroupService.AddGroupMember":              "group member add",
	"GroupService.RemoveGroupMember":           "group member remove",
	"GroupService.ListGroupMembers":            "group member list",
	"ProjectService.CreateProject":             "project create",
	"ProjectService.GetProject":                "project view",
	"ProjectService.UpdateProject":             "project edit",
	"ProjectService.DeleteProject":             "project delete",
	"ProjectService.ListProjects":              "project list",
	"ProjectService.ListProjectTasks":          "project task list",
	"ProjectService.AddProjectTask":            "project task add",
	"ProjectService.RemoveProjectTask":         "project task remove",
	"ProjectService.SetProjectTaskPosition":    "project task move",
	"ProjectService.ListProjectMembers":        "project member list",
	"ProjectService.AddProjectMember":          "project member add",
	"ProjectService.RemoveProjectMember":       "project member remove",
	"ProjectService.ListProjectOwners":         "project owner list",
	"ProjectService.AddProjectOwner":           "project owner add",
	"ProjectService.RemoveProjectOwner":        "project owner remove",
	"ProjectService.ListMilestones":            "project milestone list",
	"ProjectService.CreateMilestone":           "project milestone create",
	"ProjectService.UpdateMilestone":           "project milestone edit",
	"ProjectService.DeleteMilestone":           "project milestone delete",
	"ProjectService.SetProjectVisibility":      "project visibility set",
	"ProjectService.ListProjectGrants":         "project grant list",
	"ProjectService.PutProjectGrant":           "project grant set",
	"ProjectService.DeleteProjectGrant":        "project grant remove",
	"EventService.CreateEvent":                 "event create",
	"EventService.GetEvent":                    "event view",
	"EventService.UpdateEvent":                 "event edit",
	"EventService.DeleteEvent":                 "event delete",
	"EventService.DeleteEventAndFuture":        "event delete-future",
	"EventService.ListEvents":                  "event list",
	"EventService.GetCalendarView":             "calendar view",
	"EventService.SetCalendarView":             "calendar edit",
	"TaskService.CreateTask":                   "task create",
	"TaskService.GetTask":                      "task view",
	"TaskService.UpdateTask":                   "task edit",
	"TaskService.DeleteTask":                   "task delete",
	"TaskService.ListTasks":                    "task list",
	"TaskService.SetTaskVisibility":            "task visibility set",
	"TaskService.ListTaskGrants":               "task grant list",
	"TaskService.PutTaskGrant":                 "task grant set",
	"TaskService.DeleteTaskGrant":              "task grant remove",
	"DependencyService.AddDependency":          "dependency add",
	"DependencyService.RemoveDependency":       "dependency remove",
	"DependencyService.GetDependencies":        "dependency list",
	"CommentService.CreateComment":             "comment add",
	"CommentService.GetComment":                "comment view",
	"CommentService.UpdateComment":             "comment edit",
	"CommentService.DeleteComment":             "comment delete",
	"CommentService.ListComments":              "comment list",
	"NotificationService.ListNotifications":    "notification list",
	"NotificationService.UnreadCount":          "notification count",
	"NotificationService.MarkRead":             "notification read",
	"NotificationService.MarkAllRead":          "notification read-all",
	"ShareService.CreateShare":                 "access share create",
	"ShareService.DeleteShare":                 "access share delete",
	"ShareService.ListSharesByResource":        "access share list",
	"ShareService.CheckAccess":                 "access share check",
	"MemberAuditService.ListAuditsForMember":   "member audit",
	"SettingsService.GetSettings":              "admin settings view",
	"SettingsService.UpdateSettings":           "admin settings edit",
	"BugService.ReportBug":                     "bug report",
	"AuditService.QueryAudit":                  "admin audit list",
	"TrashService.ListTrash":                   "admin trash list",
	"TrashService.Restore":                     "admin trash restore",
	"TrashService.Purge":                       "admin trash purge",
}

var cliPositionalOverrides = map[string][]string{
	"AuthService.RevokeSession":                {"session_id"},
	"HouseService.CreateHouse":                 {"name"},
	"HouseService.GetHouse":                    {"value"},
	"HouseService.UpdateHouse":                 {"house_id"},
	"HouseService.DeleteHouse":                 {"value"},
	"MemberService.CreateMember":               {"linkkeys_domain", "linkkeys_user_id"},
	"MemberService.GetMember":                  {"value"},
	"MemberService.UpdateMember":               {"member_id"},
	"MemberService.DeactivateMember":           {"value"},
	"MemberService.ReactivateMember":           {"value"},
	"TrustedDomainService.AddTrustedDomain":    {"domain"},
	"TrustedDomainService.RemoveTrustedDomain": {"value"},
	"RoleService.CreateRole":                   {"name"},
	"RoleService.UpdateRole":                   {"role_id"},
	"RoleService.DeleteRole":                   {"value"},
	"RoleService.GrantRole":                    {"member_id", "role_id"},
	"RoleService.RevokeRole":                   {"member_id", "role_id"},
	"RoleService.ListMemberRoles":              {"member_id"},
	"SkillService.CreateSkill":                 {"name"},
	"SkillService.UpdateSkill":                 {"skill_id"},
	"SkillService.DeleteSkill":                 {"value"},
	"SkillService.AddMemberSkill":              {"member_id", "skill_id"},
	"SkillService.RemoveMemberSkill":           {"member_id", "skill_id"},
	"SkillService.ListMemberSkills":            {"member_id"},
	"SkillService.AddGroupSkill":               {"group_id", "skill_id"},
	"SkillService.RemoveGroupSkill":            {"group_id", "skill_id"},
	"SkillService.ListGroupSkills":             {"value"},
	"GroupService.CreateGroup":                 {"name"},
	"GroupService.UpdateGroup":                 {"group_id"},
	"GroupService.DeleteGroup":                 {"value"},
	"GroupService.AddGroupMember":              {"group_id", "member_id"},
	"GroupService.RemoveGroupMember":           {"group_id", "member_id"},
	"GroupService.ListGroupMembers":            {"member_id"},
	"ProjectService.CreateProject":             {"name"},
	"ProjectService.GetProject":                {"value"},
	"ProjectService.UpdateProject":             {"project_id"},
	"ProjectService.DeleteProject":             {"value"},
	"ProjectService.ListProjectTasks":          {"project_id"},
	"ProjectService.AddProjectTask":            {"project_id", "task_id"},
	"ProjectService.RemoveProjectTask":         {"project_id", "task_id"},
	"ProjectService.SetProjectTaskPosition":    {"project_id", "task_id", "position"},
	"ProjectService.ListProjectMembers":        {"value"},
	"ProjectService.AddProjectMember":          {"project_id", "member_id"},
	"ProjectService.RemoveProjectMember":       {"project_id", "member_id"},
	"ProjectService.ListProjectOwners":         {"value"},
	"ProjectService.AddProjectOwner":           {"project_id", "member_id"},
	"ProjectService.RemoveProjectOwner":        {"project_id", "member_id"},
	"ProjectService.ListMilestones":            {"value"},
	"ProjectService.CreateMilestone":           {"project_id", "label"},
	"ProjectService.UpdateMilestone":           {"milestone_id"},
	"ProjectService.DeleteMilestone":           {"value"},
	"ProjectService.SetProjectVisibility":      {"project_id", "visibility"},
	"ProjectService.ListProjectGrants":         {"value"},
	"ProjectService.PutProjectGrant":           {"project_id", "grantee_type", "grantee_id", "access_level"},
	"ProjectService.DeleteProjectGrant":        {"project_id", "grantee_type", "grantee_id"},
	"EventService.CreateEvent":                 {"title"},
	"EventService.GetEvent":                    {"value"},
	"EventService.UpdateEvent":                 {"event_id"},
	"EventService.DeleteEvent":                 {"value"},
	"EventService.DeleteEventAndFuture":        {"value"},
	"TaskService.CreateTask":                   {"title"},
	"TaskService.GetTask":                      {"value"},
	"TaskService.UpdateTask":                   {"task_id"},
	"TaskService.DeleteTask":                   {"value"},
	"TaskService.ListTaskGrants":               {"value"},
	"TaskService.SetTaskVisibility":            {"task_id", "visibility"},
	"TaskService.PutTaskGrant":                 {"task_id", "grantee_type", "grantee_id", "access_level"},
	"TaskService.DeleteTaskGrant":              {"task_id", "grantee_type", "grantee_id"},
	"DependencyService.AddDependency":          {"dependent_type", "dependent_id", "dependency_type", "dependency_id"},
	"DependencyService.RemoveDependency":       {"dependent_type", "dependent_id", "dependency_type", "dependency_id"},
	"DependencyService.GetDependencies":        {"type", "id"},
	"CommentService.CreateComment":             {"target_type", "target_id"},
	"CommentService.GetComment":                {"value"},
	"CommentService.UpdateComment":             {"comment_id"},
	"CommentService.DeleteComment":             {"value"},
	"CommentService.ListComments":              {"target_type", "target_id"},
	"NotificationService.MarkRead":             {"value"},
	"ShareService.CreateShare":                 {"resource_type", "resource_id", "linkkeys_domain", "linkkeys_user_id"},
	"ShareService.DeleteShare":                 {"value"},
	"ShareService.ListSharesByResource":        {"resource_type", "resource_id"},
	"ShareService.CheckAccess":                 {"resource_type", "resource_id", "linkkeys_domain", "linkkeys_user_id"},
	"MemberAuditService.ListAuditsForMember":   {"member_id"},
	"BugService.ReportBug":                     {"title"},
}

var cliAliases = map[string][][]string{
	"MemberService.CreateMember":          {{"member", "invite"}},
	"TaskService.SetTaskVisibility":       {{"access", "task", "visibility", "set"}},
	"TaskService.ListTaskGrants":          {{"access", "task", "grant", "list"}},
	"TaskService.PutTaskGrant":            {{"access", "task", "grant", "set"}},
	"TaskService.DeleteTaskGrant":         {{"access", "task", "grant", "remove"}},
	"ProjectService.SetProjectVisibility": {{"access", "project", "visibility", "set"}},
	"ProjectService.ListProjectGrants":    {{"access", "project", "grant", "list"}},
	"ProjectService.PutProjectGrant":      {{"access", "project", "grant", "set"}},
	"ProjectService.DeleteProjectGrant":   {{"access", "project", "grant", "remove"}},
}

func buildCLIOperations() []*cliOperation {
	var operations []*cliOperation
	for _, family := range cliClientFamilies {
		clientType := reflect.TypeOf(family.new(nil))
		for i := 0; i < clientType.NumMethod(); i++ {
			method := clientType.Method(i)
			if method.Type.NumIn() != 3 || method.Type.NumOut() != 2 {
				continue
			}
			id := family.service + "." + method.Name
			path := strings.Fields(cliPathOverrides[id])
			if len(path) == 0 {
				path = deriveCLIPath(family.service, method.Name)
			}
			operation := &cliOperation{
				ID:           id,
				Service:      family.service,
				Method:       method.Name,
				Operation:    camelToKebab(method.Name),
				Path:         path,
				Aliases:      cliAliases[id],
				Summary:      humanizeWords(splitCamel(method.Name)),
				RequestType:  method.Type.In(2),
				ResponseType: method.Type.Out(0),
				Factory:      family.new,
				Positionals:  cliPositionalOverrides[id],
			}
			operation.Mutation, operation.Destructive = classifyOperation(method.Name)
			operation.HiddenFields = hiddenInputFields(operation)
			operations = append(operations, operation)
		}
	}
	sort.Slice(operations, func(i, j int) bool {
		return strings.Join(operations[i].Path, " ") < strings.Join(operations[j].Path, " ")
	})
	return operations
}

func (o *cliOperation) newRequest() reflect.Value {
	request := reflect.New(o.RequestType)
	switch request.Interface().(type) {
	case *longhouseclient.Project:
		request.Elem().Set(reflect.ValueOf(*longhouseclient.NewProject()))
	case *longhouseclient.Event:
		request.Elem().Set(reflect.ValueOf(*longhouseclient.NewEvent()))
	case *longhouseclient.Task:
		request.Elem().Set(reflect.ValueOf(*longhouseclient.NewTask()))
	case *longhouseclient.Milestone:
		milestone := longhouseclient.Milestone{State: longhouseclient.MilestoneState("future")}
		request.Elem().Set(reflect.ValueOf(milestone))
	case *longhouseclient.Share:
		request.Elem().Set(reflect.ValueOf(*longhouseclient.NewShare()))
	case *longhouseclient.HouseListRequest:
		request.Elem().Set(reflect.ValueOf(*longhouseclient.NewHouseListRequest()))
	case *longhouseclient.HouseScopedListRequest:
		request.Elem().Set(reflect.ValueOf(*longhouseclient.NewHouseScopedListRequest()))
	case *longhouseclient.MemberScopedListRequest:
		request.Elem().Set(reflect.ValueOf(*longhouseclient.NewMemberScopedListRequest()))
	case *longhouseclient.ProjectScopedListRequest:
		request.Elem().Set(reflect.ValueOf(*longhouseclient.NewProjectScopedListRequest()))
	case *longhouseclient.CommentListRequest:
		request.Elem().Set(reflect.ValueOf(*longhouseclient.NewCommentListRequest()))
	case *longhouseclient.NotificationListRequest:
		request.Elem().Set(reflect.ValueOf(*longhouseclient.NewNotificationListRequest()))
	case *longhouseclient.AuditQuery:
		request.Elem().Set(reflect.ValueOf(*longhouseclient.NewAuditQuery()))
	}
	return request
}

func (o *cliOperation) call(ctx context.Context, transport longhouseclient.Transport, request reflect.Value) (any, error) {
	client := reflect.ValueOf(o.Factory(transport))
	method := client.MethodByName(o.Method)
	results := method.Call([]reflect.Value{reflect.ValueOf(ctx), request.Elem()})
	if !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}
	return results[0].Interface(), nil
}

func (o *cliOperation) validate(request reflect.Value) error {
	validator, ok := request.Interface().(interface{ Validate() error })
	if !ok {
		return nil
	}
	return validator.Validate()
}

func (o *cliOperation) help() cliCommandHelp {
	usage := "longhouse " + strings.Join(o.Path, " ")
	arguments := append([]string(nil), o.Positionals...)
	if o.ID == "GroupService.ListGroupMembers" && len(arguments) == 1 {
		arguments[0] = "group_id"
	}
	for _, argument := range arguments {
		if argument == "value" {
			argument = strings.TrimSuffix(strings.ToLower(o.RequestType.Name()), "id") + "-id"
		}
		usage += " <" + strings.ReplaceAll(argument, "_", "-") + ">"
	}
	if o.RequestType.Kind() == reflect.Struct {
		usage += " [options]"
	}
	effect := "read"
	if o.Mutation {
		effect = "mutation"
	}
	if o.Destructive {
		effect = "destructive mutation"
	}
	return cliCommandHelp{
		Path:         o.Path,
		Aliases:      o.Aliases,
		Summary:      o.Summary,
		Usage:        []string{usage, "longhouse " + strings.Join(o.Path, " ") + " --input <file> [--output json]"},
		Arguments:    arguments,
		Options:      append(operationFieldHelp(o), operationGlobalCLIFieldHelp(o)...),
		Service:      o.Service,
		Operation:    o.Operation,
		RequestType:  o.RequestType.String(),
		ResponseType: o.ResponseType.String(),
		OutputFields: responseFieldHelp(o.ResponseType),
		Examples:     []string{usage},
		Environment:  []string{"LONGHOUSE_URL", "LONGHOUSE_PROFILE", "LONGHOUSE_HOUSE", "LONGHOUSE_ACCESSIBLE"},
		Effect:       effect,
		Mutation:     o.Mutation,
		Destructive:  o.Destructive,
	}
}

func operationGlobalCLIFieldHelp(operation *cliOperation) []cliFieldHelp {
	options := globalCLIFieldHelp()
	filtered := make([]cliFieldHelp, 0, len(options))
	for _, option := range options {
		if option.Name == "dry-run" && !operation.Mutation {
			continue
		}
		if option.Name == "yes" && !operation.Destructive {
			continue
		}
		if option.Name == "interactive" && (!operation.Mutation || operation.RequestType.Kind() != reflect.Struct) {
			continue
		}
		filtered = append(filtered, option)
	}
	return filtered
}

func globalCLIFieldHelp() []cliFieldHelp {
	return []cliFieldHelp{
		{Name: "input", Type: "file", Description: "Read strict JSON input. Use - for standard input."},
		{Name: "output", Type: "text|json", Description: "Select the output format."},
		{Name: "json", Type: "bool", Description: "Use JSON output."},
		{Name: "profile", Type: "string", Description: "Select a local profile."},
		{Name: "house", Type: "HouseID|name", Description: "Override the active house."},
		{Name: "interactive", Type: "bool", Description: "Show the terminal form."},
		{Name: "no-input", Type: "bool", Description: "Do not prompt."},
		{Name: "dry-run", Type: "bool", Description: "Validate and show the request without the mutation."},
		{Name: "yes", Type: "bool", Description: "Approve a destructive command without a prompt."},
		{Name: "quiet", Type: "bool", Description: "Print only the primary resource identifier."},
		{Name: "accessible", Type: "bool", Description: "Use accessible terminal prompts."},
		{Name: "url", Type: "URL", Description: "Override the Longhouse server URL."},
	}
}

func operationFieldHelp(operation *cliOperation) []cliFieldHelp {
	if operation.RequestType.Kind() != reflect.Struct {
		return nil
	}
	defaults := operation.newRequest().Elem()
	return appendOperationFieldHelp(nil, operation, operation.RequestType, defaults, "")
}

func appendOperationFieldHelp(fields []cliFieldHelp, operation *cliOperation, valueType reflect.Type, defaults reflect.Value, jsonPrefix string) []cliFieldHelp {
	valueType = indirectType(valueType)
	defaults = indirectReadValue(defaults)
	for i := 0; i < valueType.NumField(); i++ {
		field := valueType.Field(i)
		name, optional := jsonFieldName(field)
		optional = cliOperationFieldOptional(operation, name, optional)
		if name == "" || operation.HiddenFields[name] {
			continue
		}
		nestedType := indirectType(field.Type)
		if nestedType.Kind() == reflect.Struct {
			var nestedDefaults reflect.Value
			if defaults.IsValid() {
				nestedDefaults = defaults.Field(i)
			}
			fields = appendOperationFieldHelp(fields, operation, nestedType, nestedDefaults, jsonPrefix+name+".")
			continue
		}
		entry := cliFieldHelp{
			Name:        name,
			Type:        cliTypeName(field.Type),
			Required:    !optional && field.Type.Kind() != reflect.Ptr && field.Type.Kind() != reflect.Slice,
			Description: humanizeField(name),
			Enum:        cliEnumValues[name],
		}
		if jsonPrefix != "" {
			entry.JSONPath = jsonPrefix + name
		}
		if name == "status" {
			if operation.RequestType.Name() == "Project" {
				entry.Enum = []string{"active", "archived"}
			} else {
				entry.Enum = []string{"open", "in_progress", "done", "cancelled"}
			}
		}
		if name == "house_id" {
			entry.Required = false
			entry.Description = "House identifier. The active house is the default."
		}
		if name == "owner_member_id" {
			entry.Required = false
			entry.Description = "Owner member identifier. The current member is the default."
		}
		if operation.ID == "GroupService.ListGroupMembers" && name == "member_id" {
			entry.Name = "group_id"
			entry.JSONPath = "member_id"
			entry.Type = "GroupID"
			entry.Description = "Group identifier. The current API names this JSON field member_id."
		}
		var value reflect.Value
		if defaults.IsValid() {
			value = defaults.Field(i)
		}
		if value.IsValid() && !value.IsZero() {
			if encoded, err := json.Marshal(value.Interface()); err == nil {
				entry.Default = encoded
			}
		}
		fields = append(fields, entry)
	}
	return fields
}

func cliOperationFieldOptional(operation *cliOperation, name string, schemaOptional bool) bool {
	if schemaOptional {
		return true
	}
	if operation.ID == "ProjectService.CreateMilestone" || operation.ID == "ProjectService.UpdateMilestone" {
		return name == "when_label" || name == "state" || name == "position"
	}
	return false
}

func responseFieldHelp(responseType reflect.Type) []cliFieldHelp {
	responseType = indirectType(responseType)
	if responseType.Kind() == reflect.Slice || responseType.Kind() == reflect.Array {
		responseType = indirectType(responseType.Elem())
	}
	if responseType.Kind() != reflect.Struct {
		return nil
	}
	fields := make([]cliFieldHelp, 0, responseType.NumField())
	for i := 0; i < responseType.NumField(); i++ {
		field := responseType.Field(i)
		name, optional := jsonFieldName(field)
		if name == "" {
			continue
		}
		fields = append(fields, cliFieldHelp{
			Name: name, Type: cliTypeName(field.Type), Required: !optional,
			Description: humanizeField(name), Enum: cliEnumValues[name],
		})
	}
	return fields
}

func hiddenInputFields(operation *cliOperation) map[string]bool {
	hidden := map[string]bool{
		"created_at": true, "updated_at": true, "audit_id": true,
		"email": true, "handle": true, "last_seen_at": true,
		"deactivated_at": true, "created_by_member_id": true,
		"shared_by": true, "viewer_member_id": true, "cached_public_key": true,
		"deleted_at": true, "recurrence_root_event_id": true,
		"recurrence_root_task_id": true,
	}
	if strings.HasPrefix(operation.Method, "Create") {
		resource := strings.TrimPrefix(operation.Method, "Create")
		idName := snakeCase(resource) + "_id"
		hidden[idName] = true
	}
	if operation.ID == "CommentService.CreateComment" || operation.ID == "CommentService.UpdateComment" {
		hidden["member_id"] = true
	}
	switch operation.ID {
	case "MemberService.CreateMember":
		hidden["avatar_url"] = true
	case "MemberService.UpdateMember":
		hidden["house_id"] = true
		hidden["linkkeys_domain"] = true
		hidden["linkkeys_user_id"] = true
	case "RoleService.UpdateRole", "SkillService.UpdateSkill", "GroupService.UpdateGroup":
		hidden["house_id"] = true
	case "ProjectService.UpdateProject":
		hidden["house_id"] = true
		hidden["visibility"] = true
	case "EventService.CreateEvent", "EventService.UpdateEvent":
		hidden["next_recurrence_at"] = true
		if operation.ID == "EventService.UpdateEvent" {
			hidden["house_id"] = true
			hidden["owner_member_id"] = true
		}
	case "TaskService.UpdateTask":
		hidden["house_id"] = true
		hidden["owner_member_id"] = true
		hidden["assigned_to_skill_id"] = true
		hidden["parent_task_id"] = true
		hidden["visibility"] = true
	case "CommentService.UpdateComment":
		hidden["house_id"] = true
		hidden["target_type"] = true
		hidden["target_id"] = true
	}
	return hidden
}

func deriveCLIPath(service, method string) []string {
	root := strings.TrimSuffix(service, "Service")
	root = strings.ToLower(root)
	words := splitCamel(method)
	if len(words) == 0 {
		return []string{root}
	}
	action := strings.ToLower(words[0])
	objects := words[1:]
	if len(objects) > 0 && strings.EqualFold(objects[0], strings.TrimSuffix(service, "Service")) {
		objects = objects[1:]
	}
	for i := range objects {
		objects[i] = singular(strings.ToLower(objects[i]))
	}
	verb := map[string]string{
		"get": "view", "update": "edit", "create": "create", "delete": "delete",
		"list": "list", "add": "add", "remove": "remove", "set": "set",
		"put": "set", "is": "check", "check": "check", "mark": "mark",
		"query": "list", "report": "report", "restore": "restore", "purge": "purge",
		"grant": "add", "revoke": "remove", "deactivate": "remove", "reactivate": "activate",
	}[action]
	if verb == "" {
		verb = action
	}
	return append(append([]string{root}, objects...), verb)
}

func classifyOperation(method string) (mutation, destructive bool) {
	readPrefixes := []string{"Get", "List", "Is", "Check", "Unread", "Inspect", "Me", "Query"}
	for _, prefix := range readPrefixes {
		if strings.HasPrefix(method, prefix) {
			return false, false
		}
	}
	destructivePrefixes := []string{"Delete", "Remove", "Revoke", "Deny", "Deactivate", "Purge"}
	for _, prefix := range destructivePrefixes {
		if strings.HasPrefix(method, prefix) {
			return true, true
		}
	}
	return true, false
}

func splitCamel(value string) []string {
	var words []string
	start := 0
	runes := []rune(value)
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) && (unicode.IsLower(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
			words = append(words, string(runes[start:i]))
			start = i
		}
	}
	if len(runes) > 0 {
		words = append(words, string(runes[start:]))
	}
	return words
}

func camelToKebab(value string) string {
	words := splitCamel(value)
	for i := range words {
		words[i] = strings.ToLower(words[i])
	}
	return strings.Join(words, "-")
}

func snakeCase(value string) string {
	words := splitCamel(value)
	for i := range words {
		words[i] = strings.ToLower(words[i])
	}
	return strings.Join(words, "_")
}

func singular(value string) string {
	if strings.HasSuffix(value, "ies") {
		return strings.TrimSuffix(value, "ies") + "y"
	}
	if strings.HasSuffix(value, "s") && !strings.HasSuffix(value, "ss") {
		return strings.TrimSuffix(value, "s")
	}
	return value
}

func humanizeWords(words []string) string {
	for i := range words {
		words[i] = strings.ToLower(words[i])
	}
	result := strings.Join(words, " ")
	if result == "" {
		return ""
	}
	return strings.ToUpper(result[:1]) + result[1:] + "."
}

func humanizeField(name string) string {
	words := strings.Fields(strings.ReplaceAll(name, "_", " "))
	if len(words) == 0 {
		return ""
	}
	words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]
	return strings.Join(words, " ") + "."
}

func jsonFieldName(field reflect.StructField) (string, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = snakeCase(field.Name)
	}
	optional := false
	for _, part := range parts[1:] {
		optional = optional || part == "omitempty"
	}
	return name, optional
}

func cliTypeName(value reflect.Type) string {
	if value.Kind() == reflect.Ptr {
		return cliTypeName(value.Elem())
	}
	if value.Kind() == reflect.Slice {
		return "list[" + cliTypeName(value.Elem()) + "]"
	}
	if value.Name() != "" {
		return value.Name()
	}
	return value.Kind().String()
}

var cliEnumValues = map[string][]string{
	"status":          {"open", "in_progress", "done", "cancelled", "active", "archived"},
	"visibility":      {"none", "read", "edit", "full"},
	"access_level":    {"none", "read", "edit", "full"},
	"target_type":     {"event", "task", "project"},
	"resource_type":   {"event", "task", "house"},
	"grantee_type":    {"member", "group"},
	"node_type":       {"task", "project"},
	"type":            {"task", "project"},
	"dependent_type":  {"task", "project"},
	"dependency_type": {"task", "project"},
	"recurrence_freq": {"hourly", "daily", "weekly", "monthly", "quarterly", "yearly"},
	"state":           {"done", "current", "future"},
}

func findOperationByPath(operations []*cliOperation, words []string) (*cliOperation, int) {
	var match *cliOperation
	matchLength := 0
	for _, operation := range operations {
		paths := append([][]string{operation.Path}, operation.Aliases...)
		for _, path := range paths {
			if len(path) <= matchLength || len(words) < len(path) {
				continue
			}
			matched := true
			for i := range path {
				if words[i] != path[i] {
					matched = false
					break
				}
			}
			if matched {
				match = operation
				matchLength = len(path)
			}
		}
	}
	return match, matchLength
}

func findOperationByID(operations []*cliOperation, value string) *cliOperation {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "/", ".")
	for _, operation := range operations {
		candidates := []string{
			operation.ID,
			operation.Service + "." + operation.Operation,
			strings.TrimSuffix(operation.Service, "Service") + "." + operation.Method,
			strings.TrimSuffix(operation.Service, "Service") + "." + operation.Operation,
		}
		serviceResource := strings.TrimSuffix(operation.Service, "Service")
		shortMethod := strings.TrimSuffix(operation.Method, serviceResource)
		if shortMethod != operation.Method {
			candidates = append(candidates, operation.Service+"."+shortMethod, serviceResource+"."+shortMethod)
		}
		for _, candidate := range candidates {
			if strings.ToLower(candidate) == normalized {
				return operation
			}
		}
	}
	return nil
}

func validateRegistry(operations []*cliOperation) error {
	seen := map[string]string{}
	seenIDs := map[string]bool{}
	for _, operation := range operations {
		if seenIDs[operation.ID] {
			return fmt.Errorf("duplicate CLI operation %s", operation.ID)
		}
		seenIDs[operation.ID] = true
		if len(operation.Path) == 0 {
			return fmt.Errorf("CLI operation %s has no command path", operation.ID)
		}
		if operation.RequestType.Kind() == reflect.Struct {
			for _, positional := range operation.Positionals {
				if !cliTypeHasJSONField(operation.RequestType, positional) {
					return fmt.Errorf("CLI operation %s has unknown positional field %q", operation.ID, positional)
				}
				if operation.HiddenFields[positional] {
					return fmt.Errorf("CLI operation %s uses hidden field %q as a positional", operation.ID, positional)
				}
			}
		} else if len(operation.Positionals) > 1 {
			return fmt.Errorf("CLI operation %s has too many positionals for %s", operation.ID, operation.RequestType)
		}
		for _, path := range append([][]string{operation.Path}, operation.Aliases...) {
			key := strings.Join(path, " ")
			if prior, ok := seen[key]; ok {
				return fmt.Errorf("CLI path %q maps to both %s and %s", key, prior, operation.ID)
			}
			seen[key] = operation.ID
		}
	}
	return nil
}

func cliTypeHasJSONField(value reflect.Type, name string) bool {
	value = indirectType(value)
	if value.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldName, _ := jsonFieldName(field)
		if fieldName == name {
			return true
		}
		nested := indirectType(field.Type)
		if nested.Kind() == reflect.Struct && cliTypeHasJSONField(nested, name) {
			return true
		}
	}
	return false
}
