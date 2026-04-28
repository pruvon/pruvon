# Pruvon Agent Guide

## Scope

- Pruvon is a Dokku-focused web UI and control plane, not a general-purpose server management app. Treat Dokku, Docker, Linux filesystem paths, and PTY/WebSocket terminal support as core product assumptions.
- Optimize for small, safe changes that preserve current routes, config shape, and template names unless the task explicitly requires broader refactoring.
- Use English for code, comments, docs, and user-facing strings unless the task explicitly asks for another language.
- Target Go 1.26+.

## Verified Commands

- Build Linux artifacts: `make build`
- Format: `make fmt` or `gofmt -w <files>`
- Vet: `make vet` or `go vet ./...`
- Test: `make test` or `go test ./...`
- CI-equivalent test pass: `go test -v -race -coverprofile=coverage.out ./...`
- Local lint: `make lint` or `golangci-lint run --timeout=5m`
- Run server: `go run ./cmd/app -server -config pruvon.yml` or `./pruvon -server -config pruvon.yml`
- Run backup: `go run ./cmd/app -backup auto -config pruvon.yml`
- Show version: `go run ./cmd/app -version`
- Release build pattern: `go build -trimpath -ldflags="-s -w -X main.PruvonVersion=<version>" -o dist/pruvon-linux-<arch> ./cmd/app`
- Start local config from `pruvon.yml.example`.
- A root `Makefile` exists for common build and verification tasks; direct Go commands remain valid.
- A checked-in `.golangci.yml` mirrors the GitHub Action lint timeout; keep local and CI lint expectations aligned.
- If local lint fails with Go or analyzer version errors, treat that as an environment/tooling mismatch first and compare against CI.

## Entry Points And Runtime

- Main entry point: `cmd/app/main.go`
- Supported flags:
  - `-server`
  - `-backup auto|daily|weekly|monthly`
  - `-config <path>` with default `/etc/pruvon.yml`
  - `-version`
- In server mode, a missing config triggers default config creation with bcrypt-hashed `admin/admin` credentials.
- Server boot order is: Fiber app -> CORS -> config middleware -> version/update middleware -> route setup -> embedded static handler -> `app.Listen`.
- Run server mode with a valid config. Outside backup/version flows, the app expects loaded config data.

## Architecture Map

- `internal/config`: YAML load/save, global config state, request middleware, atomic save via temp file + rename.
- `internal/server`: default config creation, version/update middleware, embedded static asset handler.
- `internal/appdeps`: shared runtime dependency container for handlers.
- `internal/handlers/web`: HTML page handlers.
- `internal/handlers/api`: JSON endpoints for apps, services, backups, users, templates, docker, activity, and related workflows.
- `internal/handlers/ws`: WebSocket endpoints for logs, terminals, import progress, and Docker terminals.
- `internal/dokku`: Dokku command wrappers and output parsing. Keep Dokku-specific command logic here or in services, not in handlers.
- `internal/services/apps`: higher-level app orchestration on top of Dokku.
- `internal/services/logs`: activity logging and log search/tail helpers.
- `internal/services/update`: GitHub-based version check.
- `internal/backup`: scheduled/manual database backup and retention workflow.
- `internal/docker`: Docker stats and container/resource operations.
- `internal/exec`, `internal/stream`: command execution, PTY handling, and WebSocket streaming.
- `internal/templates`: template loading and rendering helpers.
- `internal/templates/html`: base layout, partials, settings templates, and page templates.
- `static`: embedded JS/CSS/images and JSON app templates.

## Working Conventions

- Prefer existing dependency injection patterns such as `appdeps.Dependencies`, `dokku.CommandRunner`, and `exec.CommandRunner`.
- Prefer testable command execution through runners instead of adding new direct `os/exec` calls in handlers.
- Wrap returned errors with context using `%w` where appropriate.
- Preserve route paths and request/response shapes unless the task explicitly calls for API changes.
- Keep changes localized. This codebase generally favors direct, file-local logic over abstraction-heavy refactors.
- When persisting config changes, use `config.SaveConfig`; it writes back to the original loaded path atomically.
- Be careful with hardcoded operational paths. Existing code expects Linux paths such as:
  - `/etc/pruvon.yml`
  - `/var/lib/dokku/data/pruvon-backup`
  - `/var/log/pruvon/activity.log`
  - `/var/lib/dokku/...`
  - `/var/log/nginx/<app>-{access,error}.log`

## Task Workflow

- Maintain the root `TASKS.md` file for any phased or multi-step implementation that should survive across turns.
- Update `TASKS.md` before starting substantial work, and keep statuses accurate as phases complete.
- Treat `TASKS.md` as the persistent source of truth for open work unless the user explicitly replaces the plan.
- For phased implementation requests, continue through all listed phases in `TASKS.md` unless blocked or redirected by the user.

## Routing And Auth Invariants

