# Installation

Pruvon runs as a systemd service on the same Linux host as Dokku. The installer handles the full setup: downloading the binary, creating the service account, writing the systemd unit, and enabling automatic backups.

## Requirements

- A Linux host with Dokku already installed
- Root or sudo access
- systemd and Nginx available on the host

## Install

Install the latest release:

```bash
curl -fsSL https://pruvon.dev/install.sh | sudo bash
```

If `curl` is not available:

```bash
wget -qO- https://pruvon.dev/install.sh | sudo bash
```

### Install a specific version

```bash
curl -fsSL https://pruvon.dev/install.sh | sudo env PRUVON_VERSION=v0.1.0 bash
```

### Set a custom listen address

On a fresh install, you can set the initial listen address:

```bash
curl -fsSL https://pruvon.dev/install.sh | sudo env PRUVON_LISTEN=127.0.0.1:9090 bash
```

`PRUVON_LISTEN` only takes effect when the installer is creating `/etc/pruvon.yml` for the first time. On an existing installation, edit the config file directly instead. See [Configuration](/configuration).

## What the installer does

1. Downloads the release binary from GitHub and verifies it against the published checksums
2. Creates the `pruvon` system user and adds it to the necessary groups
3. Installs the binary to `/opt/pruvon/pruvon` with a symlink at `/usr/local/bin/pruvon`
4. Creates the runtime, log, and backup directories
5. Writes `/etc/pruvon.yml` with a randomly generated admin password (first install only)
6. Installs the systemd unit, sudoers policy, daily backup cron job, and logrotate config
7. Enables and starts the `pruvon` service

The generated admin password is printed once at the end of the install output. Save it before the terminal scrolls past.

## First login

After a fresh install:

- **Username:** `admin`
- **Password:** the random password printed by the installer

Pruvon stores only the bcrypt hash in `/etc/pruvon.yml`. The plain-text password is never written to disk.

Change the password after first login. See [Configuration - Admin Login](/configuration#admin-login).

## Installed file layout

| Path | Purpose |
| --- | --- |
| `/etc/pruvon.yml` | Configuration file |
| `/opt/pruvon/pruvon` | Binary |
| `/usr/local/bin/pruvon` | Symlink to the binary |
| `/etc/systemd/system/pruvon.service` | systemd unit |
| `/etc/sudoers.d/pruvon` | Sudoers policy for the service account |
| `/etc/cron.daily/pruvon-backup` | Daily backup trigger |
| `/etc/logrotate.d/pruvon` | Log rotation policy |
| `/var/log/pruvon/` | Activity and backup log directory |
| `/var/lib/dokku/data/pruvon-backup/` | Backup archive storage |

## Update

Re-run the installer. It detects an existing installation and updates the binary and supporting files in place:

```bash
curl -fsSL https://pruvon.dev/install.sh | sudo bash
```

Your `/etc/pruvon.yml` is preserved. The only exception: if the config still contains the bundled example admin password hash, the installer replaces it with a new random password and prints it.

After updating, confirm the service is running:

```bash
sudo systemctl status pruvon
```

## Verify the installation

Check that the service started:

```bash
sudo systemctl status pruvon
```

Follow the service log to watch for errors:

```bash
sudo journalctl -u pruvon -f
```

## Next steps

1. [Configuration](/configuration) -- understand and customize `/etc/pruvon.yml`
2. [Operations](/operations) -- service management, logs, and backup commands
3. [Security](/security) -- lock down access before exposing Pruvon beyond localhost
