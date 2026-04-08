# Pruvon Backup Utility

Pruvon can create Dokku database backups for PostgreSQL, MariaDB, MongoDB, and Redis services.

On a standard installation, `install.sh` also installs `/etc/cron.daily/pruvon-backup`, so backups run as part of the host's normal daily cron flow instead of being created dynamically by the application.

## How Scheduling Works

The installed cron script runs this once per day:

```bash
pruvon -backup auto -config /etc/pruvon.yml
```

`-backup auto` chooses exactly one rotation type for that day:

- monthly if today matches `backup.do_monthly`
- otherwise weekly if today matches `backup.do_weekly`
- otherwise daily

That means `do_weekly` and `do_monthly` do not create extra cron entries. They tell the daily automatic run when to write weekly or monthly rotations.

## Configuration

Backup settings live under `backup` in `pruvon.yml` or `/etc/pruvon.yml`:

```yaml
backup:
  backup_dir: "/var/lib/dokku/data/pruvon-backup"
  do_weekly: 0
  do_monthly: 1
  db_types:
    - "postgres"
    - "mariadb"
    - "mongo"
    - "redis"
  keep_daily_days: 7
  keep_weekly_num: 6
  keep_monthly_num: 3
```

## Field Reference

| Field | Meaning |
| --- | --- |
| `backup_dir` | Where archives are written |
| `do_weekly` | Weekly rotation day. Use `1-6` for Monday-Saturday, and `0` or `7` for Sunday. |
| `do_monthly` | Day of month for monthly rotation |
| `db_types` | Dokku service types to include |
| `keep_daily_days` | How many days of daily backups to keep |
| `keep_weekly_num` | How many weekly backups to keep |
| `keep_monthly_num` | How many monthly backups to keep |

## Installed Paths

On a standard install:

- binary: `/opt/pruvon/pruvon`
- config: `/etc/pruvon.yml`
- daily cron trigger: `/etc/cron.daily/pruvon-backup`
- backup log: `/var/log/pruvon/backup.log`
- backup storage: `/var/lib/dokku/data/pruvon-backup`

If your installation uses different paths, adjust `PRUVON_PATH`, `CONFIG_PATH`, and `LOG_FILE` in `scripts/cron/pruvon-backup` before installing it manually.

## Manual Usage

Run the automatic selection logic:

```bash
pruvon -backup auto -config /etc/pruvon.yml
```

Run a specific rotation explicitly:

```bash
pruvon -backup daily -config /etc/pruvon.yml
pruvon -backup weekly -config /etc/pruvon.yml
pruvon -backup monthly -config /etc/pruvon.yml
```

## Rotation And Retention

- daily backups are rotated by day and retained for `keep_daily_days`
- weekly backups keep the latest `keep_weekly_num` rotations
- monthly backups keep the latest `keep_monthly_num` rotations

## Backup Structure

Backups are organized under the configured `backup_dir` by database type and rotation:

```text
backup_dir/
├── postgres/
│   ├── daily/
│   ├── weekly/
│   └── monthly/
├── mariadb/
│   ├── daily/
│   ├── weekly/
│   └── monthly/
├── mongo/
│   ├── daily/
│   ├── weekly/
│   └── monthly/
└── redis/
    ├── daily/
    ├── weekly/
    └── monthly/
```

Each database gets its own subdirectory inside the selected rotation.

## Backup Methods

Backups are performed through the relevant Dokku plugin export command:

- PostgreSQL: `dokku postgres:export`
- MariaDB: `dokku mariadb:export`
- MongoDB: `dokku mongo:export`
- Redis: `dokku redis:export`

If a required plugin is not installed, that database type is skipped.

## Requirements

- Dokku installed and accessible
- the corresponding Dokku plugin installed for each database type you want to back up
- `gzip` available on the host
- permission to access Dokku and write to the backup directory
