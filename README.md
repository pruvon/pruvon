# Pruvon

[![CI](https://github.com/pruvon/pruvon/actions/workflows/ci.yml/badge.svg)](https://github.com/pruvon/pruvon/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/pruvon/pruvon)](https://github.com/pruvon/pruvon)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL%203.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/pruvon/pruvon)](https://github.com/pruvon/pruvon/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/pruvon/pruvon)](https://goreportcard.com/report/github.com/pruvon/pruvon)
[![CodeQL](https://github.com/pruvon/pruvon/actions/workflows/codeql.yml/badge.svg)](https://github.com/pruvon/pruvon/actions/workflows/codeql.yml)

Pruvon is a web UI for Dokku. It runs on the Dokku host itself and gives you a browser-based interface for app operations, service management, backups, logs, Docker visibility, and terminal access.

It is not a general-purpose server management panel. Pruvon assumes Dokku, Linux paths, and a service-style deployment on the target host.

> [!WARNING]
> Pruvon is under active development and should be considered early-stage software.
> If you choose to run it in environments where breakage, rough edges, or incomplete features are a problem, that risk is yours to take.

## Features

- **Dokku Web UI**: Manage apps, services, and operational workflows from the browser
- **Database Backups**: Run and retain PostgreSQL, MariaDB, MongoDB, and Redis backups through Dokku plugins
- **Logs and Terminal Access**: Use browser-based logs and terminal sessions over WebSockets
- **Docker Visibility**: Inspect containers, images, and host resource usage
- **Operational Tools**: Manage SSH keys, templates, users, and related Dokku tasks

## Requirements

- A Linux host with Dokku already installed
- `sudo` or root access on that host
- `systemd`, `nginx`, and Dokku available on the host

## Install On A Dokku Host

The supported production path is the official installer. It does more than copy a binary: it lays out the service, config, logs, cron, sudoers policy, and systemd unit for you.

Install the latest release:

```bash
curl -fsSL https://pruvon.dev/install.sh | sudo bash
```

If `curl` is not available:

```bash
wget -qO- https://pruvon.dev/install.sh | sudo bash
```

Install a specific release:

```bash
curl -fsSL https://pruvon.dev/install.sh | sudo env PRUVON_VERSION=v0.1.0 bash
```

Create a fresh config with a custom listen address:

```bash
curl -fsSL https://pruvon.dev/install.sh | sudo env PRUVON_LISTEN=127.0.0.1:9090 bash
```

`PRUVON_LISTEN` only applies when the installer creates a new config file. If `/etc/pruvon.yml` already exists, the installer keeps the existing listen address.

Install or update from a local checkout:

```bash
git clone https://github.com/pruvon/pruvon.git
cd pruvon
sudo ./install.sh
```

Install a locally built binary instead of downloading a GitHub release:

```bash
make build
sudo env PRUVON_BINARY=builds/pruvon-linux-amd64 ./install.sh
```

`install.sh` will:

- download the matching Linux release archive from GitHub Releases unless `PRUVON_BINARY` is provided
- verify release archives against `checksums.txt`
- fetch the matching `pruvon.yml.example` and `scripts/cron/pruvon-backup` from the same tagged source tree unless overrides are provided
- install the binary to `/opt/pruvon/pruvon`
- create `/usr/local/bin/pruvon` as a symlink
- create the `pruvon` service user and required group memberships
- create `/var/lib/pruvon`, `/var/log/pruvon`, and `/var/lib/dokku/data/pruvon-backup`
- create `/etc/pruvon.yml` if it does not exist
- generate a random local admin password for a fresh install and store only its bcrypt hash in config
- rotate the bundled example admin password hash if an existing config still contains it
- install the systemd unit, sudoers policy, daily backup cron script, and logrotate policy
- enable and start the `pruvon` systemd service

Any installer-generated admin password is printed once at the end of installation.

## Installed Layout

| Path | Purpose |
| --- | --- |
| `/etc/pruvon.yml` | Main configuration file |
| `/opt/pruvon/pruvon` | Installed binary |
| `/usr/local/bin/pruvon` | Convenience symlink to the installed binary |
| `/etc/systemd/system/pruvon.service` | systemd unit |
| `/etc/sudoers.d/pruvon` | Commands the service account may run with `sudo` |
| `/etc/cron.daily/pruvon-backup` | Daily backup trigger |
| `/etc/logrotate.d/pruvon` | Log rotation policy |
| `/var/log/pruvon/activity.log` | Activity log |
| `/var/log/pruvon/backup.log` | Backup job log |
| `/var/lib/dokku/data/pruvon-backup` | Backup archive storage |

## Operate The Installed Service

`install.sh` already enables and starts `pruvon`. After that, normal service operations are done with `systemctl` and `journalctl`.

Common commands:

```bash
sudo systemctl status pruvon
sudo systemctl start pruvon
sudo systemctl stop pruvon
sudo systemctl restart pruvon
sudo systemctl enable pruvon
sudo systemctl disable pruvon
sudo journalctl -u pruvon -f
```

Inspect the installed unit or create an override safely:

```bash
sudo systemctl cat pruvon
sudo systemctl edit pruvon
sudo systemctl daemon-reload
sudo systemctl restart pruvon
```

If you edit `/etc/systemd/system/pruvon.service` directly or create an override with `systemctl edit`, run `daemon-reload` before restarting the service.

## Configure Pruvon

On an installed system, the main config file lives at `/etc/pruvon.yml`.

Top-level sections:

- `admin`: Local fallback login. `admin.password` must be a bcrypt hash.
- `github`: Optional GitHub OAuth settings and allowed GitHub users.
- `pruvon`: Runtime settings such as the bind address.
- `backup`: Backup storage, schedule, database types, and retention.
- `dokku`: Reserved for future Dokku-specific settings. Leave it as `{}`.
- `server`: Legacy or reserved section. Leave it as `null` unless you know you need it.

After editing `/etc/pruvon.yml`, restart Pruvon:

```bash
sudo systemctl restart pruvon
sudo systemctl status pruvon
```

## Change The Local Admin Password

If you want to rotate the local admin password, generate a new bcrypt hash and replace `admin.password` in `/etc/pruvon.yml`.

Generate a hash interactively:

```bash
NEW_HASH="$(htpasswd -nBC 10 '' | tr -d ':\n')"
printf '%s\n' "$NEW_HASH"
```

Edit the config:

```bash
sudoedit /etc/pruvon.yml
```

Example:

```yaml
admin:
  username: admin
  password: "$2a$10$...your-new-hash..."
```

Apply the change:

```bash
sudo systemctl restart pruvon
```

Avoid `htpasswd -b ...`; it exposes the plain-text password in shell history and process listings.

## Backup Behavior

The standard install creates one daily cron job at `/etc/cron.daily/pruvon-backup`. That job runs:

```bash
pruvon -backup auto -config /etc/pruvon.yml
```

`-backup auto` does not create separate daily, weekly, and monthly timers. Instead, each daily run chooses exactly one rotation type:

- monthly if today matches `backup.do_monthly`
- otherwise weekly if today matches `backup.do_weekly`
- otherwise daily

Key settings in `/etc/pruvon.yml`:

- `backup.backup_dir`: Where backup archives are written
- `backup.db_types`: Which Dokku service types are included
- `backup.do_weekly`: Weekly rotation day. Use `1-6` for Monday-Saturday, and `0` or `7` for Sunday.
- `backup.do_monthly`: Day of month for monthly rotation
- `backup.keep_daily_days`: How many days of daily backups to keep
- `backup.keep_weekly_num`: How many weekly backups to keep
- `backup.keep_monthly_num`: How many monthly backups to keep

Run backups manually when needed:

```bash
sudo pruvon -backup auto -config /etc/pruvon.yml
sudo pruvon -backup daily -config /etc/pruvon.yml
sudo pruvon -backup weekly -config /etc/pruvon.yml
sudo pruvon -backup monthly -config /etc/pruvon.yml
```

## Local Development

The repository copy is still useful for local development and manual testing, but it is not the recommended production path.

Start from the example config when you are running from source:

```bash
cp pruvon.yml.example pruvon.yml
go run ./cmd/app -server -config pruvon.yml
```

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

## Local GitHub Release Flow

Create a release locally:

```bash
make release VERSION=v0.1.1 PREVIOUS_TAG=v0.1.0
```

This command will:

- regenerate `CHANGELOG.md` for the target version
- create a release commit such as `release: v0.1.1`
- build versioned Linux release archives in `dist/`
- create and push the git tag
- create the GitHub release and upload the archives plus `checksums.txt`

Release prerequisites:

- a clean git working tree
- `gh auth login` completed for the target GitHub account
- permission to push the current branch and the new tag to `origin`

If you want custom GitHub release notes instead of generated changelog text, pass a file path:

```bash
make release VERSION=v0.1.1 NOTES_FILE=/absolute/path/to/release-notes.md
```

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

If you already have a local checkout, `sudo ./uninstall.sh` is equivalent.

Default uninstall keeps `/etc/pruvon.yml`, `/var/log/pruvon`, and `/var/lib/dokku/data/pruvon-backup` so an accidental removal does not destroy operational data.

## Changelog Workflow

Release notes are generated from commit subjects.

Generate a changelog entry for a release:

```bash
make changelog VERSION=0.1.1 PREVIOUS_TAG=v0.1.0
```

If `PREVIOUS_TAG` is omitted, the generator uses the latest `v*` tag it can find.

Best results come from commit subjects with a conventional prefix such as:

- `feat: ...`
- `fix: ...`
- `docs: ...`
- `refactor: ...`
- `test: ...`
- `chore: ...`

These prefixes are grouped into changelog sections like `Added`, `Fixed`, `Changed`, `Documentation`, `Tests`, and `Maintenance`.

## Documentation

- [Documentation](https://docs.pruvon.dev)
- [Installation Guide](https://docs.pruvon.dev/install)
- [Configuration Guide](https://docs.pruvon.dev/configuration)
- [Operations Guide](https://docs.pruvon.dev/operations)
- [Security Guide](https://docs.pruvon.dev/security)
- [CHANGELOG.md](CHANGELOG.md)

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Security

See [SECURITY.md](SECURITY.md) for vulnerability reporting.

## License

Copyright (c) 2026 Pruvon. Licensed under [AGPL-3.0](LICENSE).
