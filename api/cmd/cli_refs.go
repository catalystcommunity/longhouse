package cmd

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"

	longhouseclient "github.com/catalystcommunity/longhouse/clients/go"
)

type cliReferenceResolver struct {
	runtime       *cliRuntime
	typeOverrides map[string]string
	members       []longhouseclient.Member
	projects      []longhouseclient.Project
	tasks         []longhouseclient.Task
	roles         []longhouseclient.Role
	skills        []longhouseclient.Skill
	groups        []longhouseclient.Group
	events        []longhouseclient.Event
	milestones    []longhouseclient.Milestone
}

func resolveCLIReferences(runtime *cliRuntime, operation *cliOperation, request reflect.Value) error {
	resolver := &cliReferenceResolver{runtime: runtime, typeOverrides: map[string]string{}}
	if operation.ID == "GroupService.ListGroupMembers" {
		resolver.typeOverrides["member_id"] = "GroupID"
	}
	if operation.RequestType.Kind() != reflect.Struct {
		field := request.Elem()
		if field.Kind() != reflect.String || fieldIsEmpty(field) {
			return nil
		}
		resolved, err := resolver.resolveReference(operation.RequestType.Name(), field.String())
		if err != nil {
			return err
		}
		field.SetString(resolved)
		return nil
	}
	return resolver.resolveStruct(request.Elem())
}

func (resolver *cliReferenceResolver) resolveStruct(value reflect.Value) error {
	value = indirectValue(value)
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return nil
	}
	for i := 0; i < value.NumField(); i++ {
		fieldInfo := value.Type().Field(i)
		name, _ := jsonFieldName(fieldInfo)
		field := value.Field(i)
		if fieldInfo.Type.Kind() == reflect.Struct {
			if err := resolver.resolveStruct(field); err != nil {
				return err
			}
			continue
		}
		if fieldInfo.Type.Kind() == reflect.Ptr && fieldInfo.Type.Elem().Kind() == reflect.Struct && !field.IsNil() {
			if err := resolver.resolveStruct(field); err != nil {
				return err
			}
			continue
		}
		if name == "assignees" && field.Kind() == reflect.Slice {
			for item := 0; item < field.Len(); item++ {
				if err := resolver.resolveIDValue(field.Index(item), "MemberID"); err != nil {
					return fmt.Errorf("resolve assignee: %w", err)
				}
			}
			continue
		}
		typeName := resolver.typeOverrides[name]
		if typeName == "" {
			typeName = referenceTypeForField(name)
		}
		if typeName == "" {
			typeName = dynamicReferenceType(value, name)
		}
		if typeName == "" {
			continue
		}
		if err := resolver.resolveIDValue(field, typeName); err != nil {
			return fmt.Errorf("resolve %s: %w", name, err)
		}
	}
	return nil
}

func dynamicReferenceType(value reflect.Value, name string) string {
	var discriminator string
	switch name {
	case "grantee_id":
		discriminator = "grantee_type"
	case "dependent_id":
		discriminator = "dependent_type"
	case "dependency_id":
		discriminator = "dependency_type"
	case "node_id":
		discriminator = "node_type"
	case "resource_id":
		discriminator = "resource_type"
	case "target_id":
		discriminator = "target_type"
	case "id":
		discriminator = "type"
	default:
		return ""
	}
	typeField := findCLIField(value, discriminator)
	if !typeField.IsValid() || fieldIsEmpty(typeField) {
		return ""
	}
	switch strings.ToLower(formatCLIValue(typeField)) {
	case "member":
		return "MemberID"
	case "group":
		return "GroupID"
	case "project":
		return "ProjectID"
	case "task":
		return "TaskID"
	case "event":
		return "EventID"
	case "house":
		return "HouseID"
	default:
		return ""
	}
}

func (resolver *cliReferenceResolver) resolveIDValue(field reflect.Value, typeName string) error {
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil
		}
		field = field.Elem()
	}
	if field.Kind() != reflect.String || field.String() == "" {
		return nil
	}
	resolved, err := resolver.resolveReference(typeName, field.String())
	if err != nil {
		return err
	}
	field.SetString(resolved)
	return nil
}

