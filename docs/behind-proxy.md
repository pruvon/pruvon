# Behind a Reverse Proxy

The recommended setup is to keep Pruvon on localhost and proxy requests through Nginx with IP-based access controls.

## Pruvon listen address

Make sure Pruvon is bound to localhost:

```yaml
pruvon:
  listen: 127.0.0.1:8080
```

If you change this value, restart the service:

```bash
sudo systemctl restart pruvon
```

## Nginx configuration

This example terminates HTTPS and restricts access to specific IP ranges:

```nginx
server {
    listen 443 ssl http2;
    server_name pruvon.example.internal;

    ssl_certificate     /etc/letsencrypt/live/pruvon.example.internal/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/pruvon.example.internal/privkey.pem;

    # Replace these with your actual operator IP ranges
    allow 100.64.12.0/24;   # Tailscale / VPN range
    allow 10.20.30.0/24;    # Office network
    allow 203.0.113.10;     # Individual operator IP
    deny all;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;

        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Required for WebSocket features (terminals, live logs)
        proxy_set_header Upgrade    $http_upgrade;
        proxy_set_header Connection "upgrade";

        # Long timeouts for terminal and log streaming sessions
        proxy_read_timeout 3600;
        proxy_send_timeout 3600;
    }
}
```

## Applying changes

Validate and reload Nginx:

```bash
sudo nginx -t && sudo systemctl reload nginx
```

If you also changed `pruvon.listen`, restart Pruvon:

```bash
sudo systemctl restart pruvon
```

## Important notes

- **WebSocket headers are required.** The `Upgrade` and `Connection` headers enable terminal access, live log streaming, and import progress tracking. Without them, these features will not work.
- **Timeouts matter.** Terminal and log sessions can run for extended periods. The `proxy_read_timeout` and `proxy_send_timeout` values prevent Nginx from closing idle connections prematurely.
- **Use the smallest allowlist possible.** Only include IP ranges that your operators actually connect from.

## Minimal allowlist example

A single office subnet and one VPN exit node:

```nginx
allow 10.20.30.0/24;
allow 100.90.10.5;
deny all;
```

## HTTP redirect (optional)

Redirect HTTP to HTTPS if you also listen on port 80:

```nginx
server {
    listen 80;
    server_name pruvon.example.internal;
    return 301 https://$host$request_uri;
}
```
