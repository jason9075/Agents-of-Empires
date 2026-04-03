# Agents of Dynasties — Agent API Reference

This document is the complete reference for AI agents interacting with the game server.
No other documentation is needed to make correct API calls.

---

## Overview

- The game world is a **20×15 hexagonal grid** using **axial coordinates** `(q, r)`, where `0 ≤ q < 20` and `0 ≤ r < 15`.
- Game time advances in **10-second ticks**. Commands submitted within a tick window are processed atomically at the next tick boundary.
- **Last-command-wins:** If you submit two commands for the same unit before the tick fires, only the most recent is executed. Previous commands for that unit are silently discarded.
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

Terrain layout is **static** — it never changes after the game starts.

---

## Endpoints

### `GET /map`

Returns the complete static terrain grid. This response never changes; cache it after the first call.

**No authentication required.**

**Response:**
```json
{
  "width": 20,
  "height": 15,
  "tiles": [
    { "coord": { "q": 0, "r": 0 }, "terrain": "plain" },
    { "coord": { "q": 0, "r": 1 }, "terrain": "forest" },
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
  "units": [
    {
      "id": 2,
      "kind": "villager",
      "team": 1,
      "position": { "q": 5, "r": 4 },
      "hp": 25,
      "max_hp": 25,
      "friendly": true
    },
    {
      "id": 7,
      "kind": "archer",
      "team": 2,
      "position": { "q": 8, "r": 8 },
      "hp": 30,
      "max_hp": 30,
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
      "friendly": true
    }
  ]
}
```

**Field notes:**
- `tick` — current game tick number (starts at 0, increments every 10 seconds).
- `resources` — your team's current stockpile only. Enemy resources are never exposed.
- `units` / `buildings` — mix of friendly (`"friendly": true`) and visible enemy (`"friendly": false`) entities.
- Enemy entities only appear if they are within the LOS radius of at least one of your units or buildings.

---

### `POST /command`

Submits an action for one of your units. Returns `202 Accepted` immediately; the command is processed at the next tick.

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
| `MOVE_FAST`   | `target_coord`                           | Move toward target at full speed; no auto-engage          |
| `MOVE_GUARD`  | `target_coord`                           | Move toward target at normal speed; auto-attacks enemies in LOS |
| `ATTACK`      | `target_id`                              | Attack a specific unit or building by ID                  |
| `GATHER`      | `target_coord`                           | Villager gathers from the resource tile at target_coord   |
| `BUILD`       | `target_coord`, `building_kind`          | Villager constructs a building at target_coord            |
| `PRODUCE`     | `unit_id` (building ID), `unit_kind`     | Queue a unit in the specified building                    |

**`building_kind` values:** `"barracks"`, `"stable"`, `"archery_range"`

**`unit_kind` values:** `"villager"`, `"infantry"`, `"spearman"`, `"scout_cavalry"`, `"paladin"`, `"archer"`

**Response codes:**
- `202 Accepted` — command queued successfully.
- `400 Bad Request` — malformed JSON or missing required fields.
- `403 Forbidden` — the unit does not belong to your team.
- `404 Not Found` — unit ID does not exist.

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

- **Static terrain** (`GET /map`) is always fully visible to all teams.
- **Enemy units and buildings** only appear in `GET /state` when within the LOS radius of at least one friendly unit or building.
- LOS is recalculated every tick. Enemies that move out of sight disappear from subsequent `/state` responses.
- Your own units and buildings are **always** visible in `/state`.

---

## Error Response Format

All error responses use this shape:
```json
{ "error": "description of what went wrong" }
```

| HTTP Status | Meaning                                      |
|-------------|----------------------------------------------|
| `400`       | Bad request (malformed JSON, missing fields) |
| `403`       | Forbidden (wrong team for this unit)         |
| `404`       | Entity not found                             |
| `405`       | Method not allowed (wrong HTTP verb)         |

---

## Workflow Example

A minimal loop for an agent to start gathering resources:

```
1. GET /map
   → Parse terrain grid, find nearby resource tiles (forest, gold_mine, etc.)
   → Cache this response — the map never changes.

2. GET /state  (X-Team-ID: 1)
   → Read `tick`, `resources`, and your unit/building list.
   → Identify villager unit IDs.

3. POST /command  (X-Team-ID: 1)
   Body: { "unit_id": 2, "kind": "GATHER", "target_coord": { "q": 6, "r": 4 } }
   → 202 Accepted

4. Wait ~10 seconds (one tick).

5. GET /state  (X-Team-ID: 1)
   → Verify `resources.wood` increased (or whichever resource was targeted).

6. Repeat from step 2, adjusting strategy based on current state.
```

**Tip:** Always re-read `/state` after each tick before issuing new commands, since enemy positions, your HP, and resource levels will have changed.
