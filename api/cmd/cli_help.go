package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	longhouseclient "github.com/catalystcommunity/longhouse/clients/go"
)

var localCLIHelp = []cliCommandHelp{
	{Path: []string{"serve"}, Summary: "Start the Longhouse API server.", Usage: []string{"longhouse serve [server options]"}},
	{Path: []string{"migrate"}, Summary: "Run database migrations.", Usage: []string{"longhouse migrate [--db-uri <uri>]"}, Mutation: true},
	{Path: []string{"auth", "login"}, Summary: "Authorize the CLI in a browser.", Usage: []string{"longhouse auth login --url <url> [--profile <name>] [--no-browser]"}, Mutation: true},
	{Path: []string{"auth", "status"}, Summary: "Show and verify the active session.", Usage: []string{"longhouse auth status [--profile <name>]"}},
	{Path: []string{"auth", "refresh"}, Summary: "Rotate the access bearer and refresh token.", Usage: []string{"longhouse auth refresh [--profile <name>]"}, Mutation: true},
	{Path: []string{"auth", "logout"}, Summary: "Revoke the active session.", Usage: []string{"longhouse auth logout [--profile <name>]"}, Mutation: true},
	{Path: []string{"profile", "list"}, Summary: "List local profiles.", Usage: []string{"longhouse profile list [--output json]"}},
	{Path: []string{"profile", "view"}, Summary: "Show a local profile without secrets.", Usage: []string{"longhouse profile view [<name>] [--output json]"}},
	{Path: []string{"profile", "add"}, Summary: "Add a server profile.", Usage: []string{"longhouse profile add <name> --url <url> [--house <house>]"}, Mutation: true},
	{Path: []string{"profile", "use"}, Summary: "Select the active profile.", Usage: []string{"longhouse profile use <name>"}, Mutation: true},
	{Path: []string{"profile", "remove"}, Summary: "Remove a profile and its credentials.", Usage: []string{"longhouse profile remove <name> --yes"}, Mutation: true, Destructive: true},
	{Path: []string{"profile", "house", "use"}, Summary: "Select the active house for a profile.", Usage: []string{"longhouse profile house use <house> [--profile <name>]"}, Mutation: true},
	{Path: []string{"task", "start"}, Summary: "Set a task status to in_progress.", Usage: []string{"longhouse task start <task-id>"}, Mutation: true},
	{Path: []string{"task", "done"}, Summary: "Set a task status to done.", Usage: []string{"longhouse task done <task-id>"}, Mutation: true},
	{Path: []string{"task", "cancel"}, Summary: "Set a task status to cancelled.", Usage: []string{"longhouse task cancel <task-id>"}, Mutation: true},
	{Path: []string{"task", "reopen"}, Summary: "Set a task status to open.", Usage: []string{"longhouse task reopen <task-id>"}, Mutation: true},
	{Path: []string{"project", "archive"}, Summary: "Set a project status to archived.", Usage: []string{"longhouse project archive <project-id>"}, Mutation: true},
	{Path: []string{"project", "activate"}, Summary: "Set a project status to active.", Usage: []string{"longhouse project activate <project-id>"}, Mutation: true},
	{Path: []string{"role", "view"}, Summary: "Show one role.", Usage: []string{"longhouse role view <role-id-or-name>"}},
	{Path: []string{"skill", "view"}, Summary: "Show one skill.", Usage: []string{"longhouse skill view <skill-id-or-name>"}},
	{Path: []string{"group", "view"}, Summary: "Show one group.", Usage: []string{"longhouse group view <group-id-or-name>"}},
	{Path: []string{"task", "visibility", "get"}, Summary: "Show task visibility.", Usage: []string{"longhouse task visibility get <task-id>"}},
	{Path: []string{"project", "visibility", "get"}, Summary: "Show project visibility.", Usage: []string{"longhouse project visibility get <project-id>"}},
	{Path: []string{"calendar", "subscription", "list"}, Summary: "List calendar subscriptions.", Usage: []string{"longhouse calendar subscription list"}},
	{Path: []string{"calendar", "subscription", "create"}, Summary: "Add or enable a calendar subscription.", Usage: []string{"longhouse calendar subscription create <member-id> [--disabled]"}, Mutation: true},
	{Path: []string{"calendar", "subscription", "delete"}, Summary: "Remove a calendar subscription.", Usage: []string{"longhouse calendar subscription delete <member-id> [--yes]"}, Mutation: true, Destructive: true},
	{Path: []string{"api", "list"}, Summary: "List all generated CSIL operations.", Usage: []string{"longhouse api list [--output json]"}},
	{Path: []string{"api", "describe"}, Summary: "Describe one generated CSIL operation.", Usage: []string{"longhouse api describe <service.operation> [--output json]"}},
	{Path: []string{"api", "call"}, Summary: "Call one generated CSIL operation.", Usage: []string{"longhouse api call <service.operation> --input <file> [--output json]"}, Mutation: true},
	{Path: []string{"completion"}, Summary: "Generate shell completion.", Usage: []string{"longhouse completion <bash|zsh|fish>"}},
	{Path: []string{"agent", "instructions"}, Summary: "Print concise instructions for an automated caller.", Usage: []string{"longhouse agent instructions [--json]"}},
	{Path: []string{"help"}, Summary: "Show command help.", Usage: []string{"longhouse help [<topic>...] [--json]"}},
}

