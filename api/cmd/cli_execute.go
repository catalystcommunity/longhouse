package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	longhouseclient "github.com/catalystcommunity/longhouse/clients/go"
)

const maxCLIInputBytes = 8 << 20

type parsedCLIArgs struct {
	Words   []string
	Options map[string]string
}

type cliRuntime struct {
	input       io.Reader
	output      io.Writer
	diagnostics io.Writer
	canPrompt   bool
	accessible  bool
	credentials *cliCredentials
	transport   *generatedClientTransport
	houseID     string
	memberID    string
}

type cliResultEnvelope struct {
	SchemaVersion string `json:"schema_version"`
	OK            bool   `json:"ok"`
	Data          any    `json:"data,omitempty"`
	Error         any    `json:"error,omitempty"`
}

var cliBooleanOptions = map[string]bool{
	"accessible": true, "dry-run": true, "help": true, "interactive": true,
	"json": true, "no-input": true, "quiet": true, "version": true, "yes": true,
	"disabled": true, "no-browser": true,
}

var cliGlobalOptions = map[string]bool{
	"accessible": true, "dry-run": true, "help": true, "input": true,
	"interactive": true, "json": true, "no-input": true, "output": true,
	"profile": true, "quiet": true, "url": true, "house": true, "yes": true,
}

func runInteractiveCLI(args []string) error {
	parsed, err := parseCLIArguments(args)
	if err != nil {
		return err
	}
	operations := buildCLIOperations()
	if err := validateRegistry(operations); err != nil {
		return err
	}

	if len(parsed.Words) == 0 {
		return renderCLIHelp(operations, nil, parsed.Options)
	}
	if parsed.Words[0] == "help" {
		return renderCLIHelp(operations, parsed.Words[1:], parsed.Options)
	}
	if parsed.Options["help"] == "true" {
		return renderCLIHelp(operations, parsed.Words, parsed.Options)
	}
	if err := validateLocalCLIOptions(parsed); err != nil {
		return err
	}
	if parsed.Words[0] == "auth" && len(parsed.Words) > 1 && containsString([]string{"login", "status", "refresh", "logout"}, parsed.Words[1]) {
		return runAuthCommand(parsed)
	}
	if parsed.Words[0] == "profile" {
		return runProfileCommand(parsed)
	}
	if len(parsed.Words) >= 2 && parsed.Words[0] == "api" {
		return runRawAPICommand(operations, parsed)
	}
	if parsed.Words[0] == "completion" {
		return runCompletionCommand(operations, parsed)
	}
	if parsed.Words[0] == "agent" {
		return runAgentInstructions(operations, parsed)
	}
	if isLocalResourceCommand(parsed.Words) {
		return runLocalResourceCommand(parsed)
	}
	if isWorkflowCommand(parsed.Words) {
		return runWorkflowCommand(operations, parsed)
	}

	operation, commandWordCount := findOperationByPath(operations, parsed.Words)
	if operation == nil {
		if helpTopicExists(operations, parsed.Words) {
			return renderCLIHelp(operations, parsed.Words, parsed.Options)
		}
		return fmt.Errorf("unknown command %q; run 'longhouse help'", strings.Join(parsed.Words, " "))
	}
	return executeCLIOperation(operation, parsed.Words[commandWordCount:], parsed.Options, reflect.Value{})
}

func helpTopicExists(operations []*cliOperation, topic []string) bool {
	for _, command := range localCLIHelp {
		if pathHasPrefix(command.Path, topic) {
			return true
		}
	}
	for _, operation := range operations {
		if pathHasPrefix(operation.Path, topic) {
			return true
		}
	}
	return false
}

func parseCLIArguments(args []string) (parsedCLIArgs, error) {
	parsed := parsedCLIArgs{Options: map[string]string{}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			parsed.Words = append(parsed.Words, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			parsed.Words = append(parsed.Words, arg)
			continue
		}
		if arg == "-h" {
			parsed.Options["help"] = "true"
			continue
		}
		if !strings.HasPrefix(arg, "--") {
			return parsed, fmt.Errorf("unknown short option %q", arg)
		}
		option := strings.TrimPrefix(arg, "--")
		if option == "" {
			return parsed, errors.New("empty option")
		}
		if before, after, ok := strings.Cut(option, "="); ok {
			if before == "" {
				return parsed, errors.New("empty option name")
			}
			parsed.Options[before] = after
			continue
		}
		if isCLIBooleanOption(option) {
			parsed.Options[option] = "true"
			continue
		}
		if i+1 >= len(args) {
			return parsed, fmt.Errorf("option --%s requires a value", option)
		}
		i++
		parsed.Options[option] = args[i]
	}
	if parsed.Options["json"] == "true" {
		parsed.Options["output"] = "json"
	}
	if output := parsed.Options["output"]; output != "" && output != "text" && output != "json" {
		return parsed, fmt.Errorf("unsupported output format %q; use text or json", output)
	}
	return parsed, nil
}

