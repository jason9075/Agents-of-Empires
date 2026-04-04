# Agents of Dynasties — Unit Rules

This document records the current unit roster, the stats that are already implemented in code, and the intended counter relationships for future combat work.

## Scope

This file is the source of truth for:

- Unit kinds
- Unit stats
- Movement and vision values
- Production source
- Intended counter relationships

It is not a promise that all listed rules are already simulated. Some parts are implemented today, and some are design targets for later phases.

## Current Implementation Status

The following are implemented today:

- Unit kinds and names
- Max HP
- Attack
- Defense
- Unit production cost
- Unit population usage
- Basic attack range rules
- `MOVE_FAST` speed
- `MOVE_GUARD` speed
- Line of sight (LOS)
- Team ownership
- Spawn and serialization to the API
- Movement command execution
- Combat command execution
- Villager gathering
- Villager building construction
- Building-based unit production

The following are not fully implemented yet:

- Formation or stance logic
- Advanced pathfinding

Important: the current implementation already applies movement, combat, gathering, building, and production in the ticker. Some systems are still simplified. Villagers now carry resources and must deposit them beside a friendly `town_center`, but there is still no richer worker automation or advanced pathfinding.

## Unit Roster

### Villager

- Role: economic worker / basic civilian
- Special role: the only unit class that can gather resources and construct buildings
- Produced by: starting spawn and `town_center`
- Max HP: 25
- Attack: 3
- Defense: 0
- `MOVE_FAST`: 2 hexes per tick
- `MOVE_GUARD`: 1 hex per tick
- LOS: 2

#### Villager Special Abilities

- Can gather map resources
- Gathered resources are carried by the villager first
- Carried resources are deposited when the villager uses `GATHER` while adjacent to a friendly `town_center`
- Can construct buildings
- Is the main unit used to expand early-game economy and infrastructure
- Should be protected by military units rather than used as a frontline fighter

### Infantry

- Role: general-purpose frontline melee
- Produced by: `barracks`
- Max HP: 40
- Attack: 8
- Defense: 3
- `MOVE_FAST`: 3 hexes per tick
- `MOVE_GUARD`: 2 hexes per tick
- LOS: 3

### Spearman

- Role: anti-cavalry frontline
- Produced by: `barracks`
- Max HP: 45
- Attack: 10
- Defense: 4
- `MOVE_FAST`: 2 hexes per tick
- `MOVE_GUARD`: 2 hexes per tick
- LOS: 3

### Scout Cavalry

- Role: fast scout / flanker
- Produced by: `stable`
- Max HP: 30
- Attack: 6
- Defense: 1
- `MOVE_FAST`: 5 hexes per tick
- `MOVE_GUARD`: 3 hexes per tick
- LOS: 4

### Paladin

- Role: heavy cavalry / durable shock unit
- Produced by: `stable`
- Max HP: 70
- Attack: 12
- Defense: 6
- `MOVE_FAST`: 3 hexes per tick
- `MOVE_GUARD`: 2 hexes per tick
- LOS: 3

### Archer

- Role: ranged damage dealer
- Produced by: `archery_range`
- Max HP: 30
- Attack: 9
- Defense: 1
- `MOVE_FAST`: 2 hexes per tick
- `MOVE_GUARD`: 2 hexes per tick
- LOS: 4

## Economy and Population Notes

- Team population cap is `20`
- Every current unit kind consumes `1` population
- Living units consume population
- Queued units also reserve population once production is successfully enqueued

### Unit Production Cost

| Unit | Food | Wood | Gold | Stone | Population |
| --- | ---: | ---: | ---: | ---: | ---: |
| `villager` | `50` | `0` | `0` | `0` | `1` |
| `infantry` | `60` | `20` | `0` | `0` | `1` |
| `spearman` | `50` | `20` | `10` | `0` | `1` |
| `archer` | `40` | `30` | `15` | `0` | `1` |
| `scout_cavalry` | `70` | `20` | `20` | `0` | `1` |
| `paladin` | `90` | `0` | `45` | `0` | `1` |

## Unit Stat Table

