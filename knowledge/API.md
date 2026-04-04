# Agents of Dynasties — Agent API Reference

This document is the complete reference for AI agents interacting with the game server.
No other documentation is needed to make correct API calls.

---

## Overview

- The game world is a **20×15 hexagonal grid** using **axial coordinates** `(q, r)`, where `0 ≤ q < 20` and `0 ≤ r < 15`.
- Game time advances in ticks. The server default is **10 seconds per tick**, but startup flags may override it.
- **Last-command-wins:** If you submit two commands for the same unit before the tick fires, only the most recent is executed. Previous commands for that unit are silently discarded.
- Unit commands are persistent. After the next tick boundary applies them, the unit keeps executing that status until it completes, is overwritten, or receives `STOP`.
- Authenticate every request by setting the `X-Team-ID` header to `1` or `2`.
- Base URL: `http://localhost:8080` (configurable via `--addr` flag on server start).

---

## Coordinate System

Each cell is identified by integer axial coordinates `(q, r)`.

```
Neighbor directions from any cell (q, r):
  (+1,  0)  East
  (+1, -1)  North-East
  ( 0, -1)  North-West
  (-1,  0)  West
  (-1, +1)  South-West
  ( 0, +1)  South-East
```

**Distance formula** (use this to plan moves and check LOS):
```
distance(a, b) = max( |a.q - b.q|, |a.r - b.r|, |(a.q + a.r) - (b.q + b.r)| )
```

---

## Terrain Types

| Terrain      | Passable | Resource Yield | Notes                            |
|--------------|----------|----------------|----------------------------------|
| `plain`      | Yes      | —              | Default open ground              |
| `forest`     | Yes      | `wood`         | Villager can gather here         |
| `mountain`   | **No**   | —              | Blocks movement                  |
| `lake`       | **No**   | —              | Blocks movement                  |
| `gold_mine`  | Yes      | `gold`         | Villager can gather here         |
| `stone_mine` | Yes      | `stone`        | Villager can gather here         |
| `orchard`    | Yes      | `food`         | Villager can gather here         |
| `deer`       | Yes      | `food`         | Villager can gather here         |

Blocking terrain is static after map generation, but exhausted resource tiles can become `plain`.

---

## Endpoints

### `GET /map`

Returns the current terrain grid.

This endpoint is **not static**. Resource depletion can change a tile to `plain`, so callers should refresh `/map` over time instead of caching it forever.

**No authentication required.**

**Response:**
```json
{
  "width": 20,
  "height": 15,
  "tiles": [
    { "coord": { "q": 0, "r": 0 }, "terrain": "plain" },
    { "coord": { "q": 0, "r": 1 }, "terrain": "forest", "remaining": 300 },
    ...
  ]
}
```

Tiles are ordered by `q * 15 + r` (row-major from q=0, r=0 to q=19, r=14).

---

### `GET /state`

Returns current game state filtered by your team's **Line of Sight (LOS)**.

**Required header:** `X-Team-ID: 1` or `X-Team-ID: 2`

**Response:**
```json
{
  "tick": 5,
  "resources": {
    "food":  185,
    "gold":  100,
    "stone": 100,
    "wood":  220
  },
  "population": {
    "used": 4,
    "reserved": 1,
    "cap": 20
  },
  "last_tick_failed_commands": [
    {
      "command_id": 14,
      "unit_id": 2,
      "kind": "MOVE_FAST",
      "target_coord": { "q": 4, "r": 4 },
      "submitted_tick": 4,
      "resolved_tick": 5,
      "code": "target_building_occupied",
      "reason": "target hex is occupied by a building at resolution"
    }
  ],
  "units": [
    {
      "id": 2,
      "kind": "villager",
      "team": 1,
      "position": { "q": 5, "r": 4 },
      "hp": 25,
      "max_hp": 25,
      "status": "GATHERING",
      "status_phase": "RETURNING",
      "status_target_coord": { "q": 5, "r": 5 },
      "carry_resource": "food",
      "carry_amount": 18,
      "friendly": true
    },
    {
      "id": 7,
      "kind": "archer",
      "team": 2,
      "position": { "q": 8, "r": 8 },
      "hp": 30,
      "max_hp": 30,
      "status": "ATTACKING",
      "status_phase": "ATTACKING",
      "status_target_id": 2,
      "attack_target_id": 2,
      "friendly": false
    }
  ],
  "buildings": [
    {
      "id": 1,
      "kind": "town_center",
      "team": 1,
      "position": { "q": 4, "r": 4 },
      "hp": 600,
      "max_hp": 600,
      "complete": true,
      "build_progress": 2,
      "build_ticks_total": 2,
      "production_queue_len": 1,
      "production_ticks_remaining": 1,
      "friendly": true
    }
  ]
}
```