var cliGroupSummaries = map[string]string{
	"access":       "Manage shares and resource access.",
	"admin":        "Manage trusted domains, settings, audit records, and trash.",
	"agent":        "Show instructions for automated callers.",
	"api":          "Discover and call generated CSIL operations.",
	"auth":         "Sign in and manage login sessions.",
	"bug":          "Report product problems.",
	"calendar":     "Manage calendar views and subscriptions.",
	"comment":      "Manage comments on tasks and projects.",
	"completion":   "Generate shell completion.",
	"dependency":   "Manage task and project dependencies.",
	"event":        "Manage calendar events.",
	"group":        "Manage groups, members, and skills.",
	"help":         "Show command help.",
	"house":        "Manage houses.",
	"member":       "Manage members, roles, and skills.",
	"migrate":      "Run database migrations.",
	"notification": "Read and manage notifications.",
	"profile":      "Manage local server profiles.",
	"project":      "Manage projects and their resources.",
	"role":         "Manage roles.",
	"serve":        "Start the Longhouse API server.",
	"skill":        "Manage skills.",
	"task":         "Manage tasks.",
}

func renderCLIHelp(operations []*cliOperation, topic []string, options map[string]string) error {
	commands := append([]cliCommandHelp(nil), localCLIHelp...)
	for i := range commands {
		enrichLocalCLIHelp(&commands[i])
	}
	for _, operation := range operations {
		if operation.Service == "DevAuthService" || (operation.Service == "AuthService" && operation.Method != "ListSessions" && operation.Method != "RevokeSession") {
			continue
		}
		commands = append(commands, operation.help())
	}
	sort.Slice(commands, func(i, j int) bool {
		return strings.Join(commands[i].Path, " ") < strings.Join(commands[j].Path, " ")
	})
	populateCLIHelpRelations(commands)
	filtered := make([]cliCommandHelp, 0, len(commands))
	for _, command := range commands {
		if pathHasPrefix(command.Path, topic) {
			filtered = append(filtered, command)
		}
	}
	if len(filtered) == 0 {
		return fmt.Errorf("no help topic matches %q", strings.Join(topic, " "))
	}
	if options["output"] == "json" {
		return writeCLIJSON(os.Stdout, cliHelpDocument{SchemaVersion: structuredHelpVersion, Topic: topic, Commands: compactCLIHelp(filtered, topic)})
	}
	if len(topic) == 0 {
		fmt.Println("longhouse - coordination from the command line")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  longhouse <noun> <verb> [arguments] [options]")
		fmt.Println("  longhouse help [<topic>...] [--json]")
		fmt.Println()
		fmt.Println("Command groups:")
		groups := map[string]string{}
		for _, command := range filtered {
			if _, exists := groups[command.Path[0]]; !exists {
				groups[command.Path[0]] = cliGroupSummary(command.Path[0])
			}
		}
		groupNames := make([]string, 0, len(groups))
		for name := range groups {
			groupNames = append(groupNames, name)
		}
		sort.Strings(groupNames)
		for _, name := range groupNames {
			fmt.Printf("  %-14s %s\n", name, groups[name])
		}
		fmt.Println()
		fmt.Println("Use 'longhouse help <group>' for commands in one group.")
		return nil
	}
	if len(filtered) == 1 && len(filtered[0].Path) == len(topic) {
		return renderCLICommandHelp(filtered[0])
	}
	fmt.Printf("Commands under %s:\n", strings.Join(topic, " "))
	for _, command := range filtered {
		fmt.Printf("  %-38s %s\n", strings.Join(command.Path, " "), command.Summary)
	}
	return nil
}

