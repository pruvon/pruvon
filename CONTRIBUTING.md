# Contributing to Pruvon

Thank you for considering contributing to Pruvon!

## Getting Started

### Prerequisites

- Go 1.22 or later
- Linux server (amd64/arm64) for deployment
- Dokku (optional, for server management features)

### Development Setup

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/pruvon.git
   cd pruvon
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```
4. Build the project:
   ```bash
   make build
   ```

   For Linux `amd64` and `arm64` build artifacts in `builds/`:
   ```bash
   make build-linux
   ```

### Running Tests

```bash
go test ./...
```

For a specific package:
```bash
go test ./internal/backup
```

For race detection:

```bash
go test -race ./...
```

## How Can I Contribute?

### Reporting Bugs

Before submitting a bug report:
- Check the [existing issues](https://github.com/pruvon/pruvon/issues) to avoid duplicates
- Verify the bug is not specific to your environment

When filing a bug report, include:
- Clear description of the issue
- Steps to reproduce
- Expected vs actual behavior
- Go version and OS information

Use the bug report template when opening an issue.

### Suggesting Features

We welcome feature suggestions! Before submitting:
- Check existing issues and pull requests
- Consider whether the feature aligns with the project's goals

Use the feature request template when opening an issue.

### Pull Requests

1. Create a feature branch from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes following Go best practices:
   - Run `gofmt` on your code
   - Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
   - Add tests for new functionality
   - Ensure `go test ./...` passes

3. Commit your changes with clear messages:
   ```bash
   git commit -m "Add feature: your feature description"
   ```

4. Push to your fork and create a Pull Request:
   ```bash
   git push origin feature/your-feature-name
   ```

5. Fill out the pull request template.

## Style Guidelines

### Go Code

- Use `gofmt` for formatting
- Follow the [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) guide
- Write clear, descriptive variable and function names
- Add comments for complex logic

### Commit Messages

- Use clear, descriptive commit messages
- Start with a verb (Add, Fix, Update, Remove)
- Reference issues when applicable