func isCLIBooleanOption(option string) bool {
	if cliBooleanOptions[option] {
		return true
	}
	fieldName := strings.ReplaceAll(option, "-", "_")
	for _, operation := range buildCLIOperations() {
		if cliTypeHasBooleanJSONField(operation.RequestType, fieldName) {
			return true
		}
	}
	return false
}

func cliTypeHasBooleanJSONField(value reflect.Type, name string) bool {
	value = indirectType(value)
	if value.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldName, _ := jsonFieldName(field)
		if fieldName == name {
			return indirectType(field.Type).Kind() == reflect.Bool
		}
		nested := indirectType(field.Type)
		if nested.Kind() == reflect.Struct && cliTypeHasBooleanJSONField(nested, name) {
			return true
		}
	}
	return false
}

func executeCLIOperation(operation *cliOperation, positionals []string, options map[string]string, requestOverride reflect.Value) error {
	if options["dry-run"] == "true" && !operation.Mutation {
		return fmt.Errorf("--dry-run is only available for mutation commands")
	}
	runtime, err := newCLIRuntimeForOperation(options, operation)
	if err != nil {
		return err
	}
	request := requestOverride
	if !request.IsValid() {
		request, err = buildCLIRequest(runtime, operation, positionals, options)
		if err != nil {
			return err
		}
	}
	if err := operation.validate(request); err != nil {
		return fmt.Errorf("validate %s request: %w", strings.Join(operation.Path, " "), err)
	}
	if err := validateOperationRequest(operation, request); err != nil {
		return err
	}
	if options["dry-run"] == "true" {
		return renderCLIResult(runtime.output, redactCLISecrets(request.Elem().Interface()), options, false)
	}
	if operation.Destructive {
		if err := confirmCLIMutation(operation, options, runtime); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := operation.call(ctx, runtime.transport, request)
	if err != nil {
		return err
	}
	return renderCLIResult(runtime.output, redactCLIResult(operation, result), options, operation.Mutation)
}

func redactCLISecrets(value any) any {
	encoded, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return value
	}
	redactCLISecretTree(decoded)
	return decoded
}

func redactCLISecretTree(value any) {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "assertion") {
				current[key] = "[redacted]"
				continue
			}
			redactCLISecretTree(child)
		}
	case []any:
		for _, child := range current {
			redactCLISecretTree(child)
		}
	}
}

func newCLIRuntime(options map[string]string) (*cliRuntime, error) {
	credentials, err := readCredentialsForProfile(options["profile"])
	if err != nil {
		return nil, err
	}
	baseURL, err := cliURL(options, credentials)
	if err != nil {
		return nil, err
	}
	credentials.URL = baseURL
	if accessTokenNeedsRefresh(credentials) {
		if err := refreshCredentials(context.Background(), credentials); err != nil {
			return nil, err
		}
	}
	runtime := &cliRuntime{
		input: os.Stdin, output: os.Stdout, diagnostics: os.Stderr,
		canPrompt:   cliInputIsTerminal() && options["no-input"] != "true" && options["input"] == "",
		accessible:  options["accessible"] == "true" || environmentEnabled("LONGHOUSE_ACCESSIBLE"),
		credentials: credentials,
		transport:   &generatedClientTransport{baseURL: baseURL, bearer: credentials.Token},
		houseID:     strings.TrimSpace(options["house"]),
	}
	if runtime.houseID == "" {
		runtime.houseID = strings.TrimSpace(os.Getenv("LONGHOUSE_HOUSE"))
	}
	if runtime.houseID == "" {
		runtime.houseID = credentials.HouseID
	}
	return runtime, nil
}

func newCLIRuntimeForOperation(options map[string]string, operation *cliOperation) (*cliRuntime, error) {
	if !operationAllowsAnonymous(operation) {
		return newCLIRuntime(options)
	}
	var profile *cliCredentials
	profile, _ = readProfile(options["profile"])
	baseURL, err := cliURL(options, profile)
	if err != nil {
		return nil, err
	}
	return &cliRuntime{
		input: os.Stdin, output: os.Stdout, diagnostics: os.Stderr,
		credentials: profile,
		transport:   &generatedClientTransport{baseURL: baseURL},
	}, nil
}

func operationAllowsAnonymous(operation *cliOperation) bool {
	if operation.Service == "DevAuthService" {
		return true
	}
	if operation.Service != "AuthService" {
		return false
	}
	return containsString([]string{"Login", "Complete", "BeginCliLogin", "ExchangeCliLogin", "RefreshSession"}, operation.Method)
}

func redactCLIResult(operation *cliOperation, result any) any {
	if operation.Service != "AuthService" && operation.Service != "DevAuthService" {
		return result
	}
	switch value := result.(type) {
	case longhouseclient.LoginResponse:
		return map[string]any{
			"domain": value.Domain, "user_id": value.UserId,
			"display_name": value.DisplayName, "expires_at": value.ExpiresAt,
		}
	case longhouseclient.CliTokenResponse:
		return map[string]any{
			"domain": value.Domain, "user_id": value.UserId, "display_name": value.DisplayName,
			"expires_at": value.ExpiresAt, "refresh_expires_at": value.RefreshExpiresAt,
			"session_id": value.SessionId,
		}
	case longhouseclient.BeginCliLoginResponse:
		return map[string]any{
			"user_code": value.UserCode, "verification_url": value.VerificationUrl,
			"expires_at": value.ExpiresAt, "interval_seconds": value.IntervalSeconds,
		}
	case longhouseclient.ExchangeCliLoginResponse:
		redacted := map[string]any{"status": value.Status}
		if value.Session != nil {
			redacted["session"] = map[string]any{
				"domain": value.Session.Domain, "user_id": value.Session.UserId,
				"display_name": value.Session.DisplayName, "expires_at": value.Session.ExpiresAt,
				"refresh_expires_at": value.Session.RefreshExpiresAt, "session_id": value.Session.SessionId,
			}
		}
		return redacted
	default:
		return result
	}
}

