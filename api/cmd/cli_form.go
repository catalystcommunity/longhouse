package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"charm.land/huh/v2"
	longhouseclient "github.com/catalystcommunity/longhouse/clients/go"
)

var cliInputIsTerminal = func() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

type cliFormBinding struct {
	apply func() error
}

func selectCLIHouse(runtime *cliRuntime, houses []longhouseclient.HouseSummary) (string, string, error) {
	selected := ""
	options := make([]huh.Option[string], 0, len(houses))
	for _, house := range houses {
		options = append(options, huh.NewOption(house.Name+" — "+string(house.HouseId), string(house.HouseId)))
	}
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("House").Options(options...).Value(&selected).Filtering(true),
	)).WithInput(runtime.input).WithOutput(runtime.diagnostics).WithAccessible(runtime.accessible)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", "", cliCanceled("house selection canceled")
		}
		return "", "", fmt.Errorf("select a house: %w", err)
	}
	for _, house := range houses {
		if string(house.HouseId) == selected {
			return selected, string(house.MemberId), nil
		}
	}
	return "", "", errors.New("selected house was not found")
}

func runCLIForm(request reflect.Value, operation *cliOperation, options map[string]string, runtime *cliRuntime) error {
	var fields []huh.Field
	var bindings []cliFormBinding
	choices, err := loadCLIFormChoices(runtime, request.Elem(), operation)
	if err != nil {
		return err
	}
	collectCLIFormFields(request.Elem(), operation, "", choices, &fields, &bindings)
	if len(fields) == 0 {
		return nil
	}
	form := huh.NewForm(huh.NewGroup(fields...)).
		WithInput(runtime.input).
		WithOutput(runtime.diagnostics).
		WithAccessible(runtime.accessible).
		WithShowErrors(true)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return cliCanceled("form canceled")
		}
		return fmt.Errorf("run terminal form: %w", err)
	}
	for _, binding := range bindings {
		if err := binding.apply(); err != nil {
			return err
		}
	}
	return nil
}

