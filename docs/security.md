# Security

Pruvon is an administrative interface for Dokku. It can manage apps, services, backups, and open terminals on the host. Treat it as privileged infrastructure.

## Recommended defaults

- Bind to `127.0.0.1:8080` (the installer default)
- Access Pruvon through a VPN, overlay network (e.g., Tailscale), or a reverse proxy with IP restrictions
- Use a strong, unique local admin password
- Limit access to the smallest set of local users and scoped permissions you need
- Replace the example admin password hash immediately if the installer did not do it for you

## Keep Pruvon off the public internet

The safest setup:

1. Leave `pruvon.listen` set to `127.0.0.1:8080`.
2. Reach it through a private network: Tailscale, WireGuard, an SSH tunnel, or a reverse proxy that only allows known source IPs.
3. Terminate TLS at the proxy if traffic leaves localhost.

This keeps the entire UI and API surface out of public reach.

See [Behind a Reverse Proxy](/behind-proxy) for an Nginx example with IP allowlists.

## What to avoid

- Binding to `0.0.0.0` without additional network controls
- Publishing Pruvon on a public domain with no source IP restrictions
- Keeping the bundled example password hash in production
- Sharing the local admin password across multiple people
- Leaving disabled or unnecessary local users in the config after they should no longer have access

## Credential practices

### Local admin

- The password in `/etc/pruvon.yml` is stored as a bcrypt hash, never in plain text
- Rotate the password when operators change or if the credential has been shared too widely
- See [Configuration - Change the admin password](/configuration#change-the-admin-password) for the procedure

### Scoped local users

- Keep non-admin users limited to the routes, apps, and services they actually need
- Remove or disable users as soon as they no longer manage the host
- Configured users are revalidated against the config on every request -- removing or disabling a user and restarting the service revokes access immediately

### Applying credential changes

After editing credentials in `/etc/pruvon.yml`:

```bash
sudo systemctl restart pruvon
sudo systemctl status pruvon
```

## After changing the listen address

If you change `pruvon.listen` or modify the proxy configuration, verify the service afterwards:

```bash
sudo systemctl restart pruvon
sudo systemctl status pruvon
sudo journalctl -u pruvon -n 50
```

Test that the UI loads from the expected network path and that unintended sources are blocked.
