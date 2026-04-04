# Agents of Dynasties — Core Mechanisms

This document defines the intended gameplay mechanisms for movement, occupancy, combat, gathering, and tick resolution.

It is the design source of truth for these systems, even where the current codebase has not yet fully implemented them.

## Current Baseline

These rules are written against the current repository state:

- Map size is `20x15`
- Coordinates use axial hex coordinates `(q, r)`
- The game advances in fixed ticks, with `10s` as the default server interval
- Commands are collected during a tick window and resolved at the next tick boundary
- Command submission uses last-command-wins semantics per actor

Important: this file defines the intended rules for implementation. The current code now covers the core movement, combat, gathering, construction progress, production timing, and resource depletion loop, but some advanced items in this file are still simplified or partial.

## Design Goals

The mechanisms in this file follow these priorities:

- Deterministic outcomes that are easy for AI agents to predict
- Clear, explicit rules over simulation realism
- Minimal hidden initiative or ordering advantage
- Low ambiguity when both teams issue commands in the same tick

## Terrain Access and Occupancy

### General Occupancy Rules

- Each hex may contain at most one unit.
- Friendly units may not stack.
- Enemy units may not stack.
- Units may not enter a hex occupied by a building.
- Buildings occupy exactly one hex.
- Blocking terrain is static after map generation, but exhausted resource tiles can change into `plain`.

### Terrain Access by Unit Type

The following table defines whether a unit may enter a terrain type.

| Unit Kind | Plain | Forest | Gold Mine | Stone Mine | Orchard | Deer | Mountain | Lake |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `villager` | Yes | Yes | Yes | Yes | Yes | Yes | No | No |
| `infantry` | Yes | Yes | Yes | Yes | Yes | Yes | No | No |
| `spearman` | Yes | Yes | Yes | Yes | Yes | Yes | No | No |
| `archer` | Yes | Yes | Yes | Yes | Yes | Yes | No | No |
| `scout_cavalry` | Yes | No | Yes | Yes | Yes | Yes | No | No |
| `paladin` | Yes | No | Yes | Yes | Yes | Yes | No | No |

### Terrain Notes

- `mountain` and `lake` are impassable to all units.
- `forest` is passable to non-cavalry units only.
- Resource tiles are real map hexes, not abstract nodes.
- A unit standing on a resource tile still occupies that tile normally.

## Tick Resolution Model

Each tick is resolved in a fixed deterministic order.

### Resolution Order

1. Freeze the command queue at the tick boundary.
2. Sort queued commands in deterministic actor order and merge them with any persistent actor intent.
3. Validate commands against the current world state.
4. Resolve movement.
5. Resolve combat.
6. Resolve economy actions.
7. Remove dead entities and finalize state changes.
8. Recompute LOS for the next visible state snapshot.

### Persistent and Non-Persistent Commands

- `ATTACK` is persistent across ticks.
- A persistent `ATTACK` remains active until:
  - the target dies
  - the target becomes invalid
  - the actor can no longer legally continue the attack
  - the actor receives a new command
- `MOVE_FAST`, `MOVE_GUARD`, `GATHER`, `BUILD`, and `PRODUCE` are resolved from the latest command state and may be replaced by a newer command before the next tick.

## Movement Resolution

### Basic Movement Rules

- `MOVE_FAST` uses the unit's `SpeedFast`.
- `MOVE_GUARD` uses the unit's `SpeedGuard`.
- A unit may move up to its speed in hexes during the movement phase.
- Movement follows a legal path toward the target coordinate.
- A step is illegal if the destination hex is:
  - out of bounds
  - impassable for that unit type
  - occupied by a building
  - occupied by another unit
  - simultaneously contested by an enemy unit trying to enter the same hex

### Path Blocking

- If a unit cannot complete its full intended movement, it stops at the last legal hex reached in that tick.
- If the next step on the path is illegal, the unit does not continue attempting alternative steps during that same tick unless pathfinding explicitly selected a different legal route before movement began.

### Same-Hex Conflict Rule

When two opposing units attempt to enter the same hex during the same movement phase:

- neither unit enters that hex
- both units remain in their previous legal positions for that step
- no hidden priority is granted by speed, command submission time, or team

This rule exists to keep simultaneous movement deterministic and fair.

### MOVE_FAST vs MOVE_GUARD

`MOVE_FAST`

- prioritizes travel only
- never auto-engages enemies
- if the move finishes with an enemy in range, the unit still does not attack unless it has a valid persistent `ATTACK`

`MOVE_GUARD`

- uses guarded movement speed
- after movement is resolved, if an enemy is within attack range, the unit enters combat targeting instead of continuing to prioritize its move target
- this allows guarded movement to naturally convert into frontline contact

