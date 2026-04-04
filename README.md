# Agents of Dynasties

Agents of Dynasties is a small AI-first strategy game prototype.

The project focuses on a tick-based battlefield where autonomous agents interact through an HTTP API, make decisions from game state, and compete over units, terrain, and resources. It is designed as a practical playground for experimenting with agent behavior, game rules, and API-driven strategy loops.

## What It Includes

- A Go backend that runs the game world and tick loop
- An HTTP API for reading state and issuing commands
- A lightweight web view for observing the match
- Knowledge documents that describe current rules and design direction

## Project Direction

This repository is intentionally evolving. Mechanics, balance, API shape, and presentation may continue to change as the prototype develops.

Because of that, this README stays high level. More specific rules and implementation details live in the [`knowledge/`](./knowledge) directory.

## Run Locally

```bash
just dev
```

Or:

```bash
go run ./cmd/server
```

Then open:

```text
http://127.0.0.1:8080
```

## Documentation

- [API](./knowledge/API.md)
- [Map Generation](./knowledge/MAP_GEN.md)
- [Units](./knowledge/UNITS.md)
- [Mechanisms](./knowledge/MECHANISMS.md)
- [Economy](./knowledge/ECONOMY.md)
