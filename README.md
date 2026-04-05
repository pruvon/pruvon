# Pruvon

[![CI](https://github.com/pruvon/pruvon/actions/workflows/ci.yml/badge.svg)](https://github.com/pruvon/pruvon/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/pruvon/pruvon)](https://github.com/pruvon/pruvon)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL%203.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/pruvon/pruvon)](https://github.com/pruvon/pruvon/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/pruvon/pruvon)](https://goreportcard.com/report/github.com/pruvon/pruvon)
[![CodeQL](https://github.com/pruvon/pruvon/actions/workflows/codeql.yml/badge.svg)](https://github.com/pruvon/pruvon/actions/workflows/codeql.yml)

Pruvon is a web UI for Dokku. It runs alongside a Dokku host and gives you a browser-based interface for app operations, service management, backups, logs, Docker visibility, and terminal access.

It is not a general-purpose server management panel. The product is built around Dokku workflows and Linux hosts where Dokku is already installed.

> [!WARNING]
> Pruvon is under active development and should be considered early-stage software.
> If you choose to run it in environments where breakage, rough edges, or incomplete features are a problem, that risk is yours to take.

## Features

- **Dokku Web UI** - Manage Dokku apps, services, and operational workflows from the browser
- **Database Backups** - Run and retain PostgreSQL, MariaDB, MongoDB, and Redis backups through Dokku plugins
- **Logs and Terminal Access** - Use browser-based logs and terminal sessions over WebSockets
- **Docker Visibility** - Inspect containers, images, and host resource usage
- **Operational Tools** - Manage SSH keys, templates, users, and related Dokku tasks

## Requirements

- Dokku installed on a Linux host

## Install On A Dokku Host

Install the latest release on the target Dokku host:

```bash
curl -fsSL https://pruvon.dev/install.sh | sudo bash
```

If `curl` is not available, use `wget` instead:

```bash
wget -qO- https://pruvon.dev/install.sh | sudo bash
```

Install a specific release tag:

```bash
curl -fsSL https://pruvon.dev/install.sh | sudo PRUVON_VERSION=v0.1.0 bash
```

Install with a custom bind address for a fresh config:

```bash
curl -fsSL https://pruvon.dev/install.sh | sudo PRUVON_LISTEN=127.0.0.1:9090 bash
```

`install.sh` downloads the matching Linux release archive from GitHub Releases, verifies it against `checksums.txt`, and fetches the tagged `pruvon.yml.example` and backup cron script from the same versioned source tree.

For local development or advanced manual installs, you can still run the repository copy directly. Without overrides it behaves the same as the remote installer and pulls from GitHub Releases:

```bash
git clone https://github.com/pruvon/pruvon.git
cd pruvon
sudo ./install.sh
```

To install a locally built binary instead, set `PRUVON_BINARY` explicitly:

```bash
make build
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

- `PRUVON_BINARY` can point to any built binary if you want to install a local artifact instead of a release download.
- `PRUVON_LISTEN` overrides `pruvon.listen` only when the installer creates a new `/etc/pruvon.yml`.
- `PRUVON_VERSION` accepts a tag such as `v0.1.0`; if omitted, the installer resolves the latest release.
- If `/etc/pruvon.yml` already exists, the installer keeps it and does not rotate the admin password.
- Before starting the service, the installer checks whether the configured listen address can be bound by the `pruvon` service user and fails fast if the port is already in use or the address is not permitted.
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
curl -fsSL https://pruvon.dev/uninstall.sh | sudo bash
```

Remove persistent data too:

```bash
curl -fsSL https://pruvon.dev/uninstall.sh | sudo bash -s -- --purge
```

Also remove the `pruvon` system user and group:

```bash
curl -fsSL https://pruvon.dev/uninstall.sh | sudo bash -s -- --purge --remove-user
```

If you already have a local checkout, running `sudo ./uninstall.sh` is equivalent.

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
