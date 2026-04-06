# Security

Pruvon is an administrative interface for Dokku. Treat it like root-adjacent infrastructure.

## Strong Recommendation

Do not expose Pruvon directly to the public internet.

Recommended baseline:

- keep `pruvon.listen` on `127.0.0.1:8080`
- access it through a private network overlay such as Tailscale
- if a reverse proxy is required, restrict it to known source IP ranges
- use strong admin credentials or GitHub auth with a tightly controlled allowlist

## Preferred Access Model: Tailscale

Tailscale is the simplest recommended option because it avoids making Pruvon publicly reachable.

Typical approach:

1. Install Tailscale on the Dokku host.
2. Keep Pruvon bound to `127.0.0.1:8080`.
3. Expose it only through a reverse proxy that listens on the Tailscale interface, or by tunneling from your private network.
4. Limit access to specific users or groups in your Tailscale policy.

This keeps the UI off the public internet and reduces attack surface significantly.

## What To Avoid

- binding Pruvon directly to `0.0.0.0:8080`
- publishing it with an open public DNS record
- allowing unrestricted access through a proxy
- keeping the example bcrypt hash in production
- sharing a single weak admin password among multiple operators

## Credential Hygiene

- replace the example admin password hash before first real use
- rotate credentials when operators leave or credentials are shared too widely
- if using GitHub auth, keep `github.users` limited to explicit usernames
- remove access immediately when a user should no longer operate the host

## Network Controls

Even behind a proxy, restrict by source network whenever possible.

Examples of acceptable controls:

- Tailscale-only reachability
- office VPN IP ranges
- jump-host or bastion IPs
- internal RFC1918 networks that are already access-controlled

See [Behind Proxy](/behind-proxy) for an Nginx example that only permits specific IP blocks.
