# Operations

Pruvon runs as a systemd service named `pruvon`. The installer enables and starts it automatically, so all day-to-day management uses standard systemd tools.

## Service commands

```bash
sudo systemctl status pruvon     # check if running
sudo systemctl start pruvon      # start the service
sudo systemctl stop pruvon       # stop the service
sudo systemctl restart pruvon    # restart after config changes
```

Always restart after editing `/etc/pruvon.yml`.

## Logs

### Service log

Follow the live service output:

```bash
sudo journalctl -u pruvon -f
```

View the most recent entries:

```bash
sudo journalctl -u pruvon -n 100
```

### Application logs

| File | Contents |
| --- | --- |
| `/var/log/pruvon/activity.log` | Actions performed through the Pruvon interface |
| `/var/log/pruvon/backup.log` | Output from the daily backup cron job |

Both files are managed by the installed logrotate policy at `/etc/logrotate.d/pruvon`.

## Applying config changes

1. Edit the config:

   ```bash
   sudoedit /etc/pruvon.yml
   ```

2. Restart and verify:

   ```bash
   sudo systemctl restart pruvon
   sudo systemctl status pruvon
   ```

3. Check for startup errors if something looks wrong:

   ```bash
   sudo journalctl -u pruvon -n 50
   ```

## Backups

### Automatic backups

The installer places a daily cron script at `/etc/cron.daily/pruvon-backup`. It runs once per day and selects the backup type automatically:

- **Monthly** if today matches the configured day of the month
- **Weekly** if today matches the configured day of the week
- **Daily** otherwise

See [Configuration - Backup settings](/configuration#backup-settings) for the schedule and retention options.

### Manual backups

Trigger a backup manually with a specific type:

```bash
sudo pruvon -backup daily -config /etc/pruvon.yml
sudo pruvon -backup weekly -config /etc/pruvon.yml
sudo pruvon -backup monthly -config /etc/pruvon.yml
```

Or let Pruvon choose the type based on the current date:

```bash
sudo pruvon -backup auto -config /etc/pruvon.yml
```

Backup archives are stored in the directory set by `backup.backup_dir` in the config (default: `/var/lib/dokku/data/pruvon-backup`).

Backups can also be triggered and managed from the Pruvon web interface.

## Updating Pruvon

Re-run the installer:

```bash
curl -fsSL https://pruvon.dev/install.sh | sudo bash
```

This updates the binary and supporting files while preserving `/etc/pruvon.yml`.

After an update, verify the service:

```bash
sudo systemctl status pruvon
sudo journalctl -u pruvon -n 50
```

To install a specific version:

```bash
curl -fsSL https://pruvon.dev/install.sh | sudo env PRUVON_VERSION=v0.1.0 bash
```

## Customizing the systemd unit

Inspect the current unit:

```bash
sudo systemctl cat pruvon
```

To add overrides (environment variables, dependencies, resource limits) without editing the unit file directly:

```bash
sudo systemctl edit pruvon
```

This opens a drop-in override file. After saving:

```bash
sudo systemctl daemon-reload
sudo systemctl restart pruvon
```

## Key file locations

| Path | Purpose |
| --- | --- |
| `/etc/pruvon.yml` | Configuration file |
| `/opt/pruvon/pruvon` | Binary |
| `/usr/local/bin/pruvon` | Symlink to the binary |
| `/etc/systemd/system/pruvon.service` | systemd unit |
| `/etc/cron.daily/pruvon-backup` | Daily backup cron script |
| `/var/log/pruvon/` | Log directory |
| `/var/lib/dokku/data/pruvon-backup/` | Backup archive directory |