var localCLIArguments = map[string][]string{
	"profile view": {"name"}, "profile add": {"name"},
	"profile use": {"name"}, "profile remove": {"name"}, "profile house use": {"house"},
	"task start": {"task_id"}, "task done": {"task_id"}, "task cancel": {"task_id"}, "task reopen": {"task_id"},
	"project archive": {"project_id"}, "project activate": {"project_id"},
	"role view": {"role_id_or_name"}, "skill view": {"skill_id_or_name"}, "group view": {"group_id_or_name"},
	"task visibility get": {"task_id_or_name"}, "project visibility get": {"project_id_or_name"},
	"calendar subscription create": {"member_id_or_name"}, "calendar subscription delete": {"member_id_or_name"},
	"api describe": {"service_operation"}, "api call": {"service_operation"},
	"completion": {"shell"}, "help": {"topic"},
}

var localCLIOptionNames = map[string][]string{
	"auth login":  {"url", "profile", "client-name", "no-browser", "output", "json"},
	"auth status": {"url", "profile", "output", "json"}, "auth refresh": {"url", "profile", "output", "json"},
	"auth logout":  {"url", "profile", "output", "json"},
	"profile list": {"output", "json"}, "profile view": {"output", "json"},
	"profile add": {"url", "house", "output", "json"}, "profile use": {"output", "json"},
	"profile remove":               {"yes", "no-input", "accessible", "output", "json"},
	"profile house use":            {"profile", "output", "json"},
	"task start":                   {"profile", "house", "url", "dry-run", "output", "json", "quiet"},
	"task done":                    {"profile", "house", "url", "dry-run", "output", "json", "quiet"},
	"task cancel":                  {"profile", "house", "url", "dry-run", "output", "json", "quiet"},
	"task reopen":                  {"profile", "house", "url", "dry-run", "output", "json", "quiet"},
	"project archive":              {"profile", "house", "url", "dry-run", "output", "json", "quiet"},
	"project activate":             {"profile", "house", "url", "dry-run", "output", "json", "quiet"},
	"calendar subscription create": {"disabled", "profile", "house", "url", "dry-run", "output", "json"},
	"calendar subscription delete": {"yes", "no-input", "accessible", "profile", "house", "url", "dry-run", "output", "json"},
	"api list":                     {"output", "json"}, "api describe": {"output", "json"},
	"api call":           {"input", "profile", "house", "url", "dry-run", "yes", "no-input", "output", "json", "quiet"},
	"agent instructions": {"output", "json"}, "help": {"output", "json"},
}

func enrichLocalCLIHelp(command *cliCommandHelp) {
	key := strings.Join(command.Path, " ")
	if len(command.Examples) == 0 && len(command.Usage) > 0 {
		command.Examples = []string{command.Usage[0]}
	}
	command.Effect = "read"
	if command.Mutation {
		command.Effect = "mutation"
	}
	if command.Destructive {
		command.Effect = "destructive mutation"
	}
	if len(command.Arguments) == 0 {
		command.Arguments = localCLIArguments[key]
	}
	if len(command.Options) != 0 {
		return
	}
	names := localCLIAllowedOptionNames(key)
	catalog := map[string]cliFieldHelp{}
	for _, option := range globalCLIFieldHelp() {
		catalog[option.Name] = option
	}
	catalog["client-name"] = cliFieldHelp{Name: "client-name", Type: "string", Description: "Set the name shown during browser approval."}
	catalog["no-browser"] = cliFieldHelp{Name: "no-browser", Type: "bool", Description: "Do not open a browser."}
	catalog["disabled"] = cliFieldHelp{Name: "disabled", Type: "bool", Description: "Create the subscription in a disabled state."}
	for _, name := range names {
		if option, ok := catalog[name]; ok {
			command.Options = append(command.Options, option)
		}
	}
	if containsString(names, "url") || containsString(names, "profile") || containsString(names, "house") {
		command.Environment = []string{"LONGHOUSE_URL", "LONGHOUSE_PROFILE", "LONGHOUSE_HOUSE", "LONGHOUSE_ACCESSIBLE"}
	}
}

