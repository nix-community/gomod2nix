# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gomod2nix converts Go modules (`go.mod`/`go.sum`) into Nix derivations. It generates a `gomod2nix.toml` file containing dependency metadata with NAR hashes, which Nix builders consume to create reproducible Go packages.

## Build & Development Commands

Development requires Nix. Enter the dev shell first:
```bash
nix develop          # or: direnv allow (uses .envrc)
```

Build and run:
```bash
go build             # build the binary
nix build .#gomod2nix  # build via Nix
```

Lint and format:
```bash
nix-shell --run 'golangci-lint run'   # Go linting
nix-shell --run 'treefmt --ci'        # Nix formatting (nixfmt-tree)
```

Tests (integration tests that build real Go projects with Nix):
```bash
# List available test cases
nix-shell --run 'go run tests/run.go list'

# Run a single test
nix-shell --run 'go run tests/run.go run <test-name>'

# Test names: cli-args, cross, ethermint, helm, minikube, mkgoenv, vendored-modules
```

Unit tests:
```bash
go test ./internal/lib/...
```

After modifying Go dependencies, regenerate the lockfile:
```bash
nix-shell --run gomod2nix    # must produce no diff in CI
```

## Architecture

### Go Code (`internal/`)

The CLI (Cobra) has three commands: root (default=generate), `generate`, and `import`.

**Generation pipeline** (`internal/generate/generate.go`):
1. Parse `go.mod` via `golang.org/x/mod/modfile`
2. Run `go mod download --json` to resolve dependencies
3. Compute NAR hashes using `go-nix` (parallel, 10 workers default via `internal/lib/executor.go`)
4. Diff against existing `gomod2nix.toml` cache to avoid rehashing
5. Marshal output as TOML via `internal/schema/schema.go` (schema version 3)

`internal/generate/temp.go` handles the `gomod2nix package@version` use case by creating a temporary Go project.

### Nix Builder (`builder/`)

`builder/default.nix` exports the key Nix functions: `buildGoApplication`, `mkGoEnv`, `mkGoCacheEnv`, and `hooks`. It reads `gomod2nix.toml` via `builder/parser.nix` (a pure-Nix go.mod parser), fetches modules with `fetchGoModule`, and sets up vendor symlinks.

**Build hooks** (`builder/hooks/`):
- `go-config-hook.sh` — sets up GOPATH, GOCACHE, GOSUMDB, vendor directory
- `go-build-hook.sh` — configures GOOS/GOARCH for cross-compilation
- `go-check-hook.sh` — runs `go test`
- `go-install-hook.sh` — installs compiled binaries

Helper tools in `builder/symlink/` (vendor symlink manager) and `builder/cachegen/` (build cache generator).

Cross-compilation uses `buildPackages` for build hook dependencies and distinguishes `stdenv.buildPlatform` vs `stdenv.targetPlatform`.

### Nix Integration Points

- `flake.nix` — defines packages, dev shell, overlay, and templates (app/container)
- `overlay.nix` — nixpkgs overlay
- `default.nix` — gomod2nix package definition (uses `buildGoApplication` on itself)
- `shell.nix` — dev shell with Go, golangci-lint, treefmt, nixfmt-tree

## Key Conventions

- Go 1.24.0, module path: `github.com/nix-community/gomod2nix`
- TOML output uses `pelletier/go-toml/v2` with indented table formatting
- CI runs on 5 platforms: x86_64-linux, x86_64-darwin, aarch64-linux, aarch64-darwin, riscv64-linux
- The `gomod2nix.toml` in the repo root is the project's own dependency lockfile and must stay in sync