func (runtime *cliRuntime) resolveHouseContext() error {
	if runtime.houseID != "" && runtime.memberID != "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	me, err := longhouseclient.NewAuthClient(runtime.transport).Me(ctx, longhouseclient.EmptyRequest{})
	if err != nil {
		return fmt.Errorf("load account context: %w", err)
	}
	if runtime.houseID != "" {
		for _, house := range me.Houses {
			if string(house.HouseId) == runtime.houseID || strings.EqualFold(house.Name, runtime.houseID) {
				runtime.houseID = string(house.HouseId)
				runtime.memberID = string(house.MemberId)
				return nil
			}
		}
		return fmt.Errorf("house %q is not available to this session", runtime.houseID)
	}
	if len(me.Houses) == 1 {
		runtime.houseID = string(me.Houses[0].HouseId)
		runtime.memberID = string(me.Houses[0].MemberId)
		return nil
	}
	if len(me.Houses) == 0 {
		return errors.New("this session has no accessible houses")
	}
	if runtime.canPrompt {
		houseID, memberID, selectErr := selectCLIHouse(runtime, me.Houses)
		if selectErr != nil {
			return selectErr
		}
		runtime.houseID = houseID
		runtime.memberID = memberID
		return nil
	}
	names := make([]string, 0, len(me.Houses))
	for _, house := range me.Houses {
		names = append(names, fmt.Sprintf("%s (%s)", house.Name, house.HouseId))
	}
	return fmt.Errorf("select a house with --house; available houses: %s", strings.Join(names, ", "))
}

func buildCLIRequest(runtime *cliRuntime, operation *cliOperation, positionals []string, options map[string]string) (reflect.Value, error) {
	if operation.ID == "EventService.SetCalendarView" && strings.TrimSpace(options["input"]) == "" {
		return reflect.Value{}, errors.New("calendar edit requires --input; use calendar subscription commands for interactive changes")
	}
	var err error
	positionals, err = expandTypedCLIPositionals(operation, positionals)
	if err != nil {
		return reflect.Value{}, err
	}
	request := operation.newRequest()
	if inputPath := strings.TrimSpace(options["input"]); inputPath != "" {
		if hasCLIFieldOptions(options) {
			return reflect.Value{}, errors.New("request options cannot be combined with --input")
		}
		reader, closeInput, err := openCLIInput(inputPath, runtime.input)
		if err != nil {
			return reflect.Value{}, err
		}
		defer closeInput()
		if err := decodeStrictCLIJSON(reader, request.Interface(), operation); err != nil {
			return reflect.Value{}, err
		}
		if len(positionals) > 0 {
			return reflect.Value{}, errors.New("positional arguments cannot be combined with --input")
		}
	} else {
		if strings.HasPrefix(operation.Method, "Update") && (len(positionals) > 0 || operation.ID == "SettingsService.UpdateSettings") {
			if len(positionals) > 0 && len(operation.Positionals) > 0 {
				typeName := referenceTypeForField(operation.Positionals[0])
				if typeName != "" {
					resolved, resolveErr := (&cliReferenceResolver{runtime: runtime, typeOverrides: map[string]string{}}).resolveReference(typeName, positionals[0])
					if resolveErr != nil {
						return reflect.Value{}, resolveErr
					}
					positionals[0] = resolved
				}
			}
			current, err := loadCurrentCLIRequest(runtime, operation, positionals, options)
			if err != nil {
				return reflect.Value{}, err
			}
			request = current
		}
		if err := applyCLIPositionals(request, operation, positionals); err != nil {
			return reflect.Value{}, err
		}
		if err := applyCLIOptions(request, operation, options); err != nil {
			return reflect.Value{}, err
		}
	}

	if operation.RequestType.Name() == "HouseID" && fieldIsEmpty(request.Elem()) {
		if err := runtime.resolveHouseContext(); err != nil {
			return reflect.Value{}, err
		}
		if err := setCLIValue(request.Elem(), runtime.houseID); err != nil {
			return reflect.Value{}, err
		}
	}
	if (!operation.HiddenFields["house_id"] && requestNeedsHouse(request.Elem())) || requestNeedsMemberOwner(request.Elem()) {
		if err := runtime.resolveHouseContext(); err != nil {
			return reflect.Value{}, err
		}
		setStringFieldIfEmpty(request.Elem(), "house_id", runtime.houseID)
		setStringFieldIfEmpty(request.Elem(), "owner_member_id", runtime.memberID)
	}
	if err := resolveCLIReferences(runtime, operation, request); err != nil {
		return reflect.Value{}, err
	}

	missing := missingRequiredFields(request.Elem(), operation)
	formEligible := operation.Mutation && operation.RequestType.Kind() == reflect.Struct
	forceInteractive := options["interactive"] == "true" && options["input"] == ""
	if forceInteractive && !formEligible {
		return reflect.Value{}, fmt.Errorf("--interactive is not available for %s", strings.Join(operation.Path, " "))
	}
	canPrompt := formEligible && options["input"] == "" && cliInputIsTerminal() && options["no-input"] != "true"
	editNeedsInput := strings.HasPrefix(operation.Method, "Update") && options["input"] == "" && !hasCLIFieldOptions(options)
	if forceInteractive && !canPrompt {
		return reflect.Value{}, errors.New("--interactive requires a terminal on standard input")
	}
	if editNeedsInput && !canPrompt {
		return reflect.Value{}, errors.New("an edit command requires field options, --input, or an interactive terminal")
	}
	formRan := false
	if forceInteractive || (editNeedsInput && canPrompt) || (len(missing) > 0 && canPrompt) {
		if err := runCLIForm(request, operation, options, runtime); err != nil {
			return reflect.Value{}, err
		}
		formRan = true
		missing = missingRequiredFields(request.Elem(), operation)
	}
	if formRan {
		if err := resolveCLIReferences(runtime, operation, request); err != nil {
			return reflect.Value{}, err
		}
	}
	if len(missing) > 0 {
		return reflect.Value{}, fmt.Errorf("missing required input: %s", strings.Join(missing, ", "))
	}
	return request, nil
}