func localCLIAllowedOptionNames(key string) []string {
	names := append([]string(nil), localCLIOptionNames[key]...)
	if len(names) == 0 && (key == "role view" || key == "skill view" || key == "group view" || strings.HasSuffix(key, "visibility get") || key == "calendar subscription list") {
		names = []string{"profile", "house", "url", "output", "json"}
	}
	if containsString(names, "house") || containsString(names, "profile") {
		for _, name := range []string{"no-input", "accessible"} {
			if !containsString(names, name) {
				names = append(names, name)
			}
		}
	}
	return names
}

func validateLocalCLIOptions(parsed parsedCLIArgs) error {
	best := ""
	bestLength := 0
	for _, command := range localCLIHelp {
		if len(command.Path) > bestLength && pathHasPrefix(parsed.Words, command.Path) {
			best = strings.Join(command.Path, " ")
			bestLength = len(command.Path)
		}
	}
	if best == "" || best == "api call" {
		return nil
	}
	allowed := localCLIAllowedOptionNames(best)
	for option := range parsed.Options {
		if !containsString(allowed, option) {
			return fmt.Errorf("unknown option --%s for %s", option, best)
		}
	}
	return nil
}

func populateCLIHelpRelations(commands []cliCommandHelp) {
	for i := range commands {
		if len(commands[i].Path) == 0 {
			continue
		}
		for j := range commands {
			if i == j || len(commands[j].Path) == 0 || commands[i].Path[0] != commands[j].Path[0] {
				continue
			}
			commands[i].Related = append(commands[i].Related, commands[j].Path)
		}
	}
}

