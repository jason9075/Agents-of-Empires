# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Agents of Dynasties** is a headless-first RTS game where AI agents (LLMs) act as tactical controllers, communicating via a REST/WebSocket API. The game runs on a 10-second tick cycle. This is a Go backend project.

## Development Environment

This project uses NixOS with a `flake.nix` for reproducible builds. The dev shell includes `go`, `gopls`, `golangci-lint`, and `delve`.

```bash
nix develop       # Enter the dev shell
```

## Expected Commands (once project is initialized)

```bash
go build ./...    # Build all packages
go test ./...     # Run all tests
go test ./pkg/... # Run tests in a specific package
golangci-lint run # Lint the codebase
```

## Architecture

### Tech Stack
- **Backend:** Go (standard library + Gorilla WebSocket or Gin)
- **Frontend:** React/Vue or Vanilla JS dashboard (WebSocket real-time updates)

### Game World
- **30x30 hexagonal grid** using axial coordinates `(q, r)`
- **Terrain types:** Plain, Forest, Mountain, Lake, Gold Mine, Stone Mine, Orchard, Deer
- **Fog of War:** Static terrain known at T=0; enemy units/buildings only visible within friendly unit LOS

### Core Domain Structs (to be built)
- `World` — manages the 30x30 grid and entity registry
- `Ticker` — 10-second loop that processes the command queue (last-command-wins per unit per tick)

### API Surface (Agent Skill Framework)
Agents interact via JSON-RPC or REST:

| Endpoint | Purpose |
|----------|---------|
| `GET /map` | Static terrain grid |
| `GET /state` | Resources, unit positions, LOS-masked enemy visibility |

Actions: `MOVE_FAST`, `MOVE_GUARD`, `ATTACK`, `GATHER`, `BUILD`, `PRODUCE`

**Last-command-wins rule:** If multiple commands arrive for the same unit within a tick window, the latest replaces all previous ones.

### Implementation Phases
1. **Hex Engine** — coordinate system, `World` struct, `Ticker`
2. **Fog of War & LOS** — visibility circles, view-filtering layer on `/state`
3. **Resource & Combat** — villager gathering per tick, rock-paper-scissors combat (Spearman > Cavalry > Archers > Spearman)
4. **Agent Integration** — `AgentSkills` prompt-friendly schema, human instruction channel

### Game Constraints
- Population cap: 20 units per team
- Buildings per team: 1 Town Center, 1 Barracks, 1 Stable, 1 Archery Range
- Resources: Food (Orchards/Deer), Gold, Stone, Wood

### Human Dashboard
Browser UI with three features:
1. **Monitor** — God Mode or Player View map
2. **Directive Injection** — inject high-level text prompt into agent context (e.g., `"Priority: Economy"`)
3. **Override / Panic Button** — manual command that agent cannot overwrite for the current turn