func hasCLIFieldOptions(options map[string]string) bool {
	for option := range options {
		if !cliGlobalOptions[option] {
			return true
		}
	}
	return false
}

func loadCurrentCLIRequest(runtime *cliRuntime, operation *cliOperation, positionals []string, options map[string]string) (reflect.Value, error) {
	id := ""
	if len(positionals) > 0 {
		id = positionals[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var current any
	var err error
	switch operation.ID {
	case "HouseService.UpdateHouse":
		current, err = longhouseclient.NewHouseClient(runtime.transport).GetHouse(ctx, longhouseclient.HouseID(id))
	case "MemberService.UpdateMember":
		current, err = longhouseclient.NewMemberClient(runtime.transport).GetMember(ctx, longhouseclient.MemberID(id))
	case "ProjectService.UpdateProject":
		current, err = longhouseclient.NewProjectClient(runtime.transport).GetProject(ctx, longhouseclient.ProjectID(id))
	case "EventService.UpdateEvent":
		current, err = longhouseclient.NewEventClient(runtime.transport).GetEvent(ctx, longhouseclient.EventID(id))
	case "TaskService.UpdateTask":
		current, err = longhouseclient.NewTaskClient(runtime.transport).GetTask(ctx, longhouseclient.TaskID(id))
	case "CommentService.UpdateComment":
		current, err = longhouseclient.NewCommentClient(runtime.transport).GetComment(ctx, longhouseclient.CommentID(id))
	case "RoleService.UpdateRole":
		if err = runtime.resolveHouseContext(); err == nil {
			var values []longhouseclient.Role
			values, err = longhouseclient.NewRoleClient(runtime.transport).ListRoles(ctx, longhouseclient.HouseScopedListRequest{HouseId: longhouseclient.HouseID(runtime.houseID)})
			if err == nil {
				current, err = findCLIResource(values, "role_id", id)
			}
		}
	case "SkillService.UpdateSkill":
		if err = runtime.resolveHouseContext(); err == nil {
			var values []longhouseclient.Skill
			values, err = longhouseclient.NewSkillClient(runtime.transport).ListSkills(ctx, longhouseclient.HouseScopedListRequest{HouseId: longhouseclient.HouseID(runtime.houseID)})
			if err == nil {
				current, err = findCLIResource(values, "skill_id", id)
			}
		}
	case "GroupService.UpdateGroup":
		if err = runtime.resolveHouseContext(); err == nil {
			var values []longhouseclient.Group
			values, err = longhouseclient.NewGroupClient(runtime.transport).ListGroups(ctx, longhouseclient.HouseScopedListRequest{HouseId: longhouseclient.HouseID(runtime.houseID)})
			if err == nil {
				current, err = findCLIResource(values, "group_id", id)
			}
		}
	case "ProjectService.UpdateMilestone":
		projectID := options["project"]
		client := longhouseclient.NewProjectClient(runtime.transport)
		var values []longhouseclient.Milestone
		if projectID != "" {
			projectID, err = (&cliReferenceResolver{runtime: runtime, typeOverrides: map[string]string{}}).resolveReference("ProjectID", projectID)
			if err == nil {
				values, err = client.ListMilestones(ctx, longhouseclient.ProjectID(projectID))
			}
		} else if err = runtime.resolveHouseContext(); err == nil {
			limit := uint64(1000)
			var projects longhouseclient.ProjectList
			projects, err = client.ListProjects(ctx, longhouseclient.HouseScopedListRequest{HouseId: longhouseclient.HouseID(runtime.houseID), Limit: &limit})
			if err == nil {
				for _, project := range projects.Projects {
					milestones, listErr := client.ListMilestones(ctx, project.ProjectId)
					if listErr != nil {
						err = listErr
						break
					}
					values = append(values, milestones...)
				}
			}
		}
		if err == nil {
			current, err = findNamedCLIResource(values, "milestone_id", "label", id)
		}
	case "SettingsService.UpdateSettings":
		if err = runtime.resolveHouseContext(); err == nil {
			var settings longhouseclient.EffectiveSettings
			settings, err = longhouseclient.NewSettingsClient(runtime.transport).GetSettings(ctx, longhouseclient.HouseID(runtime.houseID))
			if err == nil {
				current = longhouseclient.UpdateSettingsRequest{HouseId: longhouseclient.HouseID(runtime.houseID), Settings: settings}
			}
		}
	default:
		return operation.newRequest(), nil
	}
	if err != nil {
		return reflect.Value{}, fmt.Errorf("load current value for %s: %w", strings.Join(operation.Path, " "), err)
	}
	request := reflect.New(operation.RequestType)
	value := reflect.ValueOf(current)
	if value.Type() != operation.RequestType {
		return reflect.Value{}, fmt.Errorf("loaded %s, expected %s", value.Type(), operation.RequestType)
	}
	request.Elem().Set(value)
	return request, nil
}

func findCLIResource[T any](values []T, idField, id string) (T, error) {
	for _, value := range values {
		field := findCLIField(reflect.ValueOf(value), idField)
		if field.IsValid() && formatCLIValue(field) == id {
			return value, nil
		}
	}
	var zero T
	return zero, fmt.Errorf("resource %q was not found", id)
}

func openCLIInput(path string, stdin io.Reader) (io.Reader, func(), error) {
	if path == "-" {
		return io.LimitReader(stdin, maxCLIInputBytes+1), func() {}, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open input file: %w", err)
	}
	return io.LimitReader(file, maxCLIInputBytes+1), func() { _ = file.Close() }, nil
}

func decodeStrictCLIJSON(reader io.Reader, target any, operation *cliOperation) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read JSON input: %w", err)
	}
	if len(data) > maxCLIInputBytes {
		return errors.New("JSON input is too large")
	}
	if operation.RequestType.Kind() == reflect.Struct {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			return fmt.Errorf("decode JSON input: %w", err)
		}
		for name := range fields {
			if operation.HiddenFields[name] {
				return fmt.Errorf("field %q is not accepted by %s", name, strings.Join(operation.Path, " "))
			}
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode JSON input: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("JSON input contains more than one value")
		}
		return fmt.Errorf("decode JSON input: %w", err)
	}
	normalizeCLIInputValues(reflect.ValueOf(target))
	return nil
}

