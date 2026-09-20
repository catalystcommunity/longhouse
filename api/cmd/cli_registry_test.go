package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	longhouseclient "github.com/catalystcommunity/longhouse/clients/go"
)

func TestCLIRegistryCoversGeneratedOperations(t *testing.T) {
	operations := buildCLIOperations()
	if err := validateRegistry(operations); err != nil {
		t.Fatalf("validateRegistry: %v", err)
	}
	if len(operations) < 100 {
		t.Fatalf("operation count = %d, want at least 100", len(operations))
	}
	taskCreate := findOperationByID(operations, "TaskService.Create")
	if taskCreate == nil {
		t.Fatal("short API name TaskService.Create did not resolve")
	}
	if got := strings.Join(taskCreate.Path, " "); got != "task create" {
		t.Fatalf("task create path = %q", got)
	}
	byPath, consumed := findOperationByPath(operations, []string{"task", "create", "Repair gate"})
	if byPath != taskCreate || consumed != 2 {
		t.Fatalf("path lookup = %#v, %d", byPath, consumed)
	}
}

func TestCompactCLIHelpUsesProgressiveDiscovery(t *testing.T) {
	commands := []cliCommandHelp{
		{Path: []string{"task", "create"}, Summary: "Create task.", Options: []cliFieldHelp{{Name: "title"}}},
		{Path: []string{"task", "list"}, Summary: "List tasks."},
		{Path: []string{"project", "list"}, Summary: "List projects."},
	}
	root := compactCLIHelp(commands, nil)
	if len(root) != 2 || len(root[0].Path) != 1 {
		t.Fatalf("root help = %#v", root)
	}
	task := compactCLIHelp(commands[:2], []string{"task"})
	if len(task) != 2 || len(task[0].Options) != 0 {
		t.Fatalf("task help = %#v", task)
	}
	create := compactCLIHelp(commands[:1], []string{"task", "create"})
	if len(create) != 1 || len(create[0].Options) != 1 {
		t.Fatalf("exact help = %#v", create)
	}
}

func TestCLIHelpDescribesNestedInputsAndOutputs(t *testing.T) {
	operations := buildCLIOperations()
	settings := findOperationByID(operations, "SettingsService.UpdateSettings").help()
	if !helpHasField(settings.Options, "bug_reports_enabled") {
		t.Fatalf("settings options = %#v", settings.Options)
	}
	for _, field := range settings.Options {
		if field.Name == "bug_reports_enabled" && field.JSONPath != "settings.bug_reports_enabled" {
			t.Fatalf("settings JSON path = %q", field.JSONPath)
		}
	}
	task := findOperationByID(operations, "TaskService.CreateTask").help()
	if !helpHasField(task.OutputFields, "task_id") || task.Effect != "mutation" || len(task.Environment) == 0 {
		t.Fatalf("task help = %#v", task)
	}
}

func helpHasField(fields []cliFieldHelp, name string) bool {
	for _, field := range fields {
		if field.Name == name {
			return true
		}
	}
	return false
}

func TestCLIRegistryUsesGeneratedDefaultsAndValidation(t *testing.T) {
	operation := findOperationByID(buildCLIOperations(), "TaskService.CreateTask")
	request := operation.newRequest()
	task := request.Interface().(*longhouseclient.Task)
	if task.Status == nil || *task.Status != longhouseclient.TaskStatus("open") {
		t.Fatalf("task status default = %#v", task.Status)
	}
	if task.Visibility == nil || *task.Visibility != longhouseclient.AccessLevel("read") {
		t.Fatalf("task visibility default = %#v", task.Visibility)
	}
	if err := operation.validate(request); err == nil {
		t.Fatal("generated validation accepted an empty title")
	}
	task.Title = "Repair gate"
	if err := operation.validate(request); err != nil {
		t.Fatalf("generated validation rejected a title: %v", err)
	}
}

func TestMilestoneDefaultsDoNotRequireDisplayOnlyFields(t *testing.T) {
	operation := findOperationByID(buildCLIOperations(), "ProjectService.CreateMilestone")
	request := operation.newRequest()
	milestone := request.Interface().(*longhouseclient.Milestone)
	milestone.ProjectId = "project-1"
	milestone.Label = "Opening"
	if missing := missingRequiredFields(request.Elem(), operation); len(missing) != 0 {
		t.Fatalf("milestone missing fields = %#v", missing)
	}
	if milestone.State != "future" {
		t.Fatalf("milestone state = %q", milestone.State)
	}
}

