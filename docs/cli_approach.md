# Longhouse CLI approach

Status: Implemented

## Purpose

The Longhouse command-line interface (CLI) must support people, scripts, and
language models. It must use the generated Longhouse client and the CSIL-RPC
carrier. It must not add a REST or JSON wire protocol.

The CLI has three input modes:

1. Command arguments and options for short operations.
2. A terminal form for interactive create and edit operations.
3. JSON input for files, standard input, and automation.

The CLI uses plain text by default. JSON is an explicit automation format. It
is not the default human interface.

## Design goals

- Make common task and project operations short.
- Make all commands easy to find from the CLI.
- Give language models a small and stable discovery interface.
- Give users validation errors before the CLI sends a request.
- Keep standard output safe for pipes.
- Use the generated CSIL types, constructors, validators, and codecs.
- Keep the server as the authority for authorization and stored data.
- Make destructive commands clear and difficult to run by accident.

The first version does not provide a persistent full-screen application. Each
command starts, shows a form when necessary, sends a request, prints a result,
and exits.

## Interaction modes

### Direct mode

Direct mode uses arguments and options. It is suitable for short commands and
shell scripts.

```text
longhouse task done task_01J...
longhouse task create --title "Repair the west gate" --due 2026-09-22
longhouse project member add project_01J... member_01J...
```

Direct mode does not require JSON.

### Interactive mode

The CLI can show an inline form when standard input is a terminal. A create or
edit command shows a form when required input is missing. The user can also use
`--interactive` to request the form. Existing arguments and options become the
initial field values.

```text
longhouse task create
longhouse task edit task_01J...
longhouse task create --title "Repair the west gate" --interactive
```

The form writes prompts and status text to the terminal output stream. The
command result stays on standard output. This rule keeps command output safe
for a pipe.

The CLI must not prompt when standard input is not a terminal. In that case,
the command must return a clear error when required input is missing.
`--no-input` also disables forms and confirmations. A destructive operation
then requires `--yes`.

The form must provide these field types:

- Single-line text
- Multi-line text
- Date and time
- Boolean confirmation
- Single selection
- Multiple selection
- Resource selection with search

The form must show field errors next to the applicable field. The final submit
action must run all request validation again.

### JSON file and automation mode

JSON is for language models, files, and programs. A command accepts an
operation-specific request from a file or standard input.

```text
longhouse task create --input task.json
longhouse task create --input - --output json
```

`--json` is a short alias for `--output json`. The CLI must reject unknown JSON
fields and additional JSON values after the first value. It must also reject
fields that are not valid request fields for the selected operation. These
rules prevent silent input errors.

JSON is converted to a generated request type in the CLI. The generated codec
then converts the request to CBOR. The CLI sends only a CSIL-RPC CBOR envelope
to the server.

Timestamp fields accept RFC3339, `YYYY-MM-DD`, `today`, and `tomorrow` in all
input modes. The CLI converts these values to RFC3339 before validation.

## Input mode selection

The CLI uses this order:

1. If `--input` is present, read JSON from the specified file or standard
   input. Do not show a form.
2. If `--no-input` is present, use only arguments and options. Do not prompt.
3. If `--interactive` is present, require a terminal and show the form.
4. If required input is missing and standard input is a terminal, show the
   form.
5. If required input is missing and standard input is not a terminal, return a
   usage error.
6. If all required input is present, execute the command without a form.

List and view commands do not show a form. Short workflow commands such as
`task done` do not show a form. A destructive command can show a confirmation
prompt when a terminal is available.

## Data flow

```text
arguments and options  --+
terminal form          ---+--> generated request type
JSON file or stdin     --+            |
                                     v
                         generated constructor defaults
                                     |
                                     v
                         CLI normalization and checks
                                     |
                                     v
                         generated Validate method
                                     |
                                     v
                         generated CBOR operation codec
                                     |
                                     v
                              CSIL-RPC carrier
                                     |
                                     v
                              Longhouse server
```

The return path uses the generated response decoder. A renderer then produces
plain text or JSON.

## Validation

The CLI must use the generated Go client validators. This gives direct and
consistent feedback before a network request. The CLI must not copy CSIL
length constraints into a separate manual validator.

The current generated `Validate` methods check constraints that the CSIL
generator emits. In the current schema, these are mainly minimum and maximum
string lengths. The generated constructors also apply CSIL defaults. The CLI
must call both parts explicitly. The codec does not replace a validation call.

Generated validation is one validation layer. It is not complete validation.
The CLI must also perform operation-specific checks:

- Parse human date input and convert it to the canonical timestamp format.
- Limit enum input to values that the operation supports.
- Resolve names and short references to stable resource identifiers.
- Reject receive-only fields in JSON request input.
- Check required option combinations.
- Check that a field is valid for the selected operation.

