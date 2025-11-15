# Repository Guidelines

This repo contains a Go-based MCP server that exposes tools to the Lightpanda Browser via CDP. Use this guide to navigate structure, build and test locally, and contribute changes confidently.

## Project Structure & Module Organization
- `main.go`: Entrypoint and transport wiring (`stdio`, `sse`).
- `api.go`, `session.go`, `download.go`, `std.go`: Core server features and CLI commands.
- `mcp/`: MCP-specific types and tools (`mcp.go`, `tool.go`).
- `rpc/`: RPC protocol helpers.
- `Makefile`: Deployment and secrets helpers (Fly.io, 1Password).
- `README.md`: User-facing usage and install notes.

## Build, Test, and Development Commands
- `go build ./...`: Build the binary locally (outputs `gomcp`).
- `go run . <transport>`: Run in-place; e.g., `go run . stdio` or `go run . sse`.
- `./gomcp download`: Fetch Lightpanda browser binary on first run.
- `./gomcp -cdp ws://127.0.0.1:9222 stdio`: Connect to a remote CDP endpoint.
- `make help`: List Make targets (deploy/secrets convenience).
- `make deploy`: Deploy via Fly.io (requires configured `flyctl`).

## Coding Style & Naming Conventions
- Language: Go 1.x; use `go fmt`/`gofmt` and idiomatic Go.
- Indentation: tabs (Go default). Keep lines simple and readable.
- Names: exported types/functions use `CamelCase`; files `snake_case.go` when logical; package names short, lower-case.
- Error handling: return wrapped errors with context; avoid panics in library code.

## Testing Guidelines
- Framework: standard `testing` package. Place tests alongside sources as `*_test.go`.
- Run tests: `go test ./...` (add table-driven tests where helpful).
- Coverage: aim for meaningful coverage of public behaviors and edge cases.

## Commit & Pull Request Guidelines
- Commits: present tense, concise scope; prefix optional type when clear (e.g., `fix:`, `feat:`, `ci:`) as seen in history.
- PRs: include purpose, linked issues, and a brief test plan; add screenshots for user-facing changes.
- CI/Deploy: ensure the app builds locally; coordinate Fly deployment via `make deploy` if relevant.

## Security & Configuration Tips
- Secrets: never commit credentials. Use 1Password + `make secrets` to set Fly secrets.
- Config paths: Lightpanda browser stored under `$XDG_CONFIG_HOME/lightpanda-gomcp` (see README for OS specifics).

## Agent-Specific Notes
- Transports: support `stdio` (Claude Desktop) and `sse`. Favor `stdio` during local dev.
- Tools: add new MCP tools under `mcp/`, define input schemas in `tool.go`, and register in the server init path.