func normalizeCLIInputValues(value reflect.Value) {
	if !value.IsValid() {
		return
	}
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return
		}
		normalizeCLIInputValues(value.Elem())
		return
	}
	if value.Kind() == reflect.String && value.Type().Name() == "Timestamp" && value.CanSet() {
		value.SetString(normalizeCLIString(value.Type().Name(), value.String()))
		return
	}
	switch value.Kind() {
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			normalizeCLIInputValues(value.Field(i))
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			normalizeCLIInputValues(value.Index(i))
		}
	}
}

func applyCLIPositionals(request reflect.Value, operation *cliOperation, positionals []string) error {
	if len(positionals) > len(operation.Positionals) {
		return fmt.Errorf("too many arguments for %s", strings.Join(operation.Path, " "))
	}
	if operation.RequestType.Kind() != reflect.Struct {
		if len(positionals) == 0 {
			return nil
		}
		return setCLIValue(request.Elem(), positionals[0])
	}
	for i, value := range positionals {
		fieldName := operation.Positionals[i]
		if fieldName == "value" {
			return fmt.Errorf("internal CLI registry error: value positional used for a structured request")
		}
		found, err := setCLIFieldChecked(request.Elem(), fieldName, value)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", fieldName, err)
		}
		if !found {
			return fmt.Errorf("command field %q is not present in %s", fieldName, operation.RequestType.Name())
		}
	}
	return nil
}

func applyCLIOptions(request reflect.Value, operation *cliOperation, options map[string]string) error {
	for option, value := range options {
		if cliGlobalOptions[option] {
			continue
		}
		if operation.RequestType.Kind() != reflect.Struct {
			return fmt.Errorf("unknown option --%s for %s", option, strings.Join(operation.Path, " "))
		}
		fieldName := strings.ReplaceAll(option, "-", "_")
		fieldName = cliFieldAliases[fieldName]
		if fieldName == "" {
			fieldName = strings.ReplaceAll(option, "-", "_")
		}
		if operation.ID == "GroupService.ListGroupMembers" && fieldName == "group_id" {
			fieldName = "member_id"
		}
		if operation.HiddenFields[fieldName] {
			return fmt.Errorf("option --%s is not accepted by %s", option, strings.Join(operation.Path, " "))
		}
		found, err := setCLIFieldChecked(request.Elem(), fieldName, value)
		if err != nil {
			return fmt.Errorf("invalid --%s: %w", option, err)
		}
		if !found {
			return fmt.Errorf("unknown option --%s for %s", option, strings.Join(operation.Path, " "))
		}
	}
	return nil
}