The server remains authoritative for these checks:

- Identity, membership, and role permissions
- Resource existence and current state
- House boundaries
- Database conflicts and uniqueness
- Dependency cycles
- Visibility and sharing rules
- Changes that occur after the CLI reads a resource

The CLI must display server validation errors without replacing the server
message with a generic error.

For an edit operation, the first implementation can read the current resource,
merge the requested values, validate the result, and send the full update.
This method has a concurrency risk. A later API change must add partial updates
or an expected revision value.

## Generated client boundary

CLI protocol code must use the generated Go client package. It must not call
server service implementations or database code. The generated package gives
the CLI the same request types, defaults, validators, codecs, and response
types as other Go clients.

The embedded CSIL schema descriptor supports discovery. Generated Go code
supports request execution. Runtime schema interpretation must not replace the
generated codec when a generated operation codec exists.

## Terminal form library

Use [Huh version 2](https://github.com/charmbracelet/huh) for the first form
implementation. The Go module pins version 2.0.3. Huh supplies form fields,
field validators, terminal rendering, and an accessible prompt mode. It uses
Bubble Tea internally.

Keep Huh behind a small internal form interface. This boundary permits a later
move to [Bubble Tea version 2](https://github.com/charmbracelet/bubbletea) if a
workflow needs more state than a form can supply. Do not build a full-screen
application for the first version.

The implementation must pin and review the dependency version. The design does
not require a TUI dependency for non-interactive use. Form startup must occur
only after the CLI selects interactive mode.

An accessible form mode must be available through `--accessible` and the
`LONGHOUSE_ACCESSIBLE` environment variable.

## Command model

Use a command registry as the source of truth. The registry reads the method
signatures from the generated Go client. A small metadata table supplies the
human command path, positional arguments, aliases, and safety classification.
The hand-written parser reads this registry. Help text, structured help, shell
completion, forms, and raw API discovery also read it.

Each command specification contains:

- Command path
- One-line summary
- Docopt-style usage lines
- Positional arguments
- Options and environment variables
- Input and output types
- CSIL service and operation
- Mutation and destructive-operation flags
- Examples
- Related commands

The registry produces normal docopt-style help for people:

```text
Usage:
  longhouse task create [<title>] [options]
  longhouse task create --input <file> [options]

Options:
  --title <text>          Task title.
  --project <ref>         Parent project.
  --due <date>            Due date or time.
  --interactive           Show the terminal form.
  --input <file>          Read JSON from a file. Use - for standard input.
  --output <format>       Output format: text or json [default: text].
  --no-input              Do not prompt.
```

Docopt-style text is useful for people, but a language model must not have to
parse that text. The same registry produces structured help:

```text
longhouse help --json
longhouse help task --json
longhouse help task create --json
```

Structured help includes accepted arguments, option types, required fields,
defaults, examples, side effects, output fields, and related commands. It uses
a versioned schema. A caller can start with the top-level command list and
request more detail only for the selected command. This keeps discovery output
small.

## Proposed hierarchy

```text
longhouse
  auth
    login | status | refresh | logout
    session list | revoke
  profile
    list | view | add | use | remove
    house use
  house
    list | view | create | edit | delete
  task
    list | view | create | edit
    start | done | cancel | reopen | delete
    visibility get | set
    grant list | set | remove
  project
    list | view | create | edit | activate | archive | delete
    task list | add | remove | move
    member list | add | remove
    owner list | add | remove
    milestone list | create | edit | delete
    visibility get | set
    grant list | set | remove
  event
    list | view | create | edit | delete | delete-future
  calendar
    view
    subscription list | create | delete
  member
    list | view | invite | edit | remove | activate
    role list | add | remove
    skill list | add | remove
    audit
  group
    list | view | create | edit | delete
    member list | add | remove
    skill list | add | remove
  role
    list | view | create | edit | delete
  skill
    list | view | create | edit | delete
  comment
    list | view | add | edit | delete
  dependency
    list | add | remove
  access
    share list | create | delete | check
  notification
    list | count | read | read-all
  admin
    domain list | add | remove
    settings view | edit
    audit list
    trash list | restore | purge
  bug
    report
  api
    list | describe | call
  completion
  help
```

The hierarchy uses nouns before verbs. Nested nouns show ownership or a
relationship. Workflow verbs make common state changes clear:

- `task start` sets a task to in-progress.
- `task done` sets a task to done.
- `task cancel` sets a task to cancelled.
- `task reopen` sets a task to open.
- `project archive` archives a project.
- `project activate` returns a project to active use.

The command registry must map each workflow command to a normal CSIL request.
The server does not need special transport paths for these commands.

## Resource references

All commands accept a stable resource identifier. Commands can also accept an
exact name when the name is unique in the active house. A reference can include
its resource type when the target type is not otherwise clear.

```text
longhouse comment add task:task_01J... --body "Blocked by the permit"
longhouse dependency add task:task_01J... project:project_01J...
```

The CLI must show all matches when a name is not unique. It must not select the
first match without user confirmation.

Short, per-house task numbers such as `#42` would improve human use. The API
does not currently provide these numbers. This is a recommended later API
change, not a first-version requirement.

## Profiles and context

A profile stores a server URL and credentials. It can also store an active
house. A command option overrides profile context.

```text
longhouse profile use dev
longhouse profile house use house_01J...
longhouse --profile dev --house house_01J... task list
```

Use this precedence order:

1. Command option
2. Environment variable
3. Active profile value
4. Interactive selection, when a terminal is available
5. A clear error

Credentials must not appear in normal output, debug output, structured help,
or form initial values.

## Output

Plain text is the default output. Lists use compact tables. View commands use
labeled fields. Mutation commands print the stable identifier and a short
result summary.

JSON output uses a versioned envelope:

```json
{
  "schema_version": "1",
  "ok": true,
  "data": {}
}
```

Errors use the same output selection. JSON errors include a stable error code,
the message, the command path, and field details when available. Secret values
must never occur in an error value.

Standard output contains only the result. Forms, progress, warnings, and
diagnostic text use standard error or the terminal output stream. `--quiet`
prints only the primary identifier for a successful mutation.

Pagination must be explicit. Audit and trash commands print the next cursor in
text and JSON output. Other list commands accept explicit limit and offset
values. Automation must not depend on an implicit terminal pager.

## Safety

- A destructive command prompts before it sends a request when a terminal is
  available.
- A destructive command requires `--yes` when prompting is disabled.
- `--dry-run` validates and displays the planned request without a mutation
  when the operation can support this behavior safely.
- A cancelled form sends no request.
- An interrupted command returns exit status 130.
- The CLI must not retry a mutation unless the operation has an idempotency
  key.
- Shell examples must pass argument arrays internally. The CLI must not build
  a shell command from resource titles or server data.

## Raw API commands

The raw API commands make every CSIL operation available before each operation
has a polished command.

```text
longhouse api list
longhouse api describe TaskService.Create
longhouse api call TaskService.Create --input request.json
```

`api list` and `api describe` read the operation registry that the generated Go
client methods supply. `api call` invokes the selected generated typed client.
It accepts JSON locally and sends CBOR over CSIL-RPC. It applies the same strict
JSON decoding and generated validation rules as a polished command. The CSIL
schema descriptor remains available for tools that need schema data outside
the Go type system.

Raw API commands are an escape path. They do not replace the task-oriented
hierarchy.

## API gaps

The first CLI can work with the current API. These API changes would improve
reliability and scale:

1. Add short, per-house task numbers for human references.
2. Add server-side filters and stable pagination for large list operations.
3. Add partial updates or expected revision values to prevent lost updates.
4. Add idempotency keys for safe mutation retries.
5. Implement and register `ShareService` before clients use `access share`.
6. Implement and register `MemberAuditService`, or remove it from the CSIL
   contract. The current server does not register this legacy service.

## Implementation

The CLI implements these parts:

1. A registry that covers every generated CSIL client operation.
2. Text help and versioned JSON help from the same registry.
3. Named profiles, active-house context, output streams, exit codes, and strict
   JSON input.
4. Generated Go client constructors, validators, typed clients, and CBOR
   codecs.
5. `api list`, `api describe`, and `api call`.
6. The resource command groups in the proposed hierarchy.
7. Huh forms for create and edit operations.
8. Searchable form choices for resource identifiers that have list operations.
9. Shell completion output from the command registry.
10. Concise agent instructions from `longhouse agent instructions`.
11. Typed references such as `task:TASK_ID` for fields that accept more than
    one resource type.
12. Output-field, effect, environment, example, and related-command data in
    exact structured help.

The short task numbers, server-side filters, safe partial updates, and
idempotency keys remain API work. They are not CLI-only changes.

Each step must include tests for terminal and non-terminal input. Tests must
also confirm that text on standard error does not change JSON on standard
output.

## References

The hierarchy and output model use established CLI patterns:

- [GitHub CLI issue commands](https://cli.github.com/manual/gh_issue) use a
  noun and verb hierarchy.
- [GitHub CLI formatting](https://cli.github.com/manual/gh_help_formatting)
  provides explicit JSON output and field selection.
- [Taskwarrior commands](https://taskwarrior.org/docs/commands/) provide short
  workflow verbs and confirmation for changes.
- [Atlassian CLI work item search](https://developer.atlassian.com/cloud/acli/reference/commands/jira-workitem-search/)
  provides a deep resource hierarchy and explicit machine output.
- [Huh](https://github.com/charmbracelet/huh) provides terminal forms and field
  validators.
