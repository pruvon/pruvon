# Pruvon

[![CI](https://github.com/pruvon/pruvon/actions/workflows/ci.yml/badge.svg)](https://github.com/pruvon/pruvon/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/pruvon/pruvon)](https://github.com/pruvon/pruvon)
[![License: AGPL-3.0](https://img.shields.io/github/license/pruvon/pruvon)](LICENSE)
[![Release](https://img.shields.io/github/v/release/pruvon/pruvon)](https://github.com/pruvon/pruvon/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/pruvon/pruvon)](https://goreportcard.com/report/github.com/pruvon/pruvon)
[![CodeQL](https://github.com/pruvon/pruvon/actions/workflows/codeql.yml/badge.svg)](https://github.com/pruvon/pruvon/actions/workflows/codeql.yml)

Pruvon is a self-hosted server management platform for Dokku-based applications. It provides a web interface for managing containers, databases, backups, system monitoring, and real-time terminal access.

## Features

- **Dokku Management** - Deploy, scale, and manage Dokku applications
- **Database Backups** - Automated backups for PostgreSQL, MariaDB, MongoDB, and Redis
- **Real-time Terminal** - Browser-based terminal with WebSocket support
- **System Monitoring** - CPU, memory, disk, and network statistics
- **Docker Management** - Container and image management
- **SSH Key Management** - Secure key storage and deployment
- **Web UI** - Modern Alpine.js frontend with Fiber web framework

## Requirements

- Go 1.22+
- Dokku (for server management features)
- Linux server (amd64/arm64)

## Quick Start

### Build from Source

```bash
git clone https://github.com/pruvon/pruvon.git
cd pruvon
make build
```

The build artifacts are written to `builds/` for Linux `amd64` and `arm64`.

### Run in Server Mode

```bash
./pruvon -server -config config.yaml
```

### Run Backup

```bash
./pruvon -backup auto
./pruvon -backup daily
./pruvon -backup weekly
./pruvon -backup monthly
```

## Configuration

Start from the provided example and adjust it for your environment:

```bash
cp config.yaml.example config.yaml
./pruvon -server -config config.yaml
```

The default production config path is `/etc/pruvon.yml`.

## Documentation

- [Makefile](Makefile) - Build and deployment commands
- [CHANGELOG.md](CHANGELOG.md) - Release notes

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Security

Please see [SECURITY.md](SECURITY.md) for reporting security vulnerabilities.

## License

Copyright (c) 2026 Pruvon. Licensed under [AGPL-3.0](LICENSE).