**Field notes:**
- `tick` — current game tick number (starts at 0 and increments once per server tick interval).
- `resources` — your team's current stockpile only. Enemy resources are never exposed.
- `population` — current living population, reserved queue population, and the hard team cap.
- `last_tick_failed_commands` — permanent execution failures from the most recently resolved tick for your team only.
- `status` — the unit's persistent action state, such as `IDLE`, `MOVING_FAST`, `MOVING_GUARD`, `ATTACKING`, `GATHERING`, or `BUILDING`.
- `status_phase` — the unit's current sub-phase inside that state.
- `status_target_coord` / `status_target_id` / `status_building_kind` — the target the unit is currently committed to.
- failed command records include `command_id`, original target fields, `submitted_tick`, `resolved_tick`, and a machine-readable `code`.
- `units` / `buildings` — mix of friendly (`"friendly": true`) and visible enemy (`"friendly": false`) entities.
- Enemy entities only appear if they are within the LOS radius of at least one of your units or buildings.

---

### `GET /commands`

Returns your team's currently pending commands that have been accepted but not yet resolved at the next tick boundary.

This endpoint exists so an agent can avoid re-issuing duplicate or contradictory commands when it polls state multiple times within the same tick.

**Required header:** `X-Team-ID: 1` or `X-Team-ID: 2`

**Response:**
```json
{
  "tick": 5,
  "commands": [
    {
      "command_id": 14,
      "submitted_tick": 5,
      "team": 1,
      "unit_id": 2,
      "kind": "MOVE_GUARD",
      "target_coord": { "q": 7, "r": 4 }
    },
    {
      "command_id": 15,
      "submitted_tick": 5,
      "team": 1,
      "building_id": 1,
      "kind": "PRODUCE",
      "unit_kind": "villager"
    }
  ]
}
```

**Field notes:**
- `tick` — the current resolved world tick. The listed commands are queued for the next tick boundary after this state.
- `commands` — only your team's pending commands.
- `command_id` — stable ID for correlating an accepted command with later execution failure reporting.
- `submitted_tick` — the resolved tick number when the server accepted the command into the pending queue.
- pending commands already follow last-command-wins semantics, so each actor appears at most once.
- this is not command history; once the next tick resolves, the queue is cleared.
- use `/commands` to inspect submissions still waiting for the next tick, and `/state` to inspect what units are still doing across later ticks.

---

### `POST /command`

Submits an action for one of your units. Returns `202 Accepted` immediately with a `command_id`; the command is queued for the next tick and then updates the unit's persistent status.

**Required header:** `X-Team-ID: 1` or `X-Team-ID: 2`

**Request body:**
```json
{
  "unit_id": 2,
  "kind": "MOVE_FAST",
  "target_coord": { "q": 7, "r": 4 }
}
```

**Command kinds:**

| Kind          | Required fields                          | Effect                                                    |
|---------------|------------------------------------------|-----------------------------------------------------------|
| `MOVE_FAST`   | `target_coord`                           | Persistently move toward target at full speed until arrival, overwrite, or `STOP` |
| `MOVE_GUARD`  | `target_coord`                           | Persistently move toward target at guarded speed and switch into combat if an enemy enters range |
| `ATTACK`      | `target_id`                              | Persistently pursue and attack a specific unit or building until completion, overwrite, or `STOP` |
| `GATHER`      | `target_coord`                           | Villager persistently gathers from that node, returns to a friendly `town_center`, deposits, and repeats |
| `BUILD`       | `target_coord`, `building_kind`          | Villager persistently moves to the target and keeps building until completion |
| `PRODUCE`     | `building_id`, `unit_kind`               | Queue a unit in the specified building                    |
| `STOP`        | `unit_id`                                | Clear the unit's current persistent status and return it to `IDLE` |

**`building_kind` values:** `"barracks"`, `"stable"`, `"archery_range"`

**`unit_kind` values:** `"villager"`, `"infantry"`, `"spearman"`, `"scout_cavalry"`, `"paladin"`, `"archer"`

**Accepted response:**

```json
{
  "command_id": 14,
  "tick": 5
}
```