func TestParseCLIArgumentsSupportsSpacedAndEqualsOptions(t *testing.T) {
	parsed, err := parseCLIArguments([]string{"task", "create", "Repair gate", "--due=tomorrow", "--estimate-minutes", "-1", "--all-day", "--json"})
	if err != nil {
		t.Fatalf("parseCLIArguments: %v", err)
	}
	if got := strings.Join(parsed.Words, "|"); got != "task|create|Repair gate" {
		t.Fatalf("words = %q", got)
	}
	if parsed.Options["due"] != "tomorrow" || parsed.Options["estimate-minutes"] != "-1" || parsed.Options["all-day"] != "true" || parsed.Options["output"] != "json" {
		t.Fatalf("options = %#v", parsed.Options)
	}
}

func TestParseCLIArgumentsRejectsUnknownOutputFormat(t *testing.T) {
	if _, err := parseCLIArguments([]string{"task", "list", "--output", "yaml"}); err == nil {
		t.Fatal("unknown output format was accepted")
	}
}

func TestParseCLIArgumentsAcceptsLoginBooleanFlag(t *testing.T) {
	parsed, err := parseCLIArguments([]string{"auth", "login", "--url", "https://longhouse.example", "--no-browser"})
	if err != nil || parsed.Options["no-browser"] != "true" {
		t.Fatalf("login flags = %#v, %v", parsed.Options, err)
	}
}

func TestLocalCLIOptionsRejectUnknownValues(t *testing.T) {
	parsed, err := parseCLIArguments([]string{"task", "done", "task-1", "--surprise=value"})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateLocalCLIOptions(parsed); err == nil {
		t.Fatal("unknown local option was accepted")
	}
}

func TestStrictCLIJSONRejectsUnknownReceiveOnlyAndTrailingData(t *testing.T) {
	operation := findOperationByID(buildCLIOperations(), "TaskService.CreateTask")
	for name, input := range map[string]string{
		"unknown":      `{"title":"Repair gate","surprise":true}`,
		"receive-only": `{"title":"Repair gate","task_id":"task-1"}`,
		"trailing":     `{"title":"Repair gate"} {"title":"Again"}`,
	} {
		t.Run(name, func(t *testing.T) {
			request := operation.newRequest()
			if err := decodeStrictCLIJSON(strings.NewReader(input), request.Interface(), operation); err == nil {
				t.Fatalf("decodeStrictCLIJSON accepted %s input", name)
			}
		})
	}
	request := operation.newRequest()
	if err := decodeStrictCLIJSON(strings.NewReader(`{"title":"Repair gate","status":"open","due_at":"tomorrow"}`), request.Interface(), operation); err != nil {
		t.Fatalf("decodeStrictCLIJSON valid input: %v", err)
	}
	dueAt := request.Interface().(*longhouseclient.Task).DueAt
	if dueAt == nil || *dueAt == "tomorrow" {
		t.Fatalf("due_at was not normalized: %v", dueAt)
	}
	if _, err := time.Parse(time.RFC3339, string(*dueAt)); err != nil {
		t.Fatalf("normalized due_at = %q: %v", *dueAt, err)
	}
}

func TestCLIHelpHidesFieldsThatSelectedUpdateIgnores(t *testing.T) {
	operations := buildCLIOperations()
	task := findOperationByID(operations, "TaskService.UpdateTask")
	if !task.HiddenFields["visibility"] || !task.HiddenFields["parent_task_id"] || !task.HiddenFields["recurrence_root_task_id"] {
		t.Fatalf("task hidden fields = %#v", task.HiddenFields)
	}
	member := findOperationByID(operations, "MemberService.UpdateMember")
	if !member.HiddenFields["linkkeys_domain"] || !member.HiddenFields["cached_public_key"] {
		t.Fatalf("member hidden fields = %#v", member.HiddenFields)
	}
}

func TestCLIOptionsReportConversionErrors(t *testing.T) {
	operation := findOperationByID(buildCLIOperations(), "TaskService.CreateTask")
	request := operation.newRequest()
	err := applyCLIOptions(request, operation, map[string]string{"estimate-minutes": "not-a-number"})
	if err == nil || !strings.Contains(err.Error(), "non-negative integer") {
		t.Fatalf("conversion error = %v", err)
	}
}

func TestCLIInputRejectsRequestOptions(t *testing.T) {
	operation := findOperationByID(buildCLIOperations(), "TaskService.CreateTask")
	runtime := &cliRuntime{input: strings.NewReader(`{"title":"Repair gate"}`)}
	_, err := buildCLIRequest(runtime, operation, nil, map[string]string{"input": "-", "title": "Ignored"})
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("input option conflict = %v", err)
	}
}

