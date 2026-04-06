# Behind Proxy

The safest proxy pattern is:

- keep Pruvon bound to `127.0.0.1:8080`
- terminate HTTP or HTTPS in Nginx
- only allow known private or VPN source ranges
- deny everything else

## Recommended Pruvon Bind Address

```yaml
pruvon:
  listen: 127.0.0.1:8080
```

## Example Nginx Configuration

This example only permits traffic from explicitly chosen office and VPN ranges.

```nginx
server {
    listen 443 ssl http2;
    server_name pruvon.example.internal;

    ssl_certificate /etc/letsencrypt/live/pruvon.example.internal/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/pruvon.example.internal/privkey.pem;

    allow 100.64.12.0/24;
    allow 10.20.30.0/24;
    allow 203.0.113.10;
    deny all;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 3600;
        proxy_send_timeout 3600;
    }
}
```

## Notes

- replace `100.64.12.0/24`, `10.20.30.0/24`, and `203.0.113.10` with your real operator ranges
- `proxy_read_timeout` and `proxy_send_timeout` help with long-running terminal and log sessions
- `Upgrade` and `Connection` headers are required for WebSocket-backed features

## Narrower Example

If you want to allow only a single office subnet and one VPN exit node:

```nginx
allow 10.20.30.0/24;
allow 100.90.10.5;
deny all;
```

Use the smallest allowlist that still supports your operators.
