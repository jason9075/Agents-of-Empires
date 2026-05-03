# Agents of Dynasties Quick Reference

Use this reference only when you need exact API shapes or game constants while playing.

## Defaults

- Map: `20 x 15`
- Coordinates: odd-r offset hex `(q, r)`
- Default tick interval: `10s`
- Header: `X-Team-ID: 1` or `2`
- Base URL: usually `http://127.0.0.1:8080`

---

## Endpoints

### `GET /map`

Public terrain map. Refresh over time because resource tiles deplete into `plain`.

**Response:**
```json
{
  "width": 20,
  "height": 15,
  "tiles": [
    { "coord": {"q": 0, "r": 0}, "terrain": "plain" },
    { "coord": {"q": 1, "r": 2}, "terrain": "forest", "remaining": 300 },
    { "coord": {"q": 3, "r": 4}, "terrain": "gold_mine", "remaining": 400 }
  ]
}
```

- `terrain`: one of `plain`, `forest`, `mountain`, `lake`, `gold_mine`, `stone_mine`, `orchard`, `deer`
- `remaining`: only present for resource tiles; omitted when `0` (depleted → treated as `plain`)

---

### `GET /state` *(requires `X-Team-ID` header)*

Returns your team's visible game state with LOS masking applied.

**Response:**
```json
{
  "game_over": false,
  "winner": "",
  "tick": 5,
  "resources": { "food": 200, "gold": 100, "stone": 100, "wood": 200 },
  "population": { "used": 3, "reserved": 1, "cap": 20 },
  "last_tick_failed_commands": [...],
  "last_tick_contested_hexes": [...],
  "units": [...],
  "buildings": [...]
}
```

`winner` is `"1"`, `"2"`, or `"draw"` when `game_over` is `true`.

**`resources`:** `food`, `gold`, `stone`, `wood` — all integers.

**`population`:**
- `used`: living units
- `reserved`: pop slots reserved by queued production (counts toward cap)
- `cap`: always `20`

**Unit object:**
```json
{
  "id": 3,
  "kind": "villager",
  "team": 1,
  "position": {"q": 5, "r": 7},
  "hp": 25,
  "max_hp": 25,
  "carry_resource": "wood",
  "carry_amount": 12,
  "status": "GATHERING",
  "status_phase": "RETURNING",
  "status_target_coord": {"q": 3, "r": 2},
  "status_target_id": null,
  "status_building_kind": "",
  "attack_target_id": null,
  "friendly": true
}
```

- `carry_resource`: `""`, `"food"`, `"wood"`, `"gold"`, `"stone"`
- `friendly`: `true` = your unit; `false` = visible enemy unit
- See **Unit Status Values** section below for `status` and `status_phase` enums

**Building object:**
```json
{
  "id": 1,
  "kind": "town_center",
  "team": 1,
  "position": {"q": 3, "r": 3},
  "hp": 600,
  "max_hp": 600,
  "complete": true,
  "build_progress": 2,
  "build_ticks_total": 2,
  "production_queue_len": 1,
  "production_ticks_remaining": 1,
  "rally_point_coord": {"q": 4, "r": 3},
  "friendly": true
}
```

- `complete`: `false` = still under construction; do not queue production until `true`
- `build_progress` / `build_ticks_total`: construction tracking
- `production_queue_len`: number of units queued
- `production_ticks_remaining`: ticks until next unit spawns
- `rally_point_coord`: `null` if not set

**`last_tick_failed_commands` item:**
```json
{
  "command_id": 42,
  "unit_id": 3,
  "building_id": null,
  "kind": "BUILD",
  "target_coord": {"q": 6, "r": 6},
  "target_id": null,
  "building_kind": "barracks",
  "unit_kind": null,
  "submitted_tick": 4,
  "resolved_tick": 5,
  "code": "insufficient_resources",
  "reason": "team cannot afford this building"
}
```

**`last_tick_contested_hexes` item:**
```json
{
  "coord": {"q": 8, "r": 5},
  "team1_unit_ids": [7],
  "team2_unit_ids": [12]
}
```

---

### `GET /commands` *(requires `X-Team-ID` header)*

Your team's currently queued commands for the next tick.

**Response:**
```json
{
  "tick": 5,
  "commands": [
    {
      "command_id": 42,
      "submitted_tick": 5,
      "team": 1,
      "unit_id": 3,
      "building_id": null,
      "kind": "GATHER",
      "target_coord": {"q": 3, "r": 2},
      "target_id": null,
      "building_kind": null,
      "unit_kind": null
    }
  ]
}
```

---

### `POST /command` *(requires `X-Team-ID` header)*

Submit a command. Last-command-wins per actor within a tick window.

**Success response (HTTP 202):**
```json
{ "command_id": 43, "tick": 5 }
```