- Preserve route ordering where comments call it out.
- In `internal/handlers/api/service.go`, specific service routes must stay above wildcard service routes.
- In `internal/handlers/ws/ws.go`, `/ws/services/import/:taskId` must stay above `/ws/services/:type/:name/console`.
- Static assets are intentionally served after auth middleware; treat `/static/*` as authenticated unless the task explicitly changes that behavior.
- Session lookups use both `username` and legacy `user`; preserve both unless a migration is explicitly requested.
- Configured users are revalidated against config on request and should lose access immediately if removed or disabled.

## Templates And Frontend

- Frontend is server-rendered `html/template` plus Alpine.js, fetch-based API calls, and WebSocket updates. There is no SPA build pipeline.
- `internal/templates/templates.go` supports two modes:
  - embedded templates in normal runtime
  - local disk templates from `internal/templates/html` in development, with cache bypass
- Do not break either template-loading path when renaming files or changing template names.
- Base and partial templates are parsed together. The settings page also injects parse trees from other templates, so template name changes there are high-risk.
- Template initialization failure panics during route setup, so template edits should be verified carefully.
- Interactive pages usually follow the pattern: server-rendered HTML + inline Alpine state + `fetch('/api/...')` + optional WebSocket updates.
- Keep Alpine state shapes stable and defensively initialized for async data.
- Keep elements that share Alpine state inside the same `x-data` scope.
- Avoid `x-teleport` for shared/stateful modal flows unless there is a strong reason.
- Guard dynamic template expressions with safe fallbacks before using `.length`, indexing, `x-for`, or `.includes()`.
- Only load xterm or ApexCharts when the page data uses `LoadXTerm` or `LoadApexCharts`.
- For any UI, layout, Tailwind, or visual styling change, read `.agents/skills/pruvon-ui/SKILL.md` first and follow it as the source of truth.
- When adding user-triggered async actions (save, delete, confirm) in templates, always follow the loading state pattern defined in the UI skill: show a processing spinner, change the button label to "Processing...", disable all modal buttons, and reset the flag in a `finally` block.
- **Never make Alpine event handlers `async`.** `@click` handlers with the `async` keyword silently fail — the handler runs but reactive DOM mutations may not apply. Keep handlers synchronous and use fire-and-forget for async work. See `.agents/skills/pruvon-ui/SKILL.md` § 10.2 for details.
- **Avoid `<template x-for>` inside `<select>` elements.** Browser form-filling extensions (LastPass, etc.) can break parsing, causing options to not render. Use a custom dropdown with a `<button>` trigger and `x-for` in a `<div>` container instead. See `.agents/skills/pruvon-ui/SKILL.md` § 7.3 for the pattern.
- **Use `x-show` + `x-cloak` for modal overlays, never `x-if`.** This ensures Alpine.js initializes all nested directives (especially `x-for` in dropdowns) at page load, before any async fetch completes. See `.agents/skills/pruvon-ui/SKILL.md` § 7.4.
- **Never combine `backdrop-blur-*` with `overflow-y-auto` on the same element.** Browsers fail to render backdrop blur correctly on scrollable containers, leaving unblurred white areas. Always use a separate fixed backdrop div for blur and a separate fixed div for scrollable content. See `.agents/skills/pruvon-ui/SKILL.md` § 7.4.

## Testing Expectations

- Use Go's `testing` package with `testify/assert` and `testify/require`.
- Handler tests commonly use `httptest`; follow that pattern.
- Prefer command-runner stubs/fakes over requiring real Dokku or Docker in unit tests.
- Use `_integration_test.go` for broader integration coverage.
- Existing handler coverage includes smoke-style tests; do not assume light handler tests alone fully prove behavior.
- For non-trivial changes, run:
  - `gofmt -w <files>`
  - `go vet ./...`
  - `go test ./...`
- When validating against CI behavior, also consider `go test -race ./...`.
- **Mandatory: always run `make lint` (or `golangci-lint run --timeout=5m`) after writing or editing code, before considering the task complete.** Do not rely on `go vet` alone catching all issues; golangci-lint includes the `unused` checker and other analyzers that `go vet` does not. A CI failure due to an unused function, variable, or import that was not caught locally indicates the lint step was skipped.
- **Release gating is strict:** before any tag or GitHub release, the release commit must pass `go vet ./...`, `go test -v -race -coverprofile=coverage.out ./...`, and `make lint`. Do not publish a release until the pushed commit has a successful `CI` workflow run.
- Some backup, static, and streaming behavior depends on environment or embedded assets. Keep tests deterministic and avoid introducing host-specific assumptions.

## High-Risk Areas

- Auth middleware and permission checks across web, API, and WebSocket routes
- Route ordering with wildcard patterns
- Template file names, parse trees, and settings-page template composition
- WebSocket PTY/terminal flows
- Dokku command parsing and Linux path assumptions
- Backup rotation and filesystem permissions
- Static asset serving and auth behavior