## Combat Resolution

Combat is resolved after all movement for the tick has completed.

### Attack Range

Attack range is separate from LOS.

| Unit Kind | Attack Range |
| --- | --- |
| `villager` | 1 |
| `infantry` | 1 |
| `spearman` | 1 |
| `scout_cavalry` | 1 |
| `paladin` | 1 |
| `archer` | 2 |

LOS remains an information and visibility rule, not an attack range rule.

### How a Unit Selects a Target

For `ATTACK target_id`:

- the unit keeps pursuing that target across ticks until the command becomes invalid or is replaced
- if the target is in attack range during the combat phase, the unit attacks it

For `MOVE_GUARD` auto-engagement:

- if multiple enemies are valid targets, choose the nearest target
- if distance is tied, choose the target with the lowest current HP
- if still tied, choose the lowest entity ID

### Damage Model

Base damage is:

`max(1, attacker.attack - defender.defense + counter_bonus)`

Counter bonuses are additive, not multiplicative.

### Counter Relationships

The initial counter rules are:

- `spearman` attacking `scout_cavalry`: `+8`
- `spearman` attacking `paladin`: `+8`
- `archer` attacking `spearman`: `+4`
- `scout_cavalry` attacking `archer`: `+4`

All other matchups use `+0` counter bonus unless future balance changes explicitly define otherwise.

### Simultaneous Damage

All valid attacks in the combat phase are collected first, then applied simultaneously.

This means:

- a unit can still deal its attack in the current tick even if it is reduced to `0` HP by another attack in the same combat phase
- combat outcomes do not depend on internal iteration order

### Death and Removal

- Any unit or building reduced to `HP <= 0` is considered dead.
- Dead entities are removed after simultaneous damage has been applied.
- Removed entities do not persist into the next tick.

## Economy Actions

### Gathering

- Only `villager` may use `GATHER`.
- The villager must stand on the target resource tile to gather from it.
- Gathered resources are carried by the villager.
- The villager deposits carried resources by using `GATHER` while adjacent to a friendly `town_center`.
- Valid gathering targets are:
  - `forest`
  - `gold_mine`
  - `stone_mine`
  - `orchard`
  - `deer`
- A military unit may not gather.
- Because stacking is disallowed, a gathering villager also blocks that resource hex from other units.

### Building

- Only `villager` may use `BUILD`.
- A building may only be placed on a legal, unoccupied target hex.
- A build command fails if the target hex is occupied, impassable, or otherwise invalid for building placement.
- A valid build starts an under-construction building that completes after its build time in ticks.

### Production

- `PRODUCE` is issued by a building actor.
- A building must be complete before it can accept `PRODUCE`.
- A queued unit reserves population immediately once the enqueue succeeds.
- Produced units must spawn on a legal adjacent hex.
- If no legal adjacent hex is available, the produced unit remains queued and does not appear until a future tick with valid space.
- Production time is tracked per queued unit kind.

## Edge-Case Rules

### Enemy Blocking

- A unit may not path through enemy-occupied hexes.
- A unit may not end movement on an enemy-occupied hex.
- Entering attack range does not move a unit into the defender's hex; combat occurs across range as defined above.

### Friendly Blocking

- Friendly units block movement just like enemy units.
- There is no friendly pass-through or temporary overlap.

### Invalid Commands at Resolution Time

- If a command was valid when submitted but invalid at tick resolution, it is ignored for that tick.
- Examples:
  - target died before resolution
  - target hex became occupied
  - actor no longer exists
  - actor can no longer legally perform that action

## Test Scenarios

Any implementation based on this document should cover at least these cases:

- cavalry cannot enter `forest`
- non-cavalry units can enter `forest`
- no unit can enter `mountain` or `lake`
- no unit can enter a building hex
- same-team units cannot stack
- opposing units trying to move into the same hex both fail to enter it
- `MOVE_FAST` does not auto-engage
- `MOVE_GUARD` auto-engages after movement
- `ATTACK` persists across ticks until invalidated or replaced
- melee units attack at range `1`
- `archer` attacks at range `2`
- simultaneous combat allows mutual kills in the same tick
- villagers must stand on the resource hex to gather
- produced units stay queued when no adjacent spawn hex is available

## Relationship to Other Knowledge Files

- [API.md](./API.md) defines the agent-facing API contract
- [UNITS.md](./UNITS.md) defines unit roles, stats, and intended counters
- [MAP_GEN.md](./MAP_GEN.md) defines terrain generation guarantees

If gameplay rules change, update this file together with any affected API or unit documentation.
