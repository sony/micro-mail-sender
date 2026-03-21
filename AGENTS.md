# AGENTS.md

> Instructions for AI coding agents working on this repository.

## Project Overview

MicroMailSender is a lightweight, self-hosted email sending service that
provides a SendGrid-compatible API. It is designed as a simplified alternative
to SendGrid for organizations that need email sending capabilities with full
control over their infrastructure.

Primary language: Go. The service exposes a REST API for sending emails and
querying message status, backed by PostgreSQL for persistence and an SMTP relay
for delivery.

## Project Structure

```
build/local/     - Docker and local development configuration (Dockerfile, schema.sql)
doc/             - API documentation (REST endpoints)
e2e/             - End-to-end tests (run inside Docker Compose)
mailsender/      - Core application package (handlers, config, sender, message, query)
main/            - Application entry point (main.go)
sample/          - Example request payloads for the API
testdata/        - Test fixture data
tools/           - CI and test helper shell scripts
```

## Development Setup

Prerequisites:

- Go 1.25+ (see `.go-version`)
- Docker and Docker Compose (for integration and e2e tests)

Start the local development environment:

```bash
docker compose up -d --wait
```

This starts the application, a PostgreSQL database, and MailHog (for capturing
test emails). The API is available at `http://localhost:8333` and the MailHog
web console at `http://localhost:8025`.

## Build

```bash
go build -v -o mailsender ./main
```

## Test

Unit tests (no external dependencies required):

```bash
go test ./...
```

Integration tests (requires Docker Compose environment running):

```bash
make test
```

End-to-end tests (requires Docker Compose environment running):

```bash
make e2etest
```

Test files follow Go conventions: `*_test.go` alongside source files. Integration
tests use the `integration` build tag and e2e tests use the `e2e` build tag.

## Lint and Format

```bash
go vet ./...
golangci-lint run ./...
```

The project uses `golangci-lint` with a configuration in `.golangci.yml` that
excludes unused-variable checks for `mailsender/test_utils`.

Formatting is enforced via `gofmt` and `goimports`.

## Code Style

- Follow standard Go conventions and idioms
- Use `gofmt` and `goimports` for formatting
- Error handling uses `github.com/cockroachdb/errors` for wrapped errors
- Logging uses `go.uber.org/zap`
- HTTP routing uses `github.com/gorilla/mux`
- Test assertions use `github.com/stretchr/testify`

## CI/CD

- **GitHub Actions** (`.github/workflows/ci.yml`): Runs `golangci-lint` and
  builds the application on pushes and PRs to `main`.
- **GitHub Actions** (`.github/workflows/go-test.yml`): Runs unit tests and
  Docker-based integration and e2e tests on pushes and PRs to `main`.
## Important Notes

- Integration and e2e tests require a running Docker Compose environment
  (`docker compose up -d --wait`) with PostgreSQL and MailHog services.
- The `make test` and `make e2etest` commands execute tests inside the Docker
  container, not on the host directly.
- Test utility files (`mailsender/test_utils.go`) are intentionally excluded
  from unused-variable lint checks.
- API authentication requires a `Bearer` token in the `Authorization` header,
  configured via the `MAILSENDER_CONFIG` environment variable.
- The project is licensed under the MIT License.