var cliFieldAliases = map[string]string{
	"house": "house_id", "member": "member_id", "owner": "owner_member_id",
	"project": "project_id", "task": "task_id", "event": "event_id",
	"comment": "comment_id", "role": "role_id", "skill": "skill_id",
	"group": "group_id", "milestone": "milestone_id", "due": "due_at",
	"starts": "starts_at", "ends": "ends_at", "limit": "limit", "offset": "offset",
}

func setCLIField(value reflect.Value, name, input string) bool {
	found, _ := setCLIFieldChecked(value, name, input)
	return found
}

func setCLIFieldChecked(value reflect.Value, name, input string) (bool, error) {
	value = indirectValue(value)
	if value.Kind() != reflect.Struct {
		return false, nil
	}
	for i := 0; i < value.NumField(); i++ {
		fieldInfo := value.Type().Field(i)
		fieldName, _ := jsonFieldName(fieldInfo)
		field := value.Field(i)
		if fieldName == name {
			return true, setCLIFieldValue(field, fieldName, input)
		}
		kind := field.Type().Kind()
		if kind == reflect.Struct || (kind == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct) {
			if found, err := setCLIFieldChecked(field, name, input); found || err != nil {
				return found, err
			}
		}
	}
	return false, nil
}

// Some update operations distinguish an omitted string pointer from an
// explicit empty string. Keep that distinction for fields that the API uses
// this way so that an edit can clear their current value.
func setCLIFieldValue(value reflect.Value, name, input string) error {
	if input == "" && value.Kind() == reflect.Ptr && value.Type().Elem().Kind() == reflect.String && cliExplicitEmptyStringFields[name] {
		value.Set(reflect.New(value.Type().Elem()))
		value.Elem().SetString("")
		return nil
	}
	return setCLIValue(value, input)
}

var cliExplicitEmptyStringFields = map[string]bool{
	"avatar_url":      true,
	"description":     true,
	"display_name":    true,
	"location":        true,
	"recurrence_freq": true,
}

func setCLIValue(value reflect.Value, input string) error {
	if value.Kind() == reflect.Ptr {
		if strings.TrimSpace(input) == "" {
			value.SetZero()
			return nil
		}
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return setCLIValue(value.Elem(), input)
	}
	if value.Kind() == reflect.Slice {
		if value.Type().Elem().Kind() == reflect.Uint8 {
			decoded, err := base64.StdEncoding.DecodeString(input)
			if err != nil {
				return fmt.Errorf("expected base64 data: %w", err)
			}
			value.SetBytes(decoded)
			return nil
		}
		if strings.TrimSpace(input) == "" {
			value.Set(reflect.MakeSlice(value.Type(), 0, 0))
			return nil
		}
		parts := strings.Split(input, ",")
		result := reflect.MakeSlice(value.Type(), 0, len(parts))
		for _, part := range parts {
			element := reflect.New(value.Type().Elem()).Elem()
			if err := setCLIValue(element, strings.TrimSpace(part)); err != nil {
				return err
			}
			result = reflect.Append(result, element)
		}
		value.Set(result)
		return nil
	}
	switch value.Kind() {
	case reflect.String:
		value.SetString(normalizeCLIString(value.Type().Name(), input))
	case reflect.Bool:
		parsed, err := strconv.ParseBool(input)
		if err != nil {
			return fmt.Errorf("expected a boolean, got %q", input)
		}
		value.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(input, 10, value.Type().Bits())
		if err != nil {
			return fmt.Errorf("expected an integer, got %q", input)
		}
		value.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(input, 10, value.Type().Bits())
		if err != nil {
			return fmt.Errorf("expected a non-negative integer, got %q", input)
		}
		value.SetUint(parsed)
	default:
		return fmt.Errorf("unsupported command input type %s", value.Type())
	}
	return nil
}

func normalizeCLIString(typeName, value string) string {
	if typeName != "Timestamp" {
		return value
	}
	trimmed := strings.TrimSpace(value)
	now := time.Now()
	switch strings.ToLower(trimmed) {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).UTC().Format(time.RFC3339)
	case "tomorrow":
		return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.Local).UTC().Format(time.RFC3339)
	}
	if parsed, err := time.ParseInLocation("2006-01-02", trimmed, time.Local); err == nil {
		return parsed.UTC().Format(time.RFC3339)
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed.UTC().Format(time.RFC3339)
	}
	return value
}

