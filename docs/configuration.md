# Configuration

Pruvon uses a YAML configuration file. In production the default path is `/etc/pruvon.yml`.

Start from the example file:

```bash
cp pruvon.yml.example pruvon.yml
go run ./cmd/app -server -config pruvon.yml
```

After changing `/etc/pruvon.yml` on an installed system, restart the service:

```bash
sudo systemctl restart pruvon
```

## Example Configuration

```yaml
admin:
  username: admin
  password: "$2a$10$Pm8hoUAYMIgL9PWb..KzOeveml0.48arbqds4Qr.r7B38IjJjPQNa"

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

## Admin Credentials

`admin.username` is the local login name.

`admin.password` must be a bcrypt hash, not a plain-text password. Replace the example value before real use.

Example hash generation with `htpasswd`:

```bash
htpasswd -bnBC 10 "" "replace-this-password" | tr -d ':\n'
```

## Reset A Forgotten Admin Password

1. Generate a new bcrypt hash:

```bash
NEW_HASH="$(htpasswd -bnBC 10 '' 'replace-this-password' | tr -d ':\n')"
printf '%s\n' "$NEW_HASH"
```

2. Open `/etc/pruvon.yml` and replace `admin.password` with the generated hash.

Example:

```yaml
admin:
  username: admin
  password: "$2a$10$...your-new-hash..."
```

3. Restart the service:

```bash
sudo systemctl restart pruvon
```

## GitHub Authentication

If you want GitHub login, set these values:

```yaml
github:
  client_id: "your-github-oauth-client-id"
  client_secret: "your-github-oauth-client-secret"
  users:
    - "alice"
    - "bob"
```

Only the GitHub usernames listed in `github.users` are allowed to log in.

## Listen Address

Recommended default:

```yaml
pruvon:
  listen: 127.0.0.1:8080
```

Bind to localhost and put Pruvon behind a private-access proxy or VPN. Do not bind directly to a public interface unless you fully understand the risk and have additional network controls in place.

## Backup Settings

Example with weekly and monthly retention:

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

Notes:

- `backup_dir` should remain on persistent storage
- `db_types` controls which Dokku service types are included
- `do_weekly` and `do_monthly` enable additional backup schedules
- retention fields control how many old backups are kept

## Minimal Production Example

```yaml
admin:
  username: admin
  password: "$2a$10$replace-with-your-own-bcrypt-hash"

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

Read [Security](/security) before making the UI reachable from anywhere except localhost.
