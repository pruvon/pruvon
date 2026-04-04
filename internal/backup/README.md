# Pruvon Backup Utility

The backup utility provides functionality for backing up Dokku databases. It's designed to create daily, weekly, and monthly backups of PostgreSQL, MariaDB, MongoDB, and Redis databases managed by Dokku.

## Configuration

The backup configuration is specified in the `config.yaml` file under the `backup` section:

```yaml
backup:
  backup_dir: "/var/lib/dokku/data/pruvon-backup"  # Directory to store backups
  do_weekly: 7                                     # Day of week for weekly backups (1-7, where 1 is Monday)
  do_monthly: 1                                    # Day of month for monthly backups (1-31)
  db_types:                                        # Database types to backup
    - "postgres"
    - "mariadb"
    - "mongo"                                      # MongoDB support
    - "redis"                                      # Redis support
  keep_daily_days: 7                               # Number of days to keep daily backups
  keep_weekly_num: 6                               # Number of weekly backups to keep
  keep_monthly_num: 3                              # Number of monthly backups to keep
```

## Usage

Run the backup utility using the command-line flag:

```bash
# Run automatic backup (daily, weekly, or monthly based on the current date)
pruvon --backup auto

# Run a specific backup type
pruvon --backup daily
pruvon --backup weekly
pruvon --backup monthly
```

## Automatic Daily Backups

Pruvon does not install cron jobs at runtime. Install the provided cron script during provisioning or package installation so the application does not need permission to write into `/etc/cron.daily`.

To manually install the daily cron job:

```bash
sudo install -m 0755 scripts/cron/pruvon-backup /etc/cron.daily/pruvon-backup
```

The script defaults to `/opt/pruvon/pruvon` for the binary and `/etc/pruvon.yml` for the config file. If your installation uses different paths, update `PRUVON_PATH` and `CONFIG_PATH` in `scripts/cron/pruvon-backup` before installing it.

## Backup Structure

Backups are organized in the following directory structure:

```text
backup_dir/
├── postgres/
│   ├── daily/
│   │   └── database_name/
│   │       └── database_name_2023-04-01_10h30m.Monday.dump.gz
│   ├── weekly/
│   │   └── database_name/
│   │       └── database_name_weekly.14.2023-04-01_10h30m.dump.gz
│   └── monthly/
│       └── database_name/
│           └── database_name_monthly.April.2023-04-01_10h30m.dump.gz
├── mariadb/
│   ├── daily/
│   │   └── ...
│   ├── weekly/
│   │   └── ...
│   └── monthly/
│       └── ...
├── mongo/
│   ├── daily/
│   │   └── database_name/
│   │       └── database_name_2023-04-01_10h30m.Monday.archive.gz
│   ├── weekly/
│   │   └── ...
│   └── monthly/
│       └── ...
└── redis/
    ├── daily/
    │   └── database_name/
    │       └── database_name_2023-04-01_10h30m.Monday.rdb.gz
    ├── weekly/
    │   └── ...
    └── monthly/
        └── ...
```

## Backup Methods

Backups are performed using the relevant Dokku plugin's `export` command:

- **PostgreSQL**: Uses `dokku postgres:export`
- **MariaDB**: Uses `dokku mariadb:export`
- **MongoDB**: Uses `dokku mongo:export`
- **Redis**: Uses `dokku redis:export`

The backup utility will automatically check if the required Dokku plugin for a database type is installed before attempting to back it up. If a plugin is not installed, backups for that database type will be skipped.

## Rotation Policy

- **Daily backups**: Daily backups older than `keep_daily_days` (default: 7 days) are automatically deleted
- **Weekly backups**: Only the last `keep_weekly_num` (default: 6) weekly backups are kept
- **Monthly backups**: Only the last `keep_monthly_num` (default: 3) monthly backups are kept

These values can be configured in the config.yaml file. If not specified, default values will be used.

## Requirements

- Dokku installed and accessible
- The corresponding Dokku plugin installed for each database type you want to back up (e.g., `dokku-postgres`, `dokku-mongo`, `dokku-redis`).
- gzip utility available
- Proper permissions to access Dokku databases and run Dokku commands
- Write access to the backup directory
