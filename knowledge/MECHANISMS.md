# Agents of Dynasties — Core Mechanisms

This document defines the intended gameplay mechanisms for movement, occupancy, combat, gathering, and tick resolution.

It is the design source of truth for these systems, even where the current codebase has not yet fully implemented them.

## Current Baseline

These rules are written against the current repository state:

- Map size is `20x15`
- Coordinates use odd-r offset hex coordinates `(q, r)`
- The game advances in fixed ticks, with `10s` as the default server interval
- Commands are collected during a tick window and resolved at the next tick boundary
- Command submission uses last-command-wins semantics per actor for the pending queue
- Units now keep a persistent action status across ticks

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
2. Sort queued commands in deterministic actor order.
3. Apply queued commands onto persistent actor status.
4. Resolve movement from the current actor status.
5. Resolve combat from the current actor status.
6. Resolve economy actions from the current actor status.
7. Remove dead entities and finalize state changes.
8. Recompute LOS for the next visible state snapshot.

### Persistent and Non-Persistent Commands

- `MOVE_FAST`, `MOVE_GUARD`, `ATTACK`, `GATHER`, and `BUILD` are persistent unit commands.
- A persistent unit command remains active until:
  - the command completes
  - the target becomes permanently invalid
  - the actor dies
  - the actor receives a new command
  - the actor receives `STOP`
- `PRODUCE` is not a unit status; it remains a building queue action.

## Movement Resolution

### Basic Movement Rules

- `MOVE_FAST` uses the unit's `SpeedFast`.
- `MOVE_GUARD` uses the unit's `SpeedGuard`.
- A unit may move up to its speed in hexes during the movement phase.
- Movement follows a shortest legal path toward the target coordinate or approach hex.
- Pathfinding may route around `lake`, `mountain`, forest restrictions, and other occupied hexes if a legal path exists.
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

- the unit keeps pursuing that target across ticks until the command becomes invalid, is replaced, or is stopped
- if the target is out of range, the attacker keeps closing distance at guarded speed
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
- `GATHER` stores a specific target resource tile.
- The villager automatically moves to that target resource tile if needed.
- Gathered resources are carried by the villager.
- The villager automatically returns to a friendly `town_center` to deposit carried resources.
- If multiple deposit-adjacent hexes are available, the villager prefers the one with the lowest remaining round-trip path cost between its current position, the deposit hex, and the bound resource node.
- After depositing, the villager returns to the same resource target and repeats until interrupted or the node is exhausted.
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
- `BUILD` stores a target hex and building kind.
- The villager automatically moves adjacent to the build target if needed.
- If no site exists yet and the target is valid, the villager starts the construction site.
- Once the site exists, the villager keeps building until the structure completes, unless interrupted or invalidated.

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

- If a command was valid when submitted but invalid at tick resolution, it either stays pending as a persistent status that will retry later, or is cleared if the target became permanently invalid.
- Examples:
  - target died before resolution
  - target hex became permanently invalid for building
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
- `STOP` clears the unit's persistent status
- long-distance movement continues across ticks without resubmission
- `MOVE_FAST` does not auto-engage
- `MOVE_GUARD` auto-engages after movement
- `ATTACK` persists across ticks until invalidated or replaced
- `ATTACK` can close distance across ticks before attacking
- melee units attack at range `1`
- `archer` attacks at range `2`
- simultaneous combat allows mutual kills in the same tick
- `GATHER` automatically shuttles between a resource node and a friendly `town_center`
- `BUILD` automatically moves the villager to the site and persists until completion
- produced units stay queued when no adjacent spawn hex is available

## Relationship to Other Knowledge Files

- [API.md](./API.md) defines the agent-facing API contract
- [UNITS.md](./UNITS.md) defines unit roles, stats, and intended counters
- [MAP_GEN.md](./MAP_GEN.md) defines terrain generation guarantees

If gameplay rules change, update this file together with any affected API or unit documentation.