func (resolver *cliReferenceResolver) resolveReference(typeName, input string) (string, error) {
	query, err := normalizeTypedCLIReference(typeName, input)
	if err != nil {
		return "", err
	}
	if looksLikeStableID(query) {
		return query, nil
	}
	return resolver.resolveType(typeName, query)
}

func expandTypedCLIPositionals(operation *cliOperation, positionals []string) ([]string, error) {
	if len(positionals) == 0 || len(operation.Positionals) < 2 {
		return positionals, nil
	}
	expanded := make([]string, 0, len(operation.Positionals))
	argument := 0
	for field := 0; field < len(operation.Positionals) && argument < len(positionals); field++ {
		fieldName := operation.Positionals[field]
		isTypeIDPair := strings.HasSuffix(fieldName, "_type") && field+1 < len(operation.Positionals) && strings.HasSuffix(operation.Positionals[field+1], "_id")
		isTypeIDPair = isTypeIDPair || (fieldName == "type" && field+1 < len(operation.Positionals) && operation.Positionals[field+1] == "id")
		if isTypeIDPair {
			if referenceType, referenceID, ok := parseTypedCLIReference(positionals[argument]); ok {
				expanded = append(expanded, referenceType, referenceID)
				field++
				argument++
				continue
			}
		}
		expanded = append(expanded, positionals[argument])
		argument++
	}
	if argument < len(positionals) {
		expanded = append(expanded, positionals[argument:]...)
	}
	return expanded, nil
}

func parseTypedCLIReference(input string) (string, string, bool) {
	referenceType, referenceID, ok := strings.Cut(input, ":")
	if !ok || referenceID == "" {
		return "", "", false
	}
	referenceType = strings.ToLower(strings.TrimSpace(referenceType))
	if !containsString([]string{"event", "group", "house", "member", "milestone", "project", "role", "skill", "task"}, referenceType) {
		return "", "", false
	}
	return referenceType, referenceID, true
}

func normalizeTypedCLIReference(typeName, input string) (string, error) {
	referenceType, referenceID, ok := parseTypedCLIReference(input)
	if !ok {
		return input, nil
	}
	expected := strings.ToLower(strings.TrimSuffix(typeName, "ID"))
	if referenceType != expected {
		return "", fmt.Errorf("reference %q has type %s; expected %s", input, referenceType, expected)
	}
	return referenceID, nil
}

