# Pruvon

[![CI](https://github.com/pruvon/pruvon/actions/workflows/ci.yml/badge.svg)](https://github.com/pruvon/pruvon/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/pruvon/pruvon)](https://github.com/pruvon/pruvon)
[![License: AGPL-3.0](https://img.shields.io/github/license/pruvon/pruvon)](LICENSE)
[![Release](https://img.shields.io/github/v/release/pruvon/pruvon)](https://github.com/pruvon/pruvon/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/pruvon/pruvon)](https://goreportcard.com/report/github.com/pruvon/pruvon)
[![CodeQL](https://github.com/pruvon/pruvon/actions/workflows/codeql.yml/badge.svg)](https://github.com/pruvon/pruvon/actions/workflows/codeql.yml)

Pruvon is a self-hosted server management platform for Dokku-based applications. It provides a web interface for managing containers, databases, backups, system monitoring, and real-time terminal access.

## Features

- **Dokku Management** - Deploy, scale, and manage Dokku applications
- **Database Backups** - Automated backups for PostgreSQL, MariaDB, MongoDB, and Redis
- **Real-time Terminal** - Browser-based terminal with WebSocket support
- **System Monitoring** - CPU, memory, disk, and network statistics
- **Docker Management** - Container and image management
- **SSH Key Management** - Secure key storage and deployment
- **Web UI** - Modern Alpine.js frontend with Fiber web framework

## Requirements

- Go 1.26+
- Dokku (for server management features)
- Linux server (amd64/arm64)

## Quick Start

### Build from Source

```bash
git clone https://github.com/pruvon/pruvon.git
cd pruvon
make build
```

This creates a local `./pruvon` binary.

For Linux `amd64` and `arm64` artifacts in `builds/`, run:

```bash
make build-linux
```

### Install on Linux

Build a Linux binary first, then run the installer as root on the target host:

```bash
make build-linux
sudo PRUVON_BINARY=builds/pruvon-linux-amd64 ./install.sh
```

The installer will:

- install the binary to `/opt/pruvon/pruvon`
- create the `pruvon` service user
- add `pruvon` to the `adm` and `dokku` groups
- create `/etc/pruvon.yml` from `pruvon.yml.example` if it does not exist
- generate a random admin password and store only its bcrypt hash in config
- install the systemd unit, sudoers policy, cron job, and logrotate config

The generated admin password is printed once at the end of installation.

Installer notes:

- `PRUVON_BINARY` can point to any built binary if you do not want auto-detection.
- If `/etc/pruvon.yml` already exists, the installer keeps it and does not rotate the admin password.
- Dokku, nginx, sudo, and systemd are expected to already be present on the host.

### Uninstall on Linux

Remove the installed service files but keep config, logs, backups, and the service user:

```bash
sudo ./uninstall.sh
```

Remove persistent data too:

```bash
sudo ./uninstall.sh --purge
```

Also remove the `pruvon` system user and group:

```bash
sudo ./uninstall.sh --purge --remove-user
```

Default uninstall keeps `/etc/pruvon.yml`, `/var/log/pruvon`, and `/var/lib/dokku/data/pruvon-backup` so an accidental package removal does not destroy operational data.

Common local verification commands:

```bash
make fmt
make vet
make test
make lint
```

`make lint` expects a recent `golangci-lint` installation compatible with the Go toolchain in use.

### Run in Server Mode

```bash
./pruvon -server -config config.yaml
```

### Run Backup

```bash
./pruvon -backup auto
./pruvon -backup daily
./pruvon -backup weekly
./pruvon -backup monthly
```

## Configuration

Start from the provided example and adjust it for your environment:

```bash
cp pruvon.yml.example config.yaml
./pruvon -server -config config.yaml
```

The default production config path is `/etc/pruvon.yml`.

`pruvon.yml.example` mirrors the app's generated default config shape. Replace the example admin password hash and GitHub settings before real use.

`config.yaml.example` is still kept in the repository for compatibility with existing local workflows, but new install and packaging flows should use `pruvon.yml.example`.

## Changelog Workflow

Release notes are generated from commit subjects.

Generate a new changelog entry for a release:

```bash
make changelog VERSION=0.1.1 PREVIOUS_TAG=v0.1.0
```

If `PREVIOUS_TAG` is omitted, the generator uses the latest `v*` tag it can find.

Best results come from commit subjects that follow a conventional prefix such as:

- `feat: ...`
- `fix: ...`
- `docs: ...`
- `refactor: ...`
- `test: ...`
- `chore: ...`

These prefixes are grouped into changelog sections like `Added`, `Fixed`, `Changed`, `Documentation`, `Tests`, and `Maintenance`.

## Documentation

- [Makefile](Makefile) - Common build and verification commands
- [CHANGELOG.md](CHANGELOG.md) - Release notes

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Security

Please see [SECURITY.md](SECURITY.md) for reporting security vulnerabilities.

## License

Copyright (c) 2026 Pruvon. Licensed under [AGPL-3.0](LICENSE).
