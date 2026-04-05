# Pruvon

[![CI](https://github.com/pruvon/pruvon/actions/workflows/ci.yml/badge.svg)](https://github.com/pruvon/pruvon/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/pruvon/pruvon)](https://github.com/pruvon/pruvon)
[![License: AGPL-3.0](https://img.shields.io/github/license/pruvon/pruvon)](LICENSE)
[![Release](https://img.shields.io/github/v/release/pruvon/pruvon)](https://github.com/pruvon/pruvon/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/pruvon/pruvon)](https://goreportcard.com/report/github.com/pruvon/pruvon)
[![CodeQL](https://github.com/pruvon/pruvon/actions/workflows/codeql.yml/badge.svg)](https://github.com/pruvon/pruvon/actions/workflows/codeql.yml)

Pruvon is a web UI for Dokku. It runs alongside a Dokku host and gives you a browser-based interface for app operations, service management, backups, logs, Docker visibility, and terminal access.

It is not a general-purpose server management panel. The product is built around Dokku workflows and Linux hosts where Dokku is already installed.

## Features

- **Dokku Web UI** - Manage Dokku apps, services, and operational workflows from the browser
- **Database Backups** - Run and retain PostgreSQL, MariaDB, MongoDB, and Redis backups through Dokku plugins
- **Logs and Terminal Access** - Use browser-based logs and terminal sessions over WebSockets
- **Docker Visibility** - Inspect containers, images, and host resource usage
- **Operational Tools** - Manage SSH keys, templates, users, and related Dokku tasks

## Requirements

- Dokku installed on a Linux host

## Install On A Dokku Host

Build the Linux artifacts, then run the installer as root on the target Dokku host:

```bash
git clone https://github.com/pruvon/pruvon.git
cd pruvon
make build
sudo ./install.sh
```

`make build` produces Linux `amd64` and `arm64` binaries in `builds/`.

If you want to force a specific artifact, set `PRUVON_BINARY` explicitly:

```bash
sudo PRUVON_BINARY=builds/pruvon-linux-amd64 ./install.sh
```

`install.sh` will:

- install the binary to `/opt/pruvon/pruvon`
- create `/usr/local/bin/pruvon` as a symlink to the installed binary
- create the `pruvon` service user
- add `pruvon` to the `adm` and `dokku` groups, plus `docker` if that group exists
- create runtime and data directories such as `/var/lib/pruvon`, `/var/log/pruvon`, and `/var/lib/dokku/data/pruvon-backup`
- create `/etc/pruvon.yml` from `pruvon.yml.example` if it does not exist
- generate a random admin password and store only its bcrypt hash in config
- install the systemd unit, sudoers policy, cron job, and logrotate config
- enable and start the `pruvon` systemd service

The generated admin password is printed once at the end of installation.

Installer notes:

- `PRUVON_BINARY` can point to any built binary if you do not want auto-detection.
- If `/etc/pruvon.yml` already exists, the installer keeps it and does not rotate the admin password.
- Dokku, nginx, sudo, and systemd are expected to already be present on the host.

## Operate The Installed Service

Service management:

```bash
sudo systemctl status pruvon
sudo systemctl restart pruvon
sudo journalctl -u pruvon -f
```

Installed paths:

- Config: `/etc/pruvon.yml`
- Binary: `/opt/pruvon/pruvon`
- Symlink: `/usr/local/bin/pruvon`
- Systemd unit: `/etc/systemd/system/pruvon.service`
- Sudoers policy: `/etc/sudoers.d/pruvon`
- Daily backup cron script: `/etc/cron.daily/pruvon-backup`
- Logrotate config: `/etc/logrotate.d/pruvon`
- Activity log: `/var/log/pruvon/activity.log`
- Backup log: `/var/log/pruvon/backup.log`
- Backup directory: `/var/lib/dokku/data/pruvon-backup`

Operational notes:

- The example config listens on `127.0.0.1:8080`; change `pruvon.listen` in `/etc/pruvon.yml` if you want a different bind address.
- After editing `/etc/pruvon.yml`, restart the service with `sudo systemctl restart pruvon`.
- The installed cron script runs `pruvon -backup auto -config /etc/pruvon.yml` once per day and writes output to `/var/log/pruvon/backup.log`.
- The installed logrotate policy rotates `*.log` files in `/var/log/pruvon` monthly and keeps 12 compressed archives.
- The sudoers policy allows the `pruvon` service account to run the Dokku, nginx, and storage ownership commands required by the UI.

## Manual Commands

Run the server manually:

```bash
pruvon -server -config /etc/pruvon.yml
```

Run backups manually:

```bash
pruvon -backup auto -config /etc/pruvon.yml
pruvon -backup daily -config /etc/pruvon.yml
pruvon -backup weekly -config /etc/pruvon.yml
pruvon -backup monthly -config /etc/pruvon.yml
```

## Configuration

For local development or a manual setup, start from the provided example:

```bash
cp pruvon.yml.example pruvon.yml
go run ./cmd/app -server -config pruvon.yml
```

The default production config path is `/etc/pruvon.yml`.

`pruvon.yml.example` mirrors the app's expected config shape. Replace the example admin password hash and GitHub settings before real use.

## Development

Development requirements:

- Go 1.26+
- A recent `golangci-lint` installation if you want to run `make lint`

Build release artifacts for Dokku hosts:

```bash
make build
```

Build a host-native development binary if needed:

```bash
go build -o pruvon ./cmd/app
```

Common local verification commands:

```bash
make fmt
make vet
make test
make lint
```

`make lint` expects a recent `golangci-lint` installation compatible with the Go toolchain in use.

## Uninstall

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
