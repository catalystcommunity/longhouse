# Longhouse

Coordination system for small-to-medium organizations and neighborhoods.
Tasks, calendar, projects, members, groups, and skills under a per-house
membership model.

## Quick start

```bash
./tools dev        # postgres (docker, host port 5433) + api + vite, sourcing .env.dev
```

This starts Postgres in Docker, the Go API, and the Vite dev server, then
foregrounds a tail of `logs/api.log` + `logs/vite.log` (Ctrl-C just stops
the tail, not the services). The SPA is served at `http://localhost:5173`
and proxies `/api/*` to the API at `localhost:6080`.

`.env.dev` in the repo root holds local-only config (Postgres port, JWT
secret, dev-auth flag, initial admin identity) and is gitignored. With
`LONGHOUSE_ENV=dev` and dev-auth enabled, visit `/dev-login` in the SPA to
sign in without a live linkkeys identity provider.

Tear down with:

```bash
./tools dev-down   # stops services, keeps the postgres volume
```

## Tests

```bash
./tools test       # run all tests (API + webapp)
./tools test-api   # run API tests only
./tools test-web   # run webapp tests only
```

## Layout

- `api/` — Go API server (single CSIL-RPC endpoint, no REST)
- `webapp/` — SolidJS + Vite SPA
- `clients/` — generated CSIL client libraries, one package per language
- `csil/` — CSIL service/type definitions (source of truth for server + clients)
- `coredb/` — SQL migrations (goose)
- `helm_chart/` — Kubernetes Helm chart
- `deploy/` — per-environment Helm values

## Auth

Sign-in is delegated to [linkkeys](https://github.com/catalystcommunity/linkkeys)
as the identity provider. Longhouse keeps a local member record that caches the
identity data linkkeys releases; who may sign in is controlled by trusted
domains and explicit user lists.

The bearer token snapshots the caller's per-house membership and roles at mint
time, so authorization needs no database lookup per request. Staleness is
bounded by the token's `exp` and by the `AuthService.Refresh` op.

Access bearers expire after 12 hours by default. Set
`LONGHOUSE_BEARER_TTL_SECONDS` to change this lifetime. CLI refresh sessions
expire after 30 days by default. Set `LONGHOUSE_REFRESH_TOKEN_TTL_SECONDS` to
change their absolute lifetime. Refresh-token rotation does not extend that
lifetime.

## Command-line interface

The release binary contains the API server and the Longhouse command-line
interface (CLI). Sign in before you use resource commands:

```bash
longhouse auth login --url=https://longhouse.example
longhouse auth status
```

`auth login` opens a browser. The browser uses the normal linkkeys flow and asks
the user to approve the named CLI. For an SSH session, use `--no-browser` and
open the printed URL on another device. The old top-level `login`, `status`,
`refresh`, and `logout` commands remain as aliases.

The CLI stores named profiles in the operating system user configuration
directory. The profile file has mode `0600`. Each signed-in profile contains a
short-lived access bearer and a rotating refresh token. The CLI does not print
these values.

```bash
longhouse profile add dev --url=https://longhouse.example
longhouse auth login --profile dev
longhouse profile use dev
longhouse profile house use HOUSE_ID
```

Use noun and verb commands for Longhouse resources:

```bash
longhouse task list
longhouse task create "Repair the west gate" --due tomorrow
longhouse task done TASK_ID
longhouse project member add PROJECT_ID MEMBER_ID
longhouse comment add task:TASK_ID --body "Blocked by the permit"
longhouse dependency add task:TASK_ID project:PROJECT_ID
```

A create or edit command shows a terminal form when required input is missing.
Use `--interactive` to request the form. Use `--no-input` to stop all prompts.

Plain text is the default output. Use JSON only when a program needs it:

```bash
longhouse task create --input task.json --output json
longhouse help task create --json
longhouse api describe TaskService.Create --json
```

The CLI converts local JSON to generated Go client types. It validates the
request and sends CBOR through the CSIL-RPC endpoint. It does not send JSON to
the server. Run `longhouse help` to see the command hierarchy. Run `longhouse
api list` to see every generated CSIL operation. Automated callers can start
with `longhouse agent instructions --json` and `longhouse help --json`.

The account page lists CLI sessions. A user can revoke a session there. A
revoked session cannot refresh again. Its current bearer can work until the
configured bearer lifetime ends.

## Documentation

- `AGENTS.md` — architecture, conventions, and the deploy/dogfood loop
- `docs/rbac.md` — access levels, visibility, and the grant model
- `docs/dependencies.md` — task and project dependency edges
- `docs/cli_approach.md` — CLI hierarchy, input modes, validation, and output