func (resolver *cliReferenceResolver) resolveType(typeName, query string) (string, error) {
	if err := resolver.runtime.resolveHouseContext(); err != nil && typeName != "HouseID" {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	limit := uint64(1000)
	request := longhouseclient.HouseScopedListRequest{HouseId: longhouseclient.HouseID(resolver.runtime.houseID), Limit: &limit}
	switch typeName {
	case "HouseID":
		me, err := longhouseclient.NewAuthClient(resolver.runtime.transport).Me(ctx, longhouseclient.EmptyRequest{})
		if err != nil {
			return "", err
		}
		var matches []string
		for _, house := range me.Houses {
			if string(house.HouseId) == query || strings.EqualFold(house.Name, query) {
				matches = append(matches, string(house.HouseId))
			}
		}
		return oneResolvedReference(query, matches)
	case "MemberID":
		if resolver.members == nil {
			values, err := longhouseclient.NewMemberClient(resolver.runtime.transport).ListMembers(ctx, request)
			if err != nil {
				return "", err
			}
			resolver.members = values
		}
		var matches []string
		trimmed := strings.TrimPrefix(query, "@")
		for _, member := range resolver.members {
			match := string(member.MemberId) == query
			match = match || (member.DisplayName != nil && strings.EqualFold(*member.DisplayName, query))
			match = match || (member.Handle != nil && strings.EqualFold(*member.Handle, trimmed))
			if match {
				matches = append(matches, string(member.MemberId))
			}
		}
		return oneResolvedReference(query, matches)
	case "ProjectID":
		if resolver.projects == nil {
			values, err := longhouseclient.NewProjectClient(resolver.runtime.transport).ListProjects(ctx, request)
			if err != nil {
				return "", err
			}
			resolver.projects = values.Projects
		}
		return resolveNamedReference(resolver.projects, "project_id", "name", query)
	case "TaskID":
		if resolver.tasks == nil {
			values, err := longhouseclient.NewTaskClient(resolver.runtime.transport).ListTasks(ctx, request)
			if err != nil {
				return "", err
			}
			resolver.tasks = values.Tasks
		}
		return resolveNamedReference(resolver.tasks, "task_id", "title", query)
	case "RoleID":
		if resolver.roles == nil {
			values, err := longhouseclient.NewRoleClient(resolver.runtime.transport).ListRoles(ctx, request)
			if err != nil {
				return "", err
			}
			resolver.roles = values
		}
		return resolveNamedReference(resolver.roles, "role_id", "name", query)
	case "SkillID":
		if resolver.skills == nil {
			values, err := longhouseclient.NewSkillClient(resolver.runtime.transport).ListSkills(ctx, request)
			if err != nil {
				return "", err
			}
			resolver.skills = values
		}
		return resolveNamedReference(resolver.skills, "skill_id", "name", query)
	case "GroupID":
		if resolver.groups == nil {
			values, err := longhouseclient.NewGroupClient(resolver.runtime.transport).ListGroups(ctx, request)
			if err != nil {
				return "", err
			}
			resolver.groups = values
		}
		return resolveNamedReference(resolver.groups, "group_id", "name", query)
	case "EventID":
		if resolver.events == nil {
			values, err := longhouseclient.NewEventClient(resolver.runtime.transport).ListEvents(ctx, request)
			if err != nil {
				return "", err
			}
			resolver.events = values
		}
		return resolveNamedReference(resolver.events, "event_id", "title", query)
	case "MilestoneID":
		if resolver.milestones == nil {
			projects, err := longhouseclient.NewProjectClient(resolver.runtime.transport).ListProjects(ctx, request)
			if err != nil {
				return "", err
			}
			client := longhouseclient.NewProjectClient(resolver.runtime.transport)
			for _, project := range projects.Projects {
				values, listErr := client.ListMilestones(ctx, project.ProjectId)
				if listErr != nil {
					return "", listErr
				}
				resolver.milestones = append(resolver.milestones, values...)
			}
		}
		return resolveNamedReference(resolver.milestones, "milestone_id", "label", query)
	default:
		return "", fmt.Errorf("%s does not support a name reference", typeName)
	}
}

func referenceTypeForField(name string) string {
	switch name {
	case "house_id":
		return "HouseID"
	case "member_id", "owner_member_id", "subject_member_id", "actor_member_id":
		return "MemberID"
	case "project_id":
		return "ProjectID"
	case "task_id", "parent_task_id":
		return "TaskID"
	case "event_id":
		return "EventID"
	case "role_id":
		return "RoleID"
	case "skill_id", "assigned_to_skill_id":
		return "SkillID"
	case "group_id":
		return "GroupID"
	case "milestone_id":
		return "MilestoneID"
	default:
		return ""
	}
}

func resolveNamedReference[T any](values []T, idField, nameField, query string) (string, error) {
	var matches []string
	for _, value := range values {
		reflected := reflect.ValueOf(value)
		id := findCLIField(reflected, idField)
		name := findCLIField(reflected, nameField)
		if (id.IsValid() && formatCLIValue(id) == query) || (name.IsValid() && strings.EqualFold(formatCLIValue(name), query)) {
			matches = append(matches, formatCLIValue(id))
		}
	}
	return oneResolvedReference(query, matches)
}

func oneResolvedReference(query string, matches []string) (string, error) {
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("resource %q was not found", query)
	}
	return "", fmt.Errorf("resource name %q is not unique; use one of these stable identifiers: %s", query, strings.Join(matches, ", "))
}

func looksLikeStableID(value string) bool {
	if strings.Contains(value, "_") {
		return true
	}
	if len(value) == 36 && strings.Count(value, "-") == 4 {
		return true
	}
	if len(value) != 26 {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) && !(r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}