**Per-command required fields:**

| Kind | Required fields |
|------|----------------|
| `MOVE_FAST` | `unit_id`, `target_coord` |
| `MOVE_GUARD` | `unit_id`, `target_coord` |
| `ATTACK` | `unit_id`, `target_id` |
| `GATHER` | `unit_id`, `target_coord` (must be a resource tile) |
| `BUILD` | `unit_id`, `target_coord` (must be `plain`), `building_kind` |
| `PRODUCE` | `building_id`, `unit_kind` |
| `CANCEL_PRODUCE` | `building_id` |
| `SET_RALLY_POINT` | `building_id`, `target_coord` |
| `STOP` | `unit_id` |
| `DELETE` | `unit_id` OR `building_id` |

Example payloads:
```bash
# Move unit 3 to hex (8,5)
curl -s -X POST http://127.0.0.1:8080/command \
  -H 'X-Team-ID: 1' -H 'Content-Type: application/json' \
  -d '{"unit_id":3,"kind":"MOVE_FAST","target_coord":{"q":8,"r":5}}'

# Attack entity 12 with unit 7
curl -s -X POST http://127.0.0.1:8080/command \
  -H 'X-Team-ID: 1' -H 'Content-Type: application/json' \
  -d '{"unit_id":7,"kind":"ATTACK","target_id":12}'

# Build barracks with villager 3 at (6,6)
curl -s -X POST http://127.0.0.1:8080/command \
  -H 'X-Team-ID: 1' -H 'Content-Type: application/json' \
  -d '{"unit_id":3,"kind":"BUILD","target_coord":{"q":6,"r":6},"building_kind":"barracks"}'

# Produce infantry from barracks building 5
curl -s -X POST http://127.0.0.1:8080/command \
  -H 'X-Team-ID: 1' -H 'Content-Type: application/json' \
  -d '{"building_id":5,"kind":"PRODUCE","unit_kind":"infantry"}'
```

---

## Unit Status Values

`status` (persists across ticks until overwritten):

| Value | Meaning |
|-------|---------|
| `IDLE` | No active order |
| `MOVING_FAST` | Moving at full speed toward target |
| `MOVING_GUARD` | Moving cautiously, auto-attacks adjacent enemies |
| `ATTACKING` | Engaging a target |
| `GATHERING` | Gathering resources in a loop |
| `BUILDING` | Constructing a building |

`status_phase` (sub-state within the current status):

| Value | Meaning |
|-------|---------|
| `""` | None (idle) |
| `MOVING_TO_TARGET` | Pathfinding toward destination |
| `CLOSING_TO_ATTACK` | Moving into attack range |
| `ATTACKING` | Within range, dealing damage |
| `MOVING_TO_RESOURCE` | Walking toward resource tile |
| `GATHERING` | On-site, harvesting |
| `RETURNING` | Walking back to Town Center |
| `DEPOSITING` | Arrived, depositing carry |
| `MOVING_TO_BUILD` | Walking toward build site |
| `CONSTRUCTING` | Adjacent, advancing build progress |

---

## Unit Stats

| Kind | HP | ATK | DEF | Fast | Guard | LOS | Range | Forest |
|------|----|-----|-----|------|-------|-----|-------|--------|
| `villager` | 25 | 3 | 0 | 2 | 1 | 2 | 1 | yes |
| `infantry` | 40 | 8 | 3 | 3 | 2 | 3 | 1 | yes |
| `spearman` | 45 | 10 | 4 | 2 | 2 | 3 | 1 | yes |
| `scout_cavalry` | 30 | 6 | 1 | 5 | 3 | 4 | 1 | **no** |
| `paladin` | 70 | 12 | 6 | 3 | 2 | 3 | 1 | **no** |
| `archer` | 30 | 9 | 1 | 2 | 2 | 4 | **2** | yes |

- **Fast / Guard**: hexes moved per tick under `MOVE_FAST` / `MOVE_GUARD`
- **LOS**: visibility radius in hexes
- **Range**: attack range; archer can attack from 2 hexes away
- **Forest**: whether the unit can enter `forest` tiles

---

## Counter Bonus Table

Extra attack added against the defender kind:

| Attacker | Defender | Bonus ATK |
|----------|----------|-----------|
| `spearman` | `scout_cavalry` | +8 |
| `spearman` | `paladin` | +8 |
| `archer` | `spearman` | +4 |
| `scout_cavalry` | `archer` | +4 |

Effective damage = `max(1, attacker_ATK + counter_bonus - defender_DEF)`

---

## Building Stats

| Kind | Max HP | LOS | Build Ticks |
|------|--------|-----|-------------|
| `town_center` | 600 | 3 | — (cannot be built) |
| `barracks` | 400 | 3 | 2 |
| `stable` | 400 | 3 | 2 |
| `archery_range` | 350 | 3 | 2 |