func validateOperationRequest(operation *cliOperation, request reflect.Value) error {
	if operation.RequestType.Kind() != reflect.Struct {
		if request.Elem().Kind() == reflect.String && request.Elem().Len() == 0 {
			return errors.New("a resource identifier is required")
		}
		return nil
	}
	for name, allowed := range cliEnumValues {
		field := findCLIField(request.Elem(), name)
		if !field.IsValid() || fieldIsEmpty(field) {
			continue
		}
		field = indirectValue(field)
		if field.Kind() != reflect.String {
			continue
		}
		value := field.String()
		if name == "status" {
			if operation.RequestType.Name() == "Project" {
				allowed = []string{"active", "archived"}
			} else {
				allowed = []string{"open", "in_progress", "done", "cancelled"}
			}
		}
		if !containsString(allowed, value) {
			return fmt.Errorf("field %s must be one of: %s", name, strings.Join(allowed, ", "))
		}
	}
	for _, timestampName := range []string{"due_at", "starts_at", "ends_at", "expires_at", "since", "until"} {
		field := findCLIField(request.Elem(), timestampName)
		if !field.IsValid() || fieldIsEmpty(field) {
			continue
		}
		field = indirectValue(field)
		if _, err := time.Parse(time.RFC3339, field.String()); err != nil {
			return fmt.Errorf("field %s must be an RFC3339 timestamp, YYYY-MM-DD, today, or tomorrow", timestampName)
		}
	}
	if operation.ID == "TrashService.Restore" {
		deletedOperation := findCLIField(request.Elem(), "deleted_op_id")
		resourceType := findCLIField(request.Elem(), "resource_type")
		resourceID := findCLIField(request.Elem(), "resource_id")
		if fieldIsEmpty(deletedOperation) && (fieldIsEmpty(resourceType) || fieldIsEmpty(resourceID)) {
			return errors.New("trash restore requires --deleted-op-id or both --resource-type and --resource-id")
		}
	}
	return nil
}

func requestNeedsHouse(value reflect.Value) bool {
	return findCLIField(value, "house_id").IsValid() && fieldIsEmpty(findCLIField(value, "house_id"))
}

func requestNeedsMemberOwner(value reflect.Value) bool {
	return findCLIField(value, "owner_member_id").IsValid() && fieldIsEmpty(findCLIField(value, "owner_member_id"))
}

func setStringFieldIfEmpty(value reflect.Value, name, input string) {
	field := findCLIField(value, name)
	if field.IsValid() && fieldIsEmpty(field) {
		_ = setCLIValue(field, input)
	}
}

func findCLIField(value reflect.Value, name string) reflect.Value {
	value = indirectValue(value)
	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	for i := 0; i < value.NumField(); i++ {
		fieldInfo := value.Type().Field(i)
		fieldName, _ := jsonFieldName(fieldInfo)
		field := value.Field(i)
		if fieldName == name {
			return field
		}
		kind := field.Type().Kind()
		if kind == reflect.Struct || (kind == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct && !field.IsNil()) {
			if nested := findCLIField(field, name); nested.IsValid() {
				return nested
			}
		}
	}
	return reflect.Value{}
}

func indirectValue(value reflect.Value) reflect.Value {
	for value.IsValid() && value.Kind() == reflect.Ptr {
		if value.IsNil() {
			if value.CanSet() {
				value.Set(reflect.New(value.Type().Elem()))
			} else {
				return reflect.Value{}
			}
		}
		value = value.Elem()
	}
	return value
}

func fieldIsEmpty(value reflect.Value) bool {
	if !value.IsValid() {
		return true
	}
	if value.Kind() == reflect.Ptr {
		return value.IsNil() || fieldIsEmpty(value.Elem())
	}
	return value.IsZero()
}