func TestTrashRestoreRequiresOneCompleteSelector(t *testing.T) {
	operation := findOperationByID(buildCLIOperations(), "TrashService.Restore")
	request := operation.newRequest()
	if err := validateOperationRequest(operation, request); err == nil {
		t.Fatal("empty trash selector was accepted")
	}
	deletedOperation := "delete-1"
	request.Interface().(*longhouseclient.RestoreRequest).DeletedOpId = &deletedOperation
	if err := validateOperationRequest(operation, request); err != nil {
		t.Fatalf("deleted operation selector: %v", err)
	}
}

func TestGeneratedClientOperationUsesCSILNamesAndCodec(t *testing.T) {
	operation := findOperationByID(buildCLIOperations(), "TaskService.CreateTask")
	request := operation.newRequest()
	task := request.Interface().(*longhouseclient.Task)
	task.Title = "Repair gate"
	want := *task
	want.TaskId = longhouseclient.TaskID("01TESTTASK")
	transport := &recordingGeneratedTransport{response: longhouseclient.EncodeTask(want)}

	result, err := operation.call(context.Background(), transport, request)
	if err != nil {
		t.Fatalf("operation.call: %v", err)
	}
	if transport.service != "TaskService" || transport.operation != "create-task" {
		t.Fatalf("call = %s/%s", transport.service, transport.operation)
	}
	if got := result.(longhouseclient.Task).TaskId; got != want.TaskId {
		t.Fatalf("decoded task id = %q", got)
	}
	decoded, err := longhouseclient.DecodeTask(transport.request)
	if err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if decoded.Title != task.Title {
		t.Fatalf("request title = %q", decoded.Title)
	}
}

func TestRenderCLIJSONEnvelope(t *testing.T) {
	var output bytes.Buffer
	if err := renderCLIResult(&output, map[string]string{"task_id": "task-1"}, map[string]string{"output": "json"}, true); err != nil {
		t.Fatalf("renderCLIResult: %v", err)
	}
	var envelope cliResultEnvelope
	if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !envelope.OK || envelope.SchemaVersion != structuredHelpVersion {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestRenderCLITextUsesCompactMutationSummary(t *testing.T) {
	var output bytes.Buffer
	result := longhouseclient.Task{TaskId: "task-1", Title: "Repair gate"}
	if err := renderCLIResult(&output, result, map[string]string{}, true); err != nil {
		t.Fatalf("render mutation: %v", err)
	}
	if got := output.String(); got != "Task task-1: Repair gate\n" {
		t.Fatalf("mutation output = %q", got)
	}
}

func TestRawAuthResultsDoNotExposeCredentials(t *testing.T) {
	operation := findOperationByID(buildCLIOperations(), "AuthService.ExchangeCliLogin")
	result := longhouseclient.ExchangeCliLoginResponse{
		Status:  "complete",
		Session: &longhouseclient.CliTokenResponse{Token: "access-secret", RefreshToken: "refresh-secret", SessionId: "session-1"},
	}
	encoded, err := json.Marshal(redactCLIResult(operation, result))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "access-secret") || strings.Contains(string(encoded), "refresh-secret") {
		t.Fatalf("credential leaked in %s", encoded)
	}
}

func TestDryRunSecretRedaction(t *testing.T) {
	redacted := redactCLISecrets(longhouseclient.LoginRequest{SignedAssertion: "signed-secret"})
	encoded, err := json.Marshal(redacted)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "signed-secret") || !strings.Contains(string(encoded), "[redacted]") {
		t.Fatalf("dry-run redaction = %s", encoded)
	}
}

func TestFindNamedCLIResourceRejectsAmbiguousNames(t *testing.T) {
	values := []longhouseclient.Task{
		{TaskId: "one", Title: "Same"},
		{TaskId: "two", Title: "Same"},
	}
	if _, err := findNamedCLIResource(values, "task_id", "title", "Same"); err == nil {
		t.Fatal("ambiguous name was accepted")
	} else if !strings.Contains(err.Error(), "one, two") {
		t.Fatalf("ambiguous name did not show matches: %v", err)
	}
	got, err := findNamedCLIResource(values, "task_id", "title", "two")
	if err != nil || got.TaskId != "two" {
		t.Fatalf("stable id lookup = %#v, %v", got, err)
	}
}

func TestTypedCLIReferencesExpandCompoundPositionals(t *testing.T) {
	operations := buildCLIOperations()
	comment := findOperationByID(operations, "CommentService.CreateComment")
	got, err := expandTypedCLIPositionals(comment, []string{"task:Repair gate"})
	if err != nil || !reflect.DeepEqual(got, []string{"task", "Repair gate"}) {
		t.Fatalf("comment reference = %#v, %v", got, err)
	}
	dependency := findOperationByID(operations, "DependencyService.AddDependency")
	got, err = expandTypedCLIPositionals(dependency, []string{"task:Repair gate", "project:Renovation"})
	want := []string{"task", "Repair gate", "project", "Renovation"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("dependency references = %#v, %v", got, err)
	}
}

