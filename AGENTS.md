# Pruvon Agent Guide

## Commands

- **Build**: `go build -o pruvon ./cmd/app` or `make build` (cross-compile for linux amd64/arm64)
- **Run**: `./pruvon -server -config config.yaml` (server mode) or `./pruvon -backup auto` (backup mode)
- **Test**: `go test ./...` (all tests) or `go test ./internal/services` (single package) or `go test -run TestName ./path/to/package` (single test)

## Architecture

- **Entry Point**: `cmd/app/main.go` - server mode or backup operations
- **Config**: YAML-based (`config.yaml`) with admin auth, GitHub OAuth, backup settings, and app/service permissions
- **Internal Structure**: `internal/` organized by domain (backup, config, crypto, docker, dokku, exec, handlers, middleware, models, server, services, ssh, stream, system, templates)
- **Tech Stack**: Go + Fiber web framework, Alpine.js (frontend), xterm.js (terminal via WebSocket), Tailwind CSS
- **Key Features**: Dokku management UI, real-time terminal, database backups (postgres/mariadb/mongo/redis), GitHub OAuth

## Code Style

- **Language**: English for all code, comments, and user-facing content 
- **Best Practices**: Follow Go conventions, use existing project patterns, maintain premium UI/UX consistency
- **Imports**: Standard library first, then external packages, then internal packages (e.g., `pruvon/internal/...`)
- **Error Handling**: Return errors, wrap with context when needed, log appropriately
- **Testing**: Use `testing` package, `testify` for assertions, integration tests suffixed with `_integration_test.go`

## Alpine Safety

- Treat Alpine expressions as eagerly evaluated. Do not assume inner expressions are safe just because a parent `x-show`, `x-if`, or tab is currently hidden.
- Keep every element that reads or writes Alpine component state inside the same `x-data` root. After editing modals, overlays, flash messages, or teleported-looking blocks, verify the surrounding closing tags so the markup does not escape its Alpine scope.
- Avoid `x-teleport` for shared/stateful modal partials unless there is a strong reason. In this codebase it can break Alpine scope expectations, `x-ref` lookups, and modal/button interactions without obvious console errors. Prefer rendering modals inside the owning `x-data` root, and for xterm-style widgets wait until the target element is actually connected before calling `.open()`.
- For async or API-populated state, initialize the Alpine data shape to match the template usage. Prefer defaults like arrays, objects, and nested placeholders instead of `{}` when the template expects nested keys.
- When reading dynamic keys or async data in templates, always use defensive fallbacks: `foo?.bar`, `items || []`, `map || {}`, `(lookup[key] || [])`, `(lookup[key] || '')`, etc.
- Never call `.length`, `.includes()`, iterate with `x-for`, or index into dynamic objects/arrays unless the expression is protected by a fallback or the state is initialized with that exact shape.
- Normalize API responses before assigning them into Alpine state when the template expects a stable type.

## UI Design System

- For any UI, layout, component, Tailwind, or visual styling work, read `.agents/skills/pruvon-ui/SKILL.md` first.
- `.agents/skills/pruvon-ui/SKILL.md` is the authoritative design system for Pruvon UI work.
- Do not deviate from that file unless the user explicitly requests an exception.
