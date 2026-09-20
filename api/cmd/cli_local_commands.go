package cmd

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	longhouseclient "github.com/catalystcommunity/longhouse/clients/go"
)

func isLocalResourceCommand(words []string) bool {
	if len(words) >= 2 && (words[0] == "role" || words[0] == "skill" || words[0] == "group") && words[1] == "view" {
		return true
	}
	if len(words) >= 3 && (words[0] == "task" || words[0] == "project") && words[1] == "visibility" && words[2] == "get" {
		return true
	}
	return len(words) >= 3 && words[0] == "calendar" && words[1] == "subscription"
}

func runLocalResourceCommand(parsed parsedCLIArgs) error {
	runtime, err := newCLIRuntime(parsed.Options)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if parsed.Words[0] == "calendar" {
		return runCalendarSubscriptionCommand(ctx, runtime, parsed)
	}
	if len(parsed.Words) >= 3 && parsed.Words[1] == "visibility" {
		if len(parsed.Words) != 4 {
			return fmt.Errorf("usage: longhouse %s visibility get <%s-id>", parsed.Words[0], parsed.Words[0])
		}
		var result any
		id := parsed.Words[3]
		if parsed.Words[0] == "task" {
			id, err = (&cliReferenceResolver{runtime: runtime, typeOverrides: map[string]string{}}).resolveReference("TaskID", id)
			if err != nil {
				return err
			}
			value, callErr := longhouseclient.NewTaskClient(runtime.transport).GetTask(ctx, longhouseclient.TaskID(id))
			if callErr != nil {
				return callErr
			}
			result = map[string]any{"task_id": value.TaskId, "visibility": value.Visibility}
		} else {
			id, err = (&cliReferenceResolver{runtime: runtime, typeOverrides: map[string]string{}}).resolveReference("ProjectID", id)
			if err != nil {
				return err
			}
			value, callErr := longhouseclient.NewProjectClient(runtime.transport).GetProject(ctx, longhouseclient.ProjectID(id))
			if callErr != nil {
				return callErr
			}
			result = map[string]any{"project_id": value.ProjectId, "visibility": value.Visibility}
		}
		return renderCLIResult(runtime.output, result, parsed.Options, false)
	}
	if len(parsed.Words) != 3 {
		return fmt.Errorf("usage: longhouse %s view <%s-id-or-name>", parsed.Words[0], parsed.Words[0])
	}
	if err := runtime.resolveHouseContext(); err != nil {
		return err
	}
	request := longhouseclient.HouseScopedListRequest{HouseId: longhouseclient.HouseID(runtime.houseID)}
	query := parsed.Words[2]
	var result any
	switch parsed.Words[0] {
	case "role":
		values, callErr := longhouseclient.NewRoleClient(runtime.transport).ListRoles(ctx, request)
		if callErr != nil {
			return callErr
		}
		result, err = findNamedCLIResource(values, "role_id", "name", query)
	case "skill":
		values, callErr := longhouseclient.NewSkillClient(runtime.transport).ListSkills(ctx, request)
		if callErr != nil {
			return callErr
		}
		result, err = findNamedCLIResource(values, "skill_id", "name", query)
	case "group":
		values, callErr := longhouseclient.NewGroupClient(runtime.transport).ListGroups(ctx, request)
		if callErr != nil {
			return callErr
		}
		result, err = findNamedCLIResource(values, "group_id", "name", query)
	}
	if err != nil {
		return err
	}
	return renderCLIResult(runtime.output, result, parsed.Options, false)
}

func runCalendarSubscriptionCommand(ctx context.Context, runtime *cliRuntime, parsed parsedCLIArgs) error {
	if err := runtime.resolveHouseContext(); err != nil {
		return err
	}
	client := longhouseclient.NewEventClient(runtime.transport)
	view, err := client.GetCalendarView(ctx, longhouseclient.HouseID(runtime.houseID))
	if err != nil {
		return err
	}
	verb := parsed.Words[2]
	switch verb {
	case "list":
		if len(parsed.Words) != 3 {
			return errors.New("usage: longhouse calendar subscription list")
		}
		return renderCLIResult(runtime.output, view.Subscriptions, parsed.Options, false)
	case "create":
		if len(parsed.Words) != 4 {
			return errors.New("usage: longhouse calendar subscription create <member-id> [--disabled]")
		}
		memberRef := parsed.Words[3]
		memberRef, err = (&cliReferenceResolver{runtime: runtime, typeOverrides: map[string]string{}}).resolveReference("MemberID", memberRef)
		if err != nil {
			return err
		}
		memberID := longhouseclient.MemberID(memberRef)
		enabled := parsed.Options["disabled"] != "true"
		found := false
		for i := range view.Subscriptions {
			if view.Subscriptions[i].SubjectMemberId == memberID {
				view.Subscriptions[i].Enabled = enabled
				found = true
			}
		}
		if !found {
			view.Subscriptions = append(view.Subscriptions, longhouseclient.CalendarSubscription{SubjectMemberId: memberID, Enabled: enabled})
		}
	case "delete":
		if len(parsed.Words) != 4 {
			return errors.New("usage: longhouse calendar subscription delete <member-id> [--yes]")
		}
		memberRef := parsed.Words[3]
		memberRef, err = (&cliReferenceResolver{runtime: runtime, typeOverrides: map[string]string{}}).resolveReference("MemberID", memberRef)
		if err != nil {
			return err
		}
		memberID := longhouseclient.MemberID(memberRef)
		filtered := view.Subscriptions[:0]
		for _, subscription := range view.Subscriptions {
			if subscription.SubjectMemberId != memberID {
				filtered = append(filtered, subscription)
			}
		}
		view.Subscriptions = filtered
	default:
		return fmt.Errorf("unknown calendar subscription command %q", verb)
	}
	if parsed.Options["dry-run"] == "true" {
		return renderCLIResult(runtime.output, view, parsed.Options, false)
	}
	if verb == "delete" {
		if err := confirmCLIMutation(&cliOperation{Path: []string{"calendar", "subscription", "delete"}, Destructive: true}, parsed.Options, runtime); err != nil {
			return err
		}
	}
	updated, err := client.SetCalendarView(ctx, view)
	if err != nil {
		return err
	}
	return renderCLIResult(runtime.output, updated, parsed.Options, true)
}

func findNamedCLIResource[T any](values []T, idField, nameField, query string) (T, error) {
	var matches []T
	for _, value := range values {
		reflected := reflect.ValueOf(value)
		id := findCLIField(reflected, idField)
		name := findCLIField(reflected, nameField)
		if (id.IsValid() && formatCLIValue(id) == query) || (name.IsValid() && strings.EqualFold(formatCLIValue(name), query)) {
			matches = append(matches, value)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	var zero T
	if len(matches) == 0 {
		return zero, fmt.Errorf("resource %q was not found", query)
	}
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, formatCLIValue(findCLIField(reflect.ValueOf(match), idField)))
	}
	return zero, fmt.Errorf("resource name %q is not unique; use one of these stable identifiers: %s", query, strings.Join(ids, ", "))
}