| Unit | HP | ATK | DEF | Fast | Guard | LOS | Producer |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Villager | 25 | 3 | 0 | 2 | 1 | 2 | Town Center / starting spawn |
| Infantry | 40 | 8 | 3 | 3 | 2 | 3 | Barracks |
| Spearman | 45 | 10 | 4 | 2 | 2 | 3 | Barracks |
| Scout Cavalry | 30 | 6 | 1 | 5 | 3 | 4 | Stable |
| Paladin | 70 | 12 | 6 | 3 | 2 | 3 | Stable |
| Archer | 30 | 9 | 1 | 2 | 2 | 4 | Archery Range |

## Design Counter Relationships

These are the intended matchup rules for future combat implementation.

### Core Counter Matrix

- `spearman` counters `scout_cavalry`
- `spearman` counters `paladin`
- `archer` counters `spearman`
- `scout_cavalry` counters `archer`
- `paladin` is a strong all-round unit but should lose cost-efficiently to `spearman`
- `infantry` is the baseline generalist and should trade evenly into most non-counter matchups
- `villager` should lose to every military unit in direct combat

### Matchup Notes

- `villager`
  - Weak against every military unit
  - Not intended for combat except emergencies

- `infantry`
  - Stable, no-extreme-counter frontline
  - Good for contesting neutral fights and protecting ranged units
  - Should not hard-counter cavalry or archers on its own

- `spearman`
  - Main anti-cavalry unit
  - Should punish cavalry if cavalry takes a direct fight
  - Vulnerable to ranged focus and kiting

- `scout_cavalry`
  - Fastest map control unit
  - Good at reaching archers and exposed villagers
  - Should avoid direct fights into spearmen

- `paladin`
  - Heavy cavalry power unit
  - Strong raw stats, good for breaking lines and chasing
  - Should still be countered efficiently by spearmen

- `archer`
  - High offensive pressure with longer sight
  - Good into slow melee units, especially spearmen
  - Vulnerable if cavalry closes distance

## Current Combat Rules

- Base damage should start from `attack - defense`, with a minimum floor such as `1`
- Counter bonuses are additive, not multiplicative
- Implemented counter bonuses:
  - `spearman` vs `scout_cavalry` or `paladin`: `+8`
  - `archer` vs `spearman`: `+4`
  - `scout_cavalry` vs `archer`: `+4`
- Attack range is separate from LOS
- Implemented attack ranges:
  - `archer`: `2`
  - all other units: `1`

## Movement Notes

- `MOVE_FAST` is for aggressive travel speed
- `MOVE_GUARD` is for slower, safer movement
- `scout_cavalry` is intentionally the fastest unit on the map
- `villager` is intentionally weak and slower when guarded

## Economic and Construction Roles

- `villager` is a special-purpose unit, not just a weak combat unit
- Only villagers should be allowed to gather resources from `orchard`, `deer`, `gold_mine`, and `stone_mine`
- Villagers can also gather `wood` from `forest`
- Only villagers should be allowed to build new structures
- Military units should not gather resources
- Military units should not construct buildings

This separation is important because the game loop depends on villagers to turn map control into economic growth.

## Current Build and Train Rules

- Villagers can build:
  - `barracks`
  - `stable`
  - `archery_range`
- `town_center` is not currently villager-buildable
- Production mapping:
  - `town_center` produces `villager`
  - `barracks` produces `infantry` and `spearman`
  - `stable` produces `scout_cavalry` and `paladin`
  - `archery_range` produces `archer`
- Production is queue-based, and each building spawns at most one queued unit per tick
- `barracks`, `stable`, and `archery_range` currently take `2` ticks to complete construction
- Unit production time:
  - `villager`: `1` tick
  - `infantry`: `1` tick
  - `spearman`: `1` tick
  - `archer`: `1` tick
  - `scout_cavalry`: `2` ticks
  - `paladin`: `2` ticks

## Vision Notes

- `scout_cavalry` and `archer` currently have the highest LOS at `4`
- `villager` has the lowest LOS at `2`
- Most frontline units use LOS `3`

## Implementation Source

The values in this document currently come from:

- `internal/entity/catalog.go`
- `internal/entity/unit.go`
- `internal/ticker/ticker.go`
- `internal/world/actions.go`

If unit stats are changed in code, update this file in the same change.
