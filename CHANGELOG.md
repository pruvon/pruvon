# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project aims to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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