func collectCLIFormFields(value reflect.Value, operation *cliOperation, prefix string, choices map[string][]huh.Option[string], fields *[]huh.Field, bindings *[]cliFormBinding) {
	value = indirectValue(value)
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < value.NumField(); i++ {
		fieldInfo := value.Type().Field(i)
		name, optional := jsonFieldName(fieldInfo)
		optional = cliOperationFieldOptional(operation, name, optional)
		if name == "" || operation.HiddenFields[name] || formFieldIsFixedIdentifier(operation, name) {
			continue
		}
		field := value.Field(i)
		fieldType := field.Type()
		if fieldType.Kind() == reflect.Struct {
			collectCLIFormFields(field, operation, name+".", choices, fields, bindings)
			continue
		}
		if fieldType.Kind() == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct {
			if field.IsNil() {
				field.Set(reflect.New(fieldType.Elem()))
			}
			collectCLIFormFields(field, operation, name+".", choices, fields, bindings)
			continue
		}
		title := strings.TrimSuffix(humanizeField(prefix+name), ".")
		if operation.ID == "GroupService.ListGroupMembers" && name == "member_id" {
			title = "Group id"
		}
		if resourceChoices := choices[name]; len(resourceChoices) > 0 && fieldType.Kind() == reflect.Slice {
			current := make([]string, 0, field.Len())
			for item := 0; item < field.Len(); item++ {
				current = append(current, formatCLIValue(field.Index(item)))
			}
			*fields = append(*fields, huh.NewMultiSelect[string]().Title(title).Options(resourceChoices...).Value(&current).Filterable(true))
			bindingsValue := field
			*bindings = append(*bindings, cliFormBinding{apply: func() error { return setCLIValue(bindingsValue, strings.Join(current, ",")) }})
			continue
		}
		if resourceChoices := choices[name]; len(resourceChoices) > 0 && strings.HasSuffix(name, "_id") {
			current := ""
			if !fieldIsEmpty(field) {
				current = formatCLIValue(field)
			}
			if optional {
				resourceChoices = append([]huh.Option[string]{huh.NewOption("None", "")}, resourceChoices...)
			}
			if current != "" && !choiceContains(resourceChoices, current) {
				resourceChoices = append([]huh.Option[string]{huh.NewOption(current, current)}, resourceChoices...)
			}
			*fields = append(*fields, huh.NewSelect[string]().Title(title).Options(resourceChoices...).Value(&current).Filtering(true))
			bindingsValue := field
			*bindings = append(*bindings, cliFormBinding{apply: func() error { return setCLIValue(bindingsValue, current) }})
			continue
		}
		enum := cliEnumValues[name]
		if name == "status" {
			if operation.RequestType.Name() == "Project" {
				enum = []string{"active", "archived"}
			} else {
				enum = []string{"open", "in_progress", "done", "cancelled"}
			}
		}
		if len(enum) > 0 {
			current := ""
			if !fieldIsEmpty(field) {
				current = formatCLIValue(field)
			}
			enumOptions := huh.NewOptions(enum...)
			if name == "recurrence_freq" {
				enumOptions = append([]huh.Option[string]{huh.NewOption("None", "")}, enumOptions...)
			}
			selectField := huh.NewSelect[string]().Title(title).Options(enumOptions...).Value(&current)
			*fields = append(*fields, selectField)
			bindingsValue := field
			bindingsName := name
			*bindings = append(*bindings, cliFormBinding{apply: func() error { return setCLIFieldValue(bindingsValue, bindingsName, current) }})
			continue
		}
		baseType := indirectType(fieldType)
		if baseType.Kind() == reflect.Bool {
			current := false
			if !fieldIsEmpty(field) {
				current = indirectReadValue(field).Bool()
			}
			*fields = append(*fields, huh.NewConfirm().Title(title).Value(&current))
			bindingsValue := field
			*bindings = append(*bindings, cliFormBinding{apply: func() error { return setCLIValue(bindingsValue, strconv.FormatBool(current)) }})
			continue
		}
		current := ""
		if !fieldIsEmpty(field) {
			current = formatCLIValue(field)
		}
		validationType := fieldType
		validationName := name
		validate := func(input string) error {
			if !optional && baseType.Kind() == reflect.String && strings.TrimSpace(input) == "" {
				return errors.New("a value is required")
			}
			probe := reflect.New(validationType).Elem()
			if err := setCLIFieldValue(probe, validationName, input); err != nil {
				return err
			}
			if strings.TrimSpace(input) != "" && indirectType(validationType).Name() == "Timestamp" {
				normalized := formatCLIValue(probe)
				if _, err := time.Parse(time.RFC3339, normalized); err != nil {
					return errors.New("enter an RFC3339 timestamp, YYYY-MM-DD, today, or tomorrow")
				}
			}
			return nil
		}
		var formField huh.Field
		if name == "description" || name == "body" || name == "detail" {
			formField = huh.NewText().Title(title).Value(&current).Validate(validate)
		} else {
			inputField := huh.NewInput().Title(title).Value(&current).Validate(validate)
			if strings.Contains(name, "token") || strings.Contains(name, "assertion") || strings.Contains(name, "secret") {
				inputField = inputField.EchoMode(huh.EchoModePassword)
			}
			if len(cliEnumValues[name]) == 0 && (strings.HasSuffix(name, "_id") || name == "house_id") {
				inputField = inputField.Placeholder("Stable resource identifier")
			}
			formField = inputField
		}
		*fields = append(*fields, formField)
		bindingsValue := field
		bindingsName := name
		*bindings = append(*bindings, cliFormBinding{apply: func() error {
			if err := setCLIFieldValue(bindingsValue, bindingsName, current); err != nil {
				return fmt.Errorf("set %s: %w", name, err)
			}
			return nil
		}})
	}
}