func missingRequiredFields(value reflect.Value, operation *cliOperation) []string {
	value = indirectValue(value)
	if value.Kind() != reflect.Struct {
		if fieldIsEmpty(value) {
			return []string{"value"}
		}
		return nil
	}
	var missing []string
	for i := 0; i < value.NumField(); i++ {
		fieldInfo := value.Type().Field(i)
		name, optional := jsonFieldName(fieldInfo)
		optional = cliOperationFieldOptional(operation, name, optional)
		if name == "" || operation.HiddenFields[name] || optional || fieldInfo.Type.Kind() == reflect.Ptr || fieldInfo.Type.Kind() == reflect.Slice {
			continue
		}
		field := value.Field(i)
		if fieldInfo.Type.Kind() == reflect.Struct {
			missing = append(missing, missingRequiredFields(field, operation)...)
			continue
		}
		if fieldIsEmpty(field) && (field.Kind() == reflect.String || field.Kind() == reflect.Interface) {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func renderCLIResult(writer io.Writer, result any, options map[string]string, mutation bool) error {
	if options["output"] == "json" {
		return writeCLIJSON(writer, cliResultEnvelope{SchemaVersion: structuredHelpVersion, OK: true, Data: result})
	}
	if options["quiet"] == "true" {
		if id := primaryCLIIdentifier(reflect.ValueOf(result)); id != "" {
			_, err := fmt.Fprintln(writer, id)
			return err
		}
	}
	return renderCLIText(writer, reflect.ValueOf(result), mutation)
}

func writeCLIJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func renderCLIText(writer io.Writer, value reflect.Value, mutation bool) error {
	value = indirectReadValue(value)
	if !value.IsValid() || (value.Kind() == reflect.Struct && value.NumField() == 0) {
		_, err := fmt.Fprintln(writer, "Done.")
		return err
	}
	if mutation && value.Kind() == reflect.Struct {
		if id := primaryCLIIdentifier(value); id != "" {
			summary := ""
			for _, name := range []string{"name", "title", "label", "status"} {
				field := findCLIField(value, name)
				if field.IsValid() && !fieldIsEmpty(field) {
					summary = formatCLIValue(field)
					break
				}
			}
			if summary != "" {
				_, err := fmt.Fprintf(writer, "%s %s: %s\n", value.Type().Name(), id, summary)
				return err
			}
			_, err := fmt.Fprintf(writer, "%s %s\n", value.Type().Name(), id)
			return err
		}
	}
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		return renderCLITable(writer, value)
	case reflect.Struct:
		for _, collectionName := range []string{"tasks", "projects", "entries", "items", "sessions", "users"} {
			collection := findCLIField(value, collectionName)
			if collection.IsValid() && collection.Kind() == reflect.Slice {
				if err := renderCLITable(writer, collection); err != nil {
					return err
				}
				if hidden := findCLIField(value, "hidden_count"); hidden.IsValid() && !fieldIsEmpty(hidden) {
					_, err := fmt.Fprintf(writer, "Hidden results: %s\n", formatCLIValue(hidden))
					return err
				}
				if cursor := findCLIField(value, "next_cursor"); cursor.IsValid() && !fieldIsEmpty(cursor) {
					_, err := fmt.Fprintf(writer, "Next cursor: %s\n", formatCLIValue(cursor))
					return err
				}
				return nil
			}
		}
		for i := 0; i < value.NumField(); i++ {
			fieldInfo := value.Type().Field(i)
			name, _ := jsonFieldName(fieldInfo)
			field := indirectReadValue(value.Field(i))
			if !field.IsValid() || field.IsZero() {
				continue
			}
			encoded := formatCLIValue(field)
			if _, err := fmt.Fprintf(writer, "%s: %s\n", name, encoded); err != nil {
				return err
			}
		}
		return nil
	default:
		_, err := fmt.Fprintln(writer, formatCLIValue(value))
		return err
	}
}

func renderCLITable(writer io.Writer, value reflect.Value) error {
	if value.Len() == 0 {
		_, err := fmt.Fprintln(writer, "No results.")
		return err
	}
	first := indirectReadValue(value.Index(0))
	if first.Kind() != reflect.Struct {
		for i := 0; i < value.Len(); i++ {
			if _, err := fmt.Fprintln(writer, formatCLIValue(indirectReadValue(value.Index(i)))); err != nil {
				return err
			}
		}
		return nil
	}
	columns := tableColumns(first.Type())
	headings := make([]string, len(columns))
	for i, index := range columns {
		headings[i], _ = jsonFieldName(first.Type().Field(index))
	}
	if _, err := fmt.Fprintln(writer, strings.Join(headings, "\t")); err != nil {
		return err
	}
	for i := 0; i < value.Len(); i++ {
		row := indirectReadValue(value.Index(i))
		cells := make([]string, len(columns))
		for j, index := range columns {
			cells[j] = formatCLIValue(indirectReadValue(row.Field(index)))
		}
		if _, err := fmt.Fprintln(writer, strings.Join(cells, "\t")); err != nil {
			return err
		}
	}
	return nil
}

func tableColumns(value reflect.Type) []int {
	var preferred, remainder []int
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		name, _ := jsonFieldName(field)
		kind := indirectType(field.Type).Kind()
		if kind == reflect.Struct || kind == reflect.Slice || kind == reflect.Map {
			continue
		}
		if strings.HasSuffix(name, "_id") || name == "name" || name == "title" || name == "label" || name == "status" || name == "domain" {
			preferred = append(preferred, i)
		} else {
			remainder = append(remainder, i)
		}
	}
	columns := append(preferred, remainder...)
	if len(columns) > 6 {
		columns = columns[:6]
	}
	return columns
}

func formatCLIValue(value reflect.Value) string {
	value = indirectReadValue(value)
	if !value.IsValid() {
		return ""
	}
	if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		parts := make([]string, value.Len())
		for i := 0; i < value.Len(); i++ {
			parts[i] = formatCLIValue(value.Index(i))
		}
		return strings.Join(parts, ",")
	}
	if value.Kind() == reflect.Struct || value.Kind() == reflect.Map {
		encoded, _ := json.Marshal(value.Interface())
		return string(encoded)
	}
	return fmt.Sprint(value.Interface())
}

func primaryCLIIdentifier(value reflect.Value) string {
	value = indirectReadValue(value)
	if !value.IsValid() {
		return ""
	}
	if value.Kind() == reflect.Struct {
		for i := 0; i < value.NumField(); i++ {
			name, _ := jsonFieldName(value.Type().Field(i))
			if strings.HasSuffix(name, "_id") && !fieldIsEmpty(value.Field(i)) {
				return formatCLIValue(value.Field(i))
			}
		}
	}
	return ""
}

func indirectReadValue(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Ptr || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

func indirectType(value reflect.Type) reflect.Type {
	for value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	return value
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
