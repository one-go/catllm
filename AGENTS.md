# Repository Guidelines

## Project Structure & Module Organization
- Root contains `README.md`, `LICENSE`, `go.mod`, and `main.go`.
- `docs/` stores architecture and design documentation (see `docs/architecture.md`).
- `internal/config/`, `internal/codec/`, and `internal/forwarder/` hold module scaffolding and design notes.
- If you add more code, keep top-level directories explicit (for example `cmd/`, `internal/`, or `pkg/`) and update this guide.

## Build, Test, and Development Commands
- No build, test, or run scripts are defined in this repository at this time.
- When adding tooling, document the exact commands here (for example `go test ./...` or `make build`) and include any required setup steps.

## Coding Style & Naming Conventions
- No formatting, linting, or style configuration is present yet.
- If you introduce code, add the relevant configuration files (for example `.golangci.yml`, `.editorconfig`, or formatter configs) and summarize the rules here.
- Keep names explicit and consistent within any new modules, packages, or directories you add.

## Testing Guidelines
- No testing framework is configured.
- If you add tests, document the framework, test file naming pattern, and how to run the suite (for example `*_test.go` with `go test ./...`).

## Commit & Pull Request Guidelines
- Git history currently contains a single commit (`Initial commit`), so no message convention is established.
- Until a standard exists, use short, imperative commit subjects (for example "Add routing skeleton"), and include context in the body when changes are non-trivial.
- For pull requests, include a clear description, reproduction steps (if applicable), and any relevant logs or screenshots.

## Agent-Specific Instructions
- This repository uses `AGENTS.md` to guide contributions; keep it updated as the project gains structure, tooling, or conventions.
