# AGENTS.md

This file provides guidance to coding agents working in this repository. It is adapted from `CLAUDE.md`, but updated to reflect the current codebase and preferred workflows in this repo.

## Project Overview

**Agents of Dynasties** is a headless-first RTS game where AI agents act as tactical controllers through HTTP/WebSocket APIs. The backend is written in Go and the game advances on a 10-second tick cycle.

Current module path:

```text
github.com/jason9075/agents_of_dynasties
```

## Development Environment

This project uses `flake.nix` for a reproducible dev shell. Prefer entering the shell before running build, test, or lint commands.

```bash
nix develop
```

## Preferred Commands

Prefer `just` targets when they exist:

```bash
just build     # go build ./...
just run       # go run ./cmd/server
just dev       # watch Go/web files and restart server
just test      # go test ./...
just test-pkg internal/hex
just lint      # golangci-lint run
just fmt       # gofmt -w .
just tidy      # go mod tidy
```

Direct Go commands also work:

```bash
go build ./...
go test ./...
golangci-lint run
gofmt -w .
```

## Repository Layout

Important paths:

```text
cmd/server/           server entrypoint
internal/api/         HTTP handlers, middleware, server wiring
internal/entity/      units, buildings, shared entity types
internal/hex/         hex coordinate logic
internal/ticker/      tick loop and command processing
internal/terrain/     terrain definitions
internal/world/       world state and generation
knowledge/API.md      API notes / contract reference
web/                  simple browser-facing assets
```

## Product Constraints

Preserve these core game assumptions unless the user asks to change them:

- 20x15 hexagonal map using odd-r offset coordinates `(q, r)`
- 10-second tick loop
- Headless-first design for AI-agent control
- Fog of war where terrain is known initially but enemy entities require visibility
- Last-command-wins semantics within a tick window
- Population cap of 20 units per team
- Building limits per team: Town Center, Barracks, Stable, Archery Range
- Core resources: Food, Gold, Stone, Wood

## API Expectations

The main gameplay surface is agent-consumable. Keep the API simple and explicit.

- `GET /map` returns static terrain
- `GET /state` returns team state plus visibility-filtered enemy information
- Expected action vocabulary includes `MOVE_FAST`, `MOVE_GUARD`, `ATTACK`, `GATHER`, `BUILD`, `PRODUCE`

When changing handlers or payloads, check whether `knowledge/API.md`, `web/`, or agent-facing docs also need updates.

## Working Style

- Prefer small, testable changes over broad refactors
- Keep domain logic inside `internal/` packages and keep `cmd/server` thin
- Add or update tests when changing hex math, world generation, or tick/command logic
- Preserve clear separation between simulation state, API serialization, and UI/demo assets
- Prefer standard library solutions unless a dependency is clearly justified

## Agent Notes

- Read existing code before changing architecture assumptions
- Check for `just` tasks first before inventing new commands
- If you touch game rules or API shapes, update adjacent documentation when needed
- Treat `CLAUDE.md` as historical context; `AGENTS.md` is the primary agent instruction file going forward