**Response codes:**
- `202 Accepted` — command queued successfully.
- `400 Bad Request` — malformed JSON, missing required fields, insufficient resources, population cap reached, or illegal command.
- `403 Forbidden` — the unit does not belong to your team.
- `404 Not Found` — unit ID does not exist.

Error responses use this shape:

```json
{
  "error": {
    "code": "population_cap_reached",
    "reason": "team population cap would be exceeded"
  }
}
```

---

## Entity Reference

### Units

| Kind            | Produced By    | LOS Radius | Speed (Fast) | Speed (Guard) | Max HP | Attack | Notes                        |
|-----------------|----------------|------------|--------------|---------------|--------|--------|------------------------------|
| `villager`      | Town Center    | 2          | 2            | 1             | 25     | 3      | Gathers resources, builds    |
| `infantry`      | Barracks       | 3          | 3            | 2             | 40     | 8      | General melee unit           |
| `spearman`      | Barracks       | 3          | 2            | 2             | 45     | 10     | Strong vs. cavalry           |
| `scout_cavalry` | Stable         | 4          | 5            | 3             | 30     | 6      | Fastest unit, wide LOS       |
| `paladin`       | Stable         | 3          | 3            | 2             | 70     | 12     | Heavy cavalry                |
| `archer`        | Archery Range  | 4          | 2            | 2             | 30     | 9      | Strong vs. infantry          |

**Combat triangle (Phase 3):** Spearman > Cavalry > Archer > Spearman

### Buildings

| Kind             | Limit / Team | Max HP | Produces                              |
|------------------|--------------|--------|---------------------------------------|
| `town_center`    | 1            | 600    | `villager`                            |
| `barracks`       | 1            | 400    | `infantry`, `spearman`                |
| `stable`         | 1            | 400    | `scout_cavalry`, `paladin`            |
| `archery_range`  | 1            | 350    | `archer`                              |

Buildings also provide LOS (radius 3) around their position.

---

## Resources

| Resource | Gathered From               | Starting Amount |
|----------|-----------------------------|-----------------|
| `food`   | `orchard`, `deer`           | 200             |
| `gold`   | `gold_mine`                 | 100             |
| `stone`  | `stone_mine`                | 100             |
| `wood`   | `forest`                    | 200             |

---

## Fog of War

- **Terrain visibility** (`GET /map`) is always public to both teams.
- `/map` is public, but resource depletion can still change returned terrain over time.
- **Enemy units and buildings** only appear in `GET /state` when within the LOS radius of at least one friendly unit or building.
- LOS is recalculated every tick. Enemies that move out of sight disappear from subsequent `/state` responses.
- Your own units and buildings are **always** visible in `/state`.

---

## Error Response Format

All error responses use this shape:
```json
{
  "error": {
    "code": "insufficient_resources",
    "reason": "team cannot afford this building"
  }
}
```

| HTTP Status | Meaning |
|-------------|---------|
| `400` | Bad request, illegal command, insufficient resources, or population cap reached |
| `403` | Forbidden (wrong team for this unit or building) |
| `404` | Entity not found |
| `405` | Method not allowed |

---

## Workflow Example

A minimal loop for an agent to start gathering resources:

```
1. GET /map
   → Parse terrain grid, find nearby resource tiles (forest, gold_mine, etc.)
   → Refresh this response over time, because depleted resource tiles can become `plain`.

2. GET /state  (X-Team-ID: 1)
   → Read `tick`, `resources`, `population`, and your unit/building list.
   → Identify villager unit IDs and current `status`.

3. GET /commands  (X-Team-ID: 1)
   → Check which actors already have pending commands in the current tick window.

4. POST /command  (X-Team-ID: 1)
   Body: { "unit_id": 2, "kind": "GATHER", "target_coord": { "q": 6, "r": 4 } }
   → 202 Accepted with a `command_id`

5. GET /commands  (X-Team-ID: 1)
   → Verify the new command is now queued, and avoid sending a duplicate for the same actor.

6. Wait for one tick.

7. GET /state  (X-Team-ID: 1)
   → Verify the villager's `status`, `status_phase`, `carry_amount`, or stockpile changed as expected.
   → Also inspect `last_tick_failed_commands` for permanent execution failures that only became knowable at tick resolution.

8. Repeat from step 2, adjusting strategy based on current state.
```

**Tip:** Always re-read `/state` after each tick before issuing new commands, since enemy positions, your HP, and resource levels will have changed.
