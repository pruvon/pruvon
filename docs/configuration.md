# Configuration

Pruvon reads its configuration from `/etc/pruvon.yml`. The installer creates this file on first install with a randomly generated admin password.

After any change to the config file, restart the service:

```bash
sudo systemctl restart pruvon
```

## Config file sections

| Section | Purpose |
| --- | --- |
| `admin` | Local admin login credentials |
| `github` | GitHub OAuth settings and authorized users |
| `pruvon` | Runtime settings (listen address) |
| `backup` | Backup schedule, included database types, and retention policy |
| `dokku` | Reserved for future use |
| `server` | Reserved |

## Full example

```yaml
admin:
  username: admin
  password: "$2a$10$...your-bcrypt-hash..."

github:
  client_id: ""
  client_secret: ""
  users: []

pruvon:
  listen: 127.0.0.1:8080

dokku: {}

server: null

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

## Admin login

`admin.username` is the local admin username. Default is `admin`.

`admin.password` must be a bcrypt hash, not a plain-text password.

### Change the admin password

Generate a new bcrypt hash:

```bash
NEW_HASH="$(htpasswd -nBC 10 '' | tr -d ':\n')"
printf '%s\n' "$NEW_HASH"
```

Open the config file:

```bash
sudoedit /etc/pruvon.yml
```

Replace the value under `admin.password` with the new hash, then restart:

```bash
sudo systemctl restart pruvon
```

::: tip
Avoid `htpasswd -b` -- it exposes the plain-text password in shell history and process listings.
:::

### Reset a forgotten password

If you can no longer log in:

1. Generate a new bcrypt hash with the command above.
2. Replace `admin.password` in `/etc/pruvon.yml` with the new hash.
3. Restart the service with `sudo systemctl restart pruvon`.

## GitHub authentication

GitHub login is optional. To enable it, register a GitHub OAuth App and add the credentials to the config.

```yaml
github:
  client_id: "your-github-oauth-client-id"
  client_secret: "your-github-oauth-client-secret"
  users: []
```

### Managing GitHub users

`github.users` is a list of user objects with granular access controls:

| Field | Type | Purpose |
| --- | --- | --- |
| `username` | string | GitHub username (required) |
| `routes` | string list | Allowed URL route patterns |
| `apps` | string list | Allowed Dokku app names |
| `services` | map of string lists | Allowed services, grouped by type |

Example with full access:

```yaml
github:
  client_id: "your-github-oauth-client-id"
  client_secret: "your-github-oauth-client-secret"
  users:
    - username: "alice"
      routes:
        - "/*"
      apps:
        - "*"
      services:
        postgres:
          - "*"
        redis:
          - "*"
```

For most setups, the simpler path is to sign in as the local admin, enable GitHub OAuth in the Pruvon UI, and manage GitHub user permissions from there.

### Access enforcement

GitHub users are revalidated against the config on every request. Removing a user from `github.users` and restarting the service revokes their access immediately.

## Listen address

```yaml
pruvon:
  listen: 127.0.0.1:8080
```

The default binds Pruvon to localhost only. This is the recommended setting -- reach it through a VPN, overlay network, or reverse proxy instead of binding to a public interface.

If you change the listen address, restart the service:

```bash
sudo systemctl restart pruvon
```

See [Security](/security) before binding to anything other than `127.0.0.1`.

## Backup settings

A daily cron job at `/etc/cron.daily/pruvon-backup` triggers automatic backups by running:

```
pruvon -backup auto -config /etc/pruvon.yml
```

Each run produces exactly one backup type based on the current date:

1. **Monthly** -- if today's day-of-month matches `do_monthly`
2. **Weekly** -- otherwise, if today's day-of-week matches `do_weekly`
3. **Daily** -- otherwise

### Backup fields

| Field | Meaning |
| --- | --- |
| `backup_dir` | Directory where backup archives are stored |
| `do_weekly` | Day of week for weekly backups: `1`-`6` for Monday-Saturday, `0` or `7` for Sunday |
| `do_monthly` | Day of month for monthly backups (e.g., `1` for the first) |
| `db_types` | Dokku service types to back up (e.g., `postgres`, `mariadb`, `mongo`, `redis`) |
| `keep_daily_days` | Number of days to retain daily backups |
| `keep_weekly_num` | Number of weekly backups to retain |
| `keep_monthly_num` | Number of monthly backups to retain |

### Example

```yaml
backup:
  backup_dir: "/var/lib/dokku/data/pruvon-backup"
  do_weekly: 1
  do_monthly: 1
  db_types:
    - "postgres"
    - "redis"
  keep_daily_days: 7
  keep_weekly_num: 8
  keep_monthly_num: 6
```

This configuration:

- Stores backup archives under `/var/lib/dokku/data/pruvon-backup`
- Backs up only PostgreSQL and Redis services
- Creates the weekly backup on Mondays
- Creates the monthly backup on the first of the month
- Retains 7 daily, 8 weekly, and 6 monthly backups

Backups can also be managed and triggered through the Pruvon web interface. See [Operations](/operations) for manual backup commands.

## Editing the config file

Always use `sudoedit` to edit the config:

```bash
sudoedit /etc/pruvon.yml
```

After saving, restart and verify:

```bash
sudo systemctl restart pruvon
sudo systemctl status pruvon
```

Read [Security](/security) before making Pruvon reachable from outside localhost.
