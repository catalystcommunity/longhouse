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
