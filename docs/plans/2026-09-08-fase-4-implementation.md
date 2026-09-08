# Fase 4: Polish & Ekosistem Implementation Plan

## Task 1: Database Migrations

Create migration files for new tables:
- `internal/database/migrations/009_service_templates.sql`
- `internal/database/migrations/010_api_keys.sql`

## Task 2: Models

Create new models:
- `internal/models/template.go` - ServiceTemplate, TemplateVariable
- `internal/models/api_key.go` - ApiKey, CreateApiKeyInput

## Task 3: Repositories

Create new repos:
- `internal/database/repo/template.go` - TemplateRepo
- `internal/database/repo/api_key.go` - ApiKeyRepo

## Task 4: Observability

Create observability package:
- `internal/observability/health.go` - health check handler
- `internal/observability/metrics.go` - metrics handler
- Update main.go to use slog

## Task 5: API Router & Middleware

Create API package:
- `internal/api/middleware.go` - API key auth middleware
- `internal/api/router.go` - API v1 routes
- `internal/api/handlers.go` - API handlers

## Task 6: Templates Handler & UI

Create templates feature:
- `internal/handler/templates.go` - templates handler
- `internal/web/pages/templates/list.templ` - template catalog
- `internal/web/pages/templates/detail.templ` - template detail

## Task 7: API Keys Handler & UI

Create API keys feature:
- `internal/handler/api_keys.go` - API keys handler
- `internal/web/pages/settings/api_keys.templ` - manage API keys
- `internal/web/pages/settings/api_keys/create.templ` - create API key

## Task 8: Plugin System

Create plugin package:
- `internal/plugin/plugin.go` - plugin interface
- `internal/plugin/loader.go` - plugin loader

## Task 9: CLI

Create CLI:
- `cmd/cli/main.go` - CLI entry point with Cobra

## Task 10: Documentation

Create docs:
- `docs/getting-started.md`
- `docs/deployment.md`
- `docs/api.md`

## Task 11: Wire Everything

Update:
- `cmd/labuh/main.go` - add all new routes and components
- `internal/web/components/sidebar.templ` - add new nav items
- `go.mod` - add cobra dependency if needed

## Task 12: Verify Compilation

Run `go build ./cmd/labuh` and `go build ./cmd/cli`
