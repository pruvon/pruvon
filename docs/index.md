---
layout: home

hero:
  name: Pruvon
  text: A web interface for Dokku.
  tagline: Manage apps, services, backups, logs, and terminals from your browser.
  actions:
    - theme: brand
      text: Get Started
      link: /install
    - theme: alt
      text: Configuration
      link: /configuration

features:
  - title: Install
    details: One command sets up the binary, systemd service, backup schedule, and log rotation on your Dokku host.
  - title: Configure
    details: A single YAML file at /etc/pruvon.yml controls local users, listen address, scoped access, and backup policy.
  - title: Operate
    details: Manage the service with systemctl, read logs with journalctl, and trigger backups on demand.
  - title: Secure
    details: Keep Pruvon on localhost behind a VPN or reverse proxy. Control access with local credentials and scoped local users.
---

## What is Pruvon?

Pruvon is a web UI that runs alongside Dokku on a Linux host. It gives you browser-based access to app management, linked services, database backups, live logs, Docker resource views, and interactive terminals.

It is designed for operators who manage one or more Dokku hosts and want a visual interface without giving up direct SSH access.

## Documentation

Follow these pages in order if you are setting up Pruvon for the first time:

| Page | What it covers |
| --- | --- |
| [Install](/install) | Running the installer, first login, and what gets placed on the system |
| [Configuration](/configuration) | Editing `/etc/pruvon.yml`: users, scoped access, listen address, and backup settings |
| [Operations](/operations) | Starting, stopping, and updating Pruvon; reading logs; running backups |
| [Security](/security) | Recommended access controls and credential practices |
| [Behind a Reverse Proxy](/behind-proxy) | Nginx configuration for proxying Pruvon with IP restrictions |
