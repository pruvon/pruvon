# Pruvon Agent Guide

## Scope

- Pruvon is a Dokku-focused server management app. Treat Dokku, Docker, Linux filesystem paths, and PTY/WebSocket terminal support as core product assumptions.
- Optimize for small, safe changes that preserve current routes, config shape, and template names unless the task explicitly requires broader refactoring.
- Use English for code, comments, docs, and user-facing strings unless the task explicitly asks for another language.
- Target Go 1.22+.

## Verified Commands

- Build locally: `make build` or `go build -o pruvon ./cmd/app`
- Build Linux artifacts: `make build-linux`
- Run server: `go run ./cmd/app -server -config config.yaml` or `./pruvon -server -config config.yaml`
- Run backup: `go run ./cmd/app -backup auto -config config.yaml`
- Show version: `go run ./cmd/app -version`
- Format: `gofmt -w <files>`
- Vet: `go vet ./...`
- Test: `go test ./...`
- CI-equivalent test pass: `go test -v -race -coverprofile=coverage.out ./...`
- Release build pattern: `go build -trimpath -ldflags="-s -w -X main.PruvonVersion=<version>" -o dist/pruvon-linux-<arch> ./cmd/app`
- Start local config from `config.yaml.example`.
- A root `Makefile` exists for common build and verification tasks; direct Go commands remain valid.
- Do not assume a checked-in `.golangci.yml`; lint runs via the GitHub Action config.

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

## Routing And Auth Invariants

- Preserve route ordering where comments call it out.
- In `internal/handlers/api/service.go`, specific service routes must stay above wildcard service routes.
- In `internal/handlers/ws/ws.go`, `/ws/services/import/:taskId` must stay above `/ws/services/:type/:name/console`.
- Static assets are intentionally served after auth middleware; treat `/static/*` as authenticated unless the task explicitly changes that behavior.
- Session lookups use both `username` and legacy `user`; preserve both unless a migration is explicitly requested.
- GitHub-authenticated users are revalidated against config on request and should lose access immediately if removed from config.

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
- Some backup, static, and streaming behavior depends on environment or embedded assets. Keep tests deterministic and avoid introducing host-specific assumptions.

## High-Risk Areas

- Auth middleware and permission checks across web, API, and WebSocket routes
- Route ordering with wildcard patterns
- Template file names, parse trees, and settings-page template composition
- WebSocket PTY/terminal flows
- Dokku command parsing and Linux path assumptions
- Backup rotation and filesystem permissions
- Static asset serving and auth behavior