func TestTypedCLIReferenceRejectsWrongType(t *testing.T) {
	if _, err := normalizeTypedCLIReference("TaskID", "project:Renovation"); err == nil {
		t.Fatal("mismatched typed reference was accepted")
	}
	got, err := normalizeTypedCLIReference("TaskID", "task:01TEST")
	if err != nil || got != "01TEST" {
		t.Fatalf("task reference = %q, %v", got, err)
	}
}

type recordingGeneratedTransport struct {
	service   string
	operation string
	request   []byte
	response  []byte
	err       error
}

func (t *recordingGeneratedTransport) Call(_ context.Context, service, operation string, request []byte) ([]byte, error) {
	t.service = service
	t.operation = operation
	t.request = append([]byte(nil), request...)
	if t.err != nil {
		return nil, t.err
	}
	return append([]byte(nil), t.response...), nil
}

var _ longhouseclient.Transport = (*recordingGeneratedTransport)(nil)

func TestCLIExitErrorUnwraps(t *testing.T) {
	want := errors.New("stop")
	err := &cliExitError{err: want, code: 130}
	if !errors.Is(err, want) || ExitCode(err) != 130 || ErrorWasReported(err) {
		t.Fatalf("exit error = %#v", err)
	}
}

func TestSetCLIFieldFindsNestedSettings(t *testing.T) {
	request := reflect.New(reflect.TypeOf(longhouseclient.UpdateSettingsRequest{}))
	found, err := setCLIFieldChecked(request.Elem(), "bug_reports_enabled", "true")
	if err != nil || !found {
		t.Fatalf("set nested field = %v, %v", found, err)
	}
	got := request.Interface().(*longhouseclient.UpdateSettingsRequest).Settings.BugReportsEnabled
	if got == nil || !*got {
		t.Fatalf("nested setting = %#v", got)
	}
}

func TestSetCLIFieldPreservesExplicitEmptyUpdateStrings(t *testing.T) {
	task := longhouseclient.Task{}
	if found, err := setCLIFieldChecked(reflect.ValueOf(&task).Elem(), "description", ""); err != nil || !found {
		t.Fatalf("set empty description = %v, %v", found, err)
	}
	if task.Description == nil || *task.Description != "" {
		t.Fatalf("empty description = %#v", task.Description)
	}
	if found, err := setCLIFieldChecked(reflect.ValueOf(&task).Elem(), "recurrence_freq", ""); err != nil || !found {
		t.Fatalf("clear recurrence = %v, %v", found, err)
	}
	if task.RecurrenceFreq == nil || *task.RecurrenceFreq != "" {
		t.Fatalf("empty recurrence = %#v", task.RecurrenceFreq)
	}
	memberID := longhouseclient.MemberID("member-1")
	task.AssignedToSkillId = nil
	task.OwnerMemberId = memberID
	if found, err := setCLIFieldChecked(reflect.ValueOf(&task).Elem(), "assigned_to_skill_id", ""); err != nil || !found {
		t.Fatalf("clear skill = %v, %v", found, err)
	}
	if task.AssignedToSkillId != nil {
		t.Fatalf("empty skill id = %#v", task.AssignedToSkillId)
	}
}

func TestProfileStoreMigratesLegacySessionAndKeepsProfiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LONGHOUSE_CONFIG_DIR", dir)
	legacy := cliCredentials{
		URL: "https://one.example", Token: "access", RefreshToken: "refresh",
		SessionID: "session", ExpiresAt: "2030-01-01T00:00:00Z", RefreshExpiresAt: "2030-02-01T00:00:00Z",
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "session.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readCredentials()
	if err != nil {
		t.Fatalf("read legacy credentials: %v", err)
	}
	if got.Profile != defaultCLIProfile || got.URL != legacy.URL {
		t.Fatalf("legacy profile = %#v", got)
	}
	second := &cliCredentials{
		Profile: "second", URL: "https://two.example", Token: "two-access", RefreshToken: "two-refresh",
		SessionID: "two-session", ExpiresAt: "2030-01-01T00:00:00Z", RefreshExpiresAt: "2030-02-01T00:00:00Z",
	}
	if err := saveProfile(second); err != nil {
		t.Fatalf("save second profile: %v", err)
	}
	if _, err := readCredentialsForProfile("second"); err != nil {
		t.Fatalf("read second profile: %v", err)
	}
	if first, err := readCredentialsForProfile(defaultCLIProfile); err != nil || first.URL != legacy.URL {
		t.Fatalf("first profile after second save = %#v, %v", first, err)
	}
}
