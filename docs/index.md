---
layout: home

hero:
  name: Pruvon Docs
  text: Operate Pruvon safely on a Dokku host.
  tagline: Installation, configuration, and secure network exposure guidance.
  actions:
    - theme: brand
      text: Get Started
      link: /install
    - theme: alt
      text: Configuration
      link: /configuration

features:
  - title: Install
    details: Install Pruvon on a Dokku host using the official installer or a local binary.
  - title: Configure
    details: Start from the example YAML config and adjust the bind address, auth, and backup settings.
  - title: Secure
    details: Keep Pruvon private, prefer Tailscale, and only publish it behind a tightly controlled reverse proxy.
---

# Overview

Pruvon is a web UI for Dokku. It is designed to run on the same Linux host as Dokku and expose app, service, backup, log, and terminal operations through a browser.

Start with these pages:

- [Install](/install)
- [Configuration](/configuration)
- [Security](/security)
- [Behind Proxy](/behind-proxy)