---

## Buildable Buildings

- `barracks`
- `stable`
- `archery_range`

`town_center` cannot be built by villagers.

---

## Trainable Units & Producers

| Building | Trains |
|----------|--------|
| `town_center` | `villager` |
| `barracks` | `infantry`, `spearman` |
| `stable` | `scout_cavalry`, `paladin` |
| `archery_range` | `archer` |

---

## Costs

### Buildings

| Kind | Stone | Wood | Gold |
|------|-------|------|------|
| `barracks` | 60 | 120 | — |
| `stable` | 80 | 140 | 20 |
| `archery_range` | 60 | 130 | — |

### Units

| Kind | Food | Gold | Wood | Train Ticks |
|------|------|------|------|-------------|
| `villager` | 50 | — | — | 1 |
| `infantry` | 60 | — | 20 | 1 |
| `spearman` | 50 | 10 | 20 | 1 |
| `scout_cavalry` | 70 | 20 | 20 | 2 |
| `paladin` | 90 | 45 | — | 2 |
| `archer` | 40 | 15 | 30 | 1 |

---

## Population

- Team cap: `20`
- All units cost `1` population
- `population.reserved` counts units queued for production — they hold pop slots before spawning

---

## Starting Resources

Each team begins with: `food: 200`, `gold: 100`, `stone: 100`, `wood: 200`

---

## Resource Gathering

Only villagers can gather. Gathering is an auto-loop: the villager moves to the node, harvests, returns to town center, deposits, then repeats.

| Terrain | Resource | Per-tick yield | Node capacity |
|---------|----------|---------------|---------------|
| `forest` | wood | 20 | 300 |
| `orchard` | food | 18 | 240 |
| `deer` | food | 24 | 120 |
| `gold_mine` | gold | 12 | 400 |
| `stone_mine` | stone | 10 | 350 |

Villager carry capacity: **24** units of any resource.

When a node depletes (`remaining` reaches 0), the tile becomes `plain` — refresh `/map` if a villager unexpectedly idles.

---

## Terrain Passability

| Terrain | Passable | Blocks LOS |
|---------|----------|-----------|
| `plain` | all units | no |
| `forest` | all except cavalry | yes |
| `mountain` | none | yes |
| `lake` | none | no |
| `gold_mine` | all units | no |
| `stone_mine` | all units | no |
| `orchard` | all units | no |
| `deer` | all units | no |

Cavalry = `scout_cavalry` and `paladin`.

---

## Useful Facts

- Commands are persistent after resolution — one `GATHER` or `ATTACK` keeps running across many ticks.
- Last-command-wins inside the current tick window.
- Own units and buildings are always visible in `/state`.
- Enemy entities require LOS to appear in `/state`.
- `forest` and `mountain` block line of sight.
- Only `plain` tiles can be used as build targets.
- A builder moves adjacent to the target tile; it does not stand on it.
- A building must be `complete: true` before it can produce units.
- Buildings also provide LOS radius 3 once complete.

---

## Error Response Pattern

```json
{
  "error": {
    "code": "insufficient_resources",
    "reason": "team cannot afford this building"
  }
}
```

Common error codes:

| Code | Cause |
|------|-------|
| `missing_target_coord` | `target_coord` omitted for a move/gather/build command |
| `missing_target_id` | `target_id` omitted for ATTACK |
| `missing_building_id` | `building_id` omitted for PRODUCE/CANCEL_PRODUCE/SET_RALLY_POINT |
| `missing_building_kind` | `building_kind` omitted for BUILD |
| `missing_unit_kind` | `unit_kind` omitted for PRODUCE |
| `target_out_of_bounds` | `target_coord` outside 20×15 grid |
| `target_not_passable` | destination terrain not passable for this unit type |
| `no_gatherable_resource` | `target_coord` is not a resource tile |
| `invalid_build_tile` | build target is not `plain` or is already occupied |
| `building_under_construction` | building not yet `complete` |
| `queue_empty` | CANCEL_PRODUCE on a building with empty queue |
| `invalid_producer` | building cannot train the requested unit kind |
| `insufficient_resources` | not enough resources for unit or building |
| `population_cap_reached` | `used + reserved + 1 > 20` |
| `friendly_fire_forbidden` | ATTACK target belongs to own team |
| `unit_cannot_gather` | only villagers can gather |
| `unit_cannot_build` | only villagers can build |
| `unit_not_found` | no unit with that ID |
| `building_not_found` | no building with that ID |
| `building_wrong_team` | building belongs to the other team |
| `unit_wrong_team` | unit belongs to the other team |
| `game_over` | match has already ended |
| `invalid_command_kind` | unrecognised `kind` string |