func loadCLIFormChoices(runtime *cliRuntime, value reflect.Value, operation *cliOperation) (map[string][]huh.Option[string], error) {
	choices := map[string][]huh.Option[string]{}
	choiceFieldNames := []string{"house_id", "member_id", "owner_member_id", "assignees", "project_id", "task_id", "parent_task_id", "role_id", "skill_id", "assigned_to_skill_id", "group_id"}
	needsChoices := false
	for _, name := range choiceFieldNames {
		if !operation.HiddenFields[name] && requestHasCLIField(value, name) {
			needsChoices = true
			break
		}
	}
	if !needsChoices {
		return choices, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	me, err := longhouseclient.NewAuthClient(runtime.transport).Me(ctx, longhouseclient.EmptyRequest{})
	if err != nil {
		return nil, fmt.Errorf("load form context: %w", err)
	}
	for _, house := range me.Houses {
		choices["house_id"] = append(choices["house_id"], huh.NewOption(house.Name+" — "+string(house.HouseId), string(house.HouseId)))
	}
	if runtime.houseID == "" {
		if field := findCLIField(value, "house_id"); field.IsValid() && !fieldIsEmpty(field) {
			runtime.houseID = formatCLIValue(field)
		}
	}
	if runtime.houseID == "" {
		if err := runtime.resolveHouseContext(); err != nil {
			return nil, err
		}
	}
	houseID := longhouseclient.HouseID(runtime.houseID)
	listRequest := longhouseclient.HouseScopedListRequest{HouseId: houseID}
	if (requestHasCLIField(value, "member_id") && operation.ID != "GroupService.ListGroupMembers") || requestHasCLIField(value, "owner_member_id") || requestHasCLIField(value, "assignees") {
		members, callErr := longhouseclient.NewMemberClient(runtime.transport).ListMembers(ctx, listRequest)
		if callErr != nil {
			return nil, fmt.Errorf("load members for form: %w", callErr)
		}
		for _, member := range members {
			label := string(member.MemberId)
			if member.DisplayName != nil && *member.DisplayName != "" {
				label = *member.DisplayName + " — " + label
			} else if member.Handle != nil && *member.Handle != "" {
				label = "@" + *member.Handle + " — " + label
			}
			option := huh.NewOption(label, string(member.MemberId))
			choices["member_id"] = append(choices["member_id"], option)
			choices["owner_member_id"] = append(choices["owner_member_id"], option)
			choices["assignees"] = append(choices["assignees"], option)
		}
	}
	if requestHasCLIField(value, "project_id") {
		projects, callErr := longhouseclient.NewProjectClient(runtime.transport).ListProjects(ctx, listRequest)
		if callErr != nil {
			return nil, fmt.Errorf("load projects for form: %w", callErr)
		}
		for _, project := range projects.Projects {
			choices["project_id"] = append(choices["project_id"], huh.NewOption(project.Name+" — "+string(project.ProjectId), string(project.ProjectId)))
		}
	}
	if requestHasCLIField(value, "task_id") || requestHasCLIField(value, "parent_task_id") {
		tasks, callErr := longhouseclient.NewTaskClient(runtime.transport).ListTasks(ctx, listRequest)
		if callErr != nil {
			return nil, fmt.Errorf("load tasks for form: %w", callErr)
		}
		for _, task := range tasks.Tasks {
			option := huh.NewOption(task.Title+" — "+string(task.TaskId), string(task.TaskId))
			choices["task_id"] = append(choices["task_id"], option)
			choices["parent_task_id"] = append(choices["parent_task_id"], option)
		}
	}
	if requestHasCLIField(value, "role_id") {
		roles, callErr := longhouseclient.NewRoleClient(runtime.transport).ListRoles(ctx, listRequest)
		if callErr != nil {
			return nil, fmt.Errorf("load roles for form: %w", callErr)
		}
		for _, role := range roles {
			choices["role_id"] = append(choices["role_id"], huh.NewOption(role.Name+" — "+string(role.RoleId), string(role.RoleId)))
		}
	}
	if requestHasCLIField(value, "skill_id") || requestHasCLIField(value, "assigned_to_skill_id") {
		skills, callErr := longhouseclient.NewSkillClient(runtime.transport).ListSkills(ctx, listRequest)
		if callErr != nil {
			return nil, fmt.Errorf("load skills for form: %w", callErr)
		}
		for _, skill := range skills {
			option := huh.NewOption(skill.Name+" — "+string(skill.SkillId), string(skill.SkillId))
			choices["skill_id"] = append(choices["skill_id"], option)
			choices["assigned_to_skill_id"] = append(choices["assigned_to_skill_id"], option)
		}
	}
	if requestHasCLIField(value, "group_id") || operation.ID == "GroupService.ListGroupMembers" {
		groups, callErr := longhouseclient.NewGroupClient(runtime.transport).ListGroups(ctx, listRequest)
		if callErr != nil {
			return nil, fmt.Errorf("load groups for form: %w", callErr)
		}
		for _, group := range groups {
			fieldName := "group_id"
			if operation.ID == "GroupService.ListGroupMembers" {
				fieldName = "member_id"
			}
			choices[fieldName] = append(choices[fieldName], huh.NewOption(group.Name+" — "+string(group.GroupId), string(group.GroupId)))
		}
	}
	return choices, nil
}

func requestHasCLIField(value reflect.Value, name string) bool {
	return findCLIField(value, name).IsValid()
}

func choiceContains(options []huh.Option[string], value string) bool {
	for _, option := range options {
		if option.Value == value {
			return true
		}
	}
	return false
}

func formFieldIsFixedIdentifier(operation *cliOperation, name string) bool {
	if !strings.HasPrefix(operation.Method, "Update") {
		return false
	}
	if operation.ID == "ProjectService.UpdateMilestone" && name == "project_id" {
		return true
	}
	for _, positional := range operation.Positionals {
		if positional == name && strings.HasSuffix(name, "_id") {
			return true
		}
	}
	return false
}

func confirmCLIMutation(operation *cliOperation, options map[string]string, runtime *cliRuntime) error {
	if options["yes"] == "true" {
		return nil
	}
	if options["no-input"] == "true" || !cliInputIsTerminal() {
		return fmt.Errorf("%s is destructive; use --yes to continue", strings.Join(operation.Path, " "))
	}
	confirmed := false
	accessible := options["accessible"] == "true" || environmentEnabled("LONGHOUSE_ACCESSIBLE")
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Continue with " + strings.Join(operation.Path, " ") + "?").Value(&confirmed),
	)).WithInput(runtime.input).WithOutput(runtime.diagnostics).WithAccessible(accessible)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return cliCanceled("operation canceled")
		}
		return err
	}
	if !confirmed {
		return cliCanceled("operation canceled")
	}
	return nil
}

func environmentEnabled(name string) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}
