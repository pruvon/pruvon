# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.1] - 2026-04-27

### Changed
- fix(ci): block unverified releases and stabilize user tests


## [0.2.0] - 2026-04-27

### Added
- add auditEventTitle helper for descriptive audit event titles
- add env var edit modal with loading state

### Fixed
- show status before action for deploy audit events

### Changed
- refactor(auth): replace GitHub OAuth with canonical scoped-user model
- feat(audit): add service audit export and improve audit UI
- feat(apps): add download button for environment variables
- add loading states to domain and redirect modal actions
- feat(domains): auto-enable SSL when adding domain if Let's Encrypt is active


## [0.1.5] - 2026-04-12

### Changed
- fix(audit): enrich deploy events with actor and meta from correlated timeline
- perf(dashboard): speed up initial load with async data and skeleton UI
- feat(audit): add dokku-audit integration for dashboard, app and service detail


## [0.1.4] - 2026-04-10

### Changed
- remove update version cache for real-time release detection


## [0.1.3] - 2026-04-10

### Fixed
- sidebar version display and update check mechanism

### Changed
- refactor(ui): improve sidebar collapse with expand icon in header
- feat(ui): update logo and favicon to P branding icon
- feat(ui): update logo and favicon to sun-style icon


## [0.1.2] - 2026-04-10

### Added
- add tracked operations for app stop/restart/rebuild with status polling
- add tracked restart operations with status polling

### Changed
- fix(dokku): run all dokku commands as dokku user for proper permission handling
- fix(services): use sudo -n in shell export commands
- fix(install): resolve latest version via GitHub redirect instead of API
- feat(install): add listen address, service status, and docs links to summary
- fix(install): rotate example admin hash in existing config and add reset docs
- fix(install): escape colon in sudoers storage alias for Ubuntu

### Documentation
- rewrite all public docs as production help pages
- use htpasswd interactive or stdin instead of -b flag


## [0.1.1] - 2026-04-07

### Changed
- docs(changelog): update for v0.1.1 release
- fix(users): use json.Marshal for github_user_permissions_updated activity log


## [0.1.0] - 2026-04-07

### Fixed
- validate backup command before checking prerequisites

### Changed
- replace CI-based release with local make release
- fix(ci): use go install for golangci-lint to resolve Go version mismatch
- feat(install): add PRUVON_LISTEN support and pre-start listen check
- feat(installer): add remote GitHub release installation support
- feat(install): add Linux install and uninstall scripts with changelog workflow
- init

### Documentation
- add docs.pruvon.dev link to README
- add VitePress documentation site
- rewrite AGENTS.md with verified commands, architecture map, and project conventions

### Maintenance
- rename config.yaml to pruvon.yml and refresh project documentation
- upgrade Go toolchain to 1.26 and update all dependencies