func compactCLIHelp(commands []cliCommandHelp, topic []string) []cliCommandHelp {
	if len(topic) > 0 {
		for _, command := range commands {
			if len(command.Path) == len(topic) {
				return []cliCommandHelp{command}
			}
		}
		compact := make([]cliCommandHelp, 0, len(commands))
		for _, command := range commands {
			compact = append(compact, cliCommandHelp{
				Path: command.Path, Summary: command.Summary,
				Mutation: command.Mutation, Destructive: command.Destructive,
			})
		}
		return compact
	}
	byGroup := map[string]cliCommandHelp{}
	for _, command := range commands {
		if len(command.Path) == 0 {
			continue
		}
		group := command.Path[0]
		entry := byGroup[group]
		entry.Path = []string{group}
		if entry.Summary == "" {
			entry.Summary = cliGroupSummary(group)
		}
		byGroup[group] = entry
	}
	groups := make([]string, 0, len(byGroup))
	for group := range byGroup {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	compact := make([]cliCommandHelp, 0, len(groups))
	for _, group := range groups {
		compact = append(compact, byGroup[group])
	}
	return compact
}

func cliGroupSummary(group string) string {
	if summary := cliGroupSummaries[group]; summary != "" {
		return summary
	}
	return "Manage " + group + " resources."
}

func renderCLICommandHelp(command cliCommandHelp) error {
	fmt.Println(command.Summary)
	fmt.Println()
	fmt.Println("Usage:")
	for _, usage := range command.Usage {
		fmt.Println("  " + usage)
	}
	if len(command.Options) > 0 {
		fmt.Println()
		fmt.Println("Request options:")
		for _, option := range command.Options {
			required := ""
			if option.Required {
				required = " (required)"
			}
			allowed := ""
			if len(option.Enum) > 0 {
				allowed = " [" + strings.Join(option.Enum, "|") + "]"
			}
			jsonPath := ""
			if option.JSONPath != "" {
				jsonPath = " [JSON: " + option.JSONPath + "]"
			}
			fmt.Printf("  --%-28s %s%s%s%s\n", strings.ReplaceAll(option.Name, "_", "-"), option.Type, required, allowed, jsonPath)
		}
	}
	if command.Service != "" {
		fmt.Printf("\nCSIL: %s/%s\n", command.Service, command.Operation)
	}
	return nil
}

func pathHasPrefix(path, prefix []string) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i := range prefix {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}

func runRawAPICommand(operations []*cliOperation, parsed parsedCLIArgs) error {
	if len(parsed.Words) < 2 {
		return errors.New("api requires one of: list, describe, call")
	}
	switch parsed.Words[1] {
	case "list":
		help := make([]cliCommandHelp, 0, len(operations))
		for _, operation := range operations {
			detail := operation.help()
			help = append(help, cliCommandHelp{
				Path: detail.Path, Summary: detail.Summary, Service: detail.Service,
				Operation: detail.Operation, RequestType: detail.RequestType,
				ResponseType: detail.ResponseType, Mutation: detail.Mutation,
				Destructive: detail.Destructive,
			})
		}
		if parsed.Options["output"] == "json" {
			return writeCLIJSON(os.Stdout, cliHelpDocument{SchemaVersion: structuredHelpVersion, Topic: []string{"api"}, Commands: help})
		}
		for _, operation := range operations {
			fmt.Printf("%-48s %-32s %s -> %s\n", operation.ID, operation.Operation, operation.RequestType, operation.ResponseType)
		}
		return nil
	case "describe":
		if len(parsed.Words) < 3 {
			return errors.New("api describe requires a service and operation")
		}
		operation := findOperationByID(operations, parsed.Words[2])
		if operation == nil {
			return fmt.Errorf("unknown CSIL operation %q", parsed.Words[2])
		}
		if parsed.Options["output"] == "json" {
			return writeCLIJSON(os.Stdout, cliHelpDocument{SchemaVersion: structuredHelpVersion, Topic: []string{"api", "describe", parsed.Words[2]}, Commands: []cliCommandHelp{operation.help()}})
		}
		return renderCLICommandHelp(operation.help())
	case "call":
		if len(parsed.Words) < 3 {
			return errors.New("api call requires a service and operation")
		}
		operation := findOperationByID(operations, parsed.Words[2])
		if operation == nil {
			return fmt.Errorf("unknown CSIL operation %q", parsed.Words[2])
		}
		return executeCLIOperation(operation, parsed.Words[3:], parsed.Options, reflect.Value{})
	default:
		return fmt.Errorf("unknown api command %q", parsed.Words[1])
	}
}

func isWorkflowCommand(words []string) bool {
	if len(words) < 2 {
		return false
	}
	if words[0] == "task" {
		return words[1] == "start" || words[1] == "done" || words[1] == "cancel" || words[1] == "reopen"
	}
	return words[0] == "project" && (words[1] == "archive" || words[1] == "activate")
}

func runWorkflowCommand(_ []*cliOperation, parsed parsedCLIArgs) error {
	if len(parsed.Words) != 3 {
		return fmt.Errorf("usage: longhouse %s %s <id>", parsed.Words[0], parsed.Words[1])
	}
	runtime, err := newCLIRuntime(parsed.Options)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	id := parsed.Words[2]
	var result any
	if parsed.Words[0] == "task" {
		id, err = (&cliReferenceResolver{runtime: runtime, typeOverrides: map[string]string{}}).resolveReference("TaskID", id)
		if err != nil {
			return err
		}
		taskClient := longhouseclient.NewTaskClient(runtime.transport)
		task, err := taskClient.GetTask(ctx, longhouseclient.TaskID(id))
		if err != nil {
			return err
		}
		status := map[string]string{"start": "in_progress", "done": "done", "cancel": "cancelled", "reopen": "open"}[parsed.Words[1]]
		value := longhouseclient.TaskStatus(status)
		task.Status = &value
		if err := task.Validate(); err != nil {
			return err
		}
		if parsed.Options["dry-run"] == "true" {
			return renderCLIResult(runtime.output, task, parsed.Options, false)
		}
		result, err = taskClient.UpdateTask(ctx, task)
		if err != nil {
			return err
		}
	} else {
		id, err = (&cliReferenceResolver{runtime: runtime, typeOverrides: map[string]string{}}).resolveReference("ProjectID", id)
		if err != nil {
			return err
		}
		projectClient := longhouseclient.NewProjectClient(runtime.transport)
		project, err := projectClient.GetProject(ctx, longhouseclient.ProjectID(id))
		if err != nil {
			return err
		}
		status := map[string]string{"archive": "archived", "activate": "active"}[parsed.Words[1]]
		value := longhouseclient.ProjectStatus(status)
		project.Status = &value
		if err := project.Validate(); err != nil {
			return err
		}
		if parsed.Options["dry-run"] == "true" {
			return renderCLIResult(runtime.output, project, parsed.Options, false)
		}
		result, err = projectClient.UpdateProject(ctx, project)
		if err != nil {
			return err
		}
	}
	return renderCLIResult(runtime.output, result, parsed.Options, true)
}

func runCompletionCommand(operations []*cliOperation, parsed parsedCLIArgs) error {
	if len(parsed.Words) != 2 {
		return errors.New("usage: longhouse completion <bash|zsh|fish>")
	}
	paths := make([]string, 0, len(operations)+len(localCLIHelp))
	for _, operation := range operations {
		if operation.Service == "DevAuthService" || (operation.Service == "AuthService" && operation.Method != "ListSessions" && operation.Method != "RevokeSession") {
			continue
		}
		paths = append(paths, strings.Join(operation.Path, " "))
	}
	for _, command := range localCLIHelp {
		paths = append(paths, strings.Join(command.Path, " "))
	}
	sort.Strings(paths)
	words := strings.Join(paths, " ")
	switch parsed.Words[1] {
	case "bash":
		fmt.Printf("complete -W %q longhouse\n", words)
	case "zsh":
		fmt.Printf("#compdef longhouse\n_arguments '*:command:(%s)'\n", strings.Join(paths, " "))
	case "fish":
		for _, path := range paths {
			fmt.Printf("complete -c longhouse -a %q\n", path)
		}
	default:
		return fmt.Errorf("unsupported shell %q", parsed.Words[1])
	}
	return nil
}

func runAgentInstructions(operations []*cliOperation, parsed parsedCLIArgs) error {
	if len(parsed.Words) != 2 || parsed.Words[1] != "instructions" {
		return errors.New("usage: longhouse agent instructions [--json]")
	}
	groups := map[string]bool{}
	for _, operation := range operations {
		if operation.Service != "DevAuthService" && len(operation.Path) > 0 {
			groups[operation.Path[0]] = true
		}
	}
	groupNames := make([]string, 0, len(groups))
	for group := range groups {
		groupNames = append(groupNames, group)
	}
	sort.Strings(groupNames)
	instructions := []string{
		"Use `longhouse help --json` to discover top-level commands.",
		"Use `longhouse help <path> --json` before you run an unfamiliar command.",
		"Use `--output json` for results that another program will read.",
		"Use `--input -` for a JSON request from standard input.",
		"Use `--no-input` in automation. Add `--yes` to an approved destructive command.",
		"Use stable resource identifiers from prior results. Exact unique names are also accepted for common resources.",
		"Do not retry a mutation after an uncertain transport failure.",
		"Use `longhouse api list --json` when no polished command covers an operation.",
	}
	if parsed.Options["output"] == "json" {
		return renderCLIResult(os.Stdout, map[string]any{
			"instructions":        instructions,
			"command_groups":      groupNames,
			"help_schema_version": structuredHelpVersion,
		}, parsed.Options, false)
	}
	fmt.Println("# Longhouse CLI instructions")
	fmt.Println()
	for _, instruction := range instructions {
		fmt.Println("- " + instruction)
	}
	fmt.Println()
	fmt.Println("Command groups: " + strings.Join(groupNames, ", "))
	return nil
}
