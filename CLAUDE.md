# Longhouse

Coordination system for small-to-medium organizations and neighborhoods. Provides task management, calendaring, commentary, and coordination features.

## Architecture

- **api/**: Go API server with TCP (CSIL protocol, primary) and HTTP (secondary) interfaces
- **webapp/**: Go server-rendered HTML with HTMX for the web UI
- **coredb/**: Embedded SQL migrations module (goose)
- **csil/**: CSIL service definitions for code generation via csilgen
- **helm_chart/**: Kubernetes Helm chart with Gateway API (TCPRoute + HTTPRoute) and legacy Ingress support

## Auth

Uses linkkeys as external IDP. Local member records cache identity data. Trusted domains and explicit user lists control access. Per-resource sharing supports external READ access via linkkeys identity.

## Dev Setup

```bash
docker compose up postgres    # Start PostgreSQL
cd api && go run . serve      # Start API server
cd webapp && go run . serve   # Start web UI
```

## Environment Variables

All prefixed with `LONGHOUSE_`. See `api/internal/config/config.go` for full list.

## Conventions

- PostgreSQL only (no SQLite)
- CSIL-first API design, HTTP is secondary
- No external CLI framework (hand-rolled docopt-style)
- goose for migrations, GORM for ORM
- Multi-module Go project (api, webapp, coredb are separate modules)
