# Agents of Dynasties — Open Questions and Risk Notes

This document lists the main uncertainties, edge cases, and design risks that still need validation or future refinement.

It is based on the current knowledge files:

- [API.md](./API.md)
- [MECHANISMS.md](./MECHANISMS.md)
- [UNITS.md](./UNITS.md)
- [ECONOMY.md](./ECONOMY.md)
- [MAP_GEN.md](./MAP_GEN.md)

The goal of this file is not to replace the design documents above. Its purpose is to make unclear or high-risk areas visible before implementation expands further.

## How to Read This File

Items in this document fall into three types:

- rules that are already suggested by the current docs, but may still need product validation
- rules that are not fully specified yet and would force implementers to guess
- areas where the documents are directionally aligned but still incomplete for API or gameplay purposes

## 1. Combat Outcome Questions

### 1.1 Mutual kill from simultaneous damage

Current direction:

- [MECHANISMS.md](./MECHANISMS.md) says combat damage is applied simultaneously
- that implies two units can kill each other in the same tick

Open question:

- is mutual destruction desirable for equal-unit fights

Example:

- two `infantry` attack each other
- if both are low enough that each attack reduces the other to `HP <= 0`, both die in the same tick

Why this needs attention:

- simultaneous resolution is fair and deterministic
- but it also creates a harsher combat model than many RTS players expect
- this directly affects how expensive trades feel in a `20`-population game

### 1.2 Ranged units and melee retaliation in the same tick

Current direction:

- `archer` attacks at range `2`
- most other units attack at range `1`
- all valid attacks for the combat phase are applied simultaneously

Open question:

- if an `archer` is at range `2`, does the melee unit simply fail to retaliate that tick, or should some guard/closing behavior happen automatically before next tick

Why this needs attention:

- this determines whether ranged units are merely positional or structurally dominant
- in a `10-second` tick game, one missed retaliation matters a lot

### 1.3 Focus fire and overkill waste

Current direction:

- targeting rules are deterministic
- simultaneous attacks are collected first and then applied

Open question:

- if multiple units attack a target that only needed one more hit to die, should all extra damage be wasted

Likely current answer:

- yes, because simultaneous resolution implies overkill waste

Why this needs attention:

- this strongly affects whether agents should split fire
- it changes the value of large ranged groups

### 1.4 Building combat behavior

Current direction:

- buildings have HP
- units may attack units or buildings

Open question:

- what happens if a building is destroyed on the same tick it would have spawned a unit or completed a queue event

Why this needs attention:

- this affects whether siege timing and base dives are intuitive
- implementation will otherwise need to guess whether production resolves before or after destruction

## 2. Movement and Collision Questions

### 2.1 More-than-two units contesting the same hex

Current direction:

- if two opposing units attempt to enter the same hex, both stay out

Open questions:

- what if three or more units from both teams all attempt to enter the same hex
- should the same "all fail" rule apply universally

Why this needs attention:

- once armies become larger, this edge case will happen naturally around choke points

### 2.2 Cross-swap movement

Open question:

- if unit A tries to move from hex X to Y, while unit B tries to move from Y to X in the same tick, is that allowed as a simultaneous swap or should both be blocked

Why this needs attention:

- without a rule, movement implementation may differ depending on iteration order

### 2.3 Friendly path blocking under multi-step movement

Current direction:

- friendly units block each other
- units may move multiple hexes per tick depending on speed

Open question:

- if a front friendly unit moves away earlier in the same movement phase, may a rear unit then use the newly freed hex in that same tick

Why this needs attention:

- this determines whether movement is effectively sequential, batched, or wave-based
- cavalry and fast movement are highly sensitive to this detail

### 2.4 MOVE_GUARD stop condition

Current direction:

- `MOVE_GUARD` moves first and then auto-engages if an enemy is in attack range

Open question:

- should `MOVE_GUARD` stop as soon as it first reaches attack range during movement, or should it always consume its full movement budget before combat selection

Why this needs attention:

- this changes frontline sticking behavior
- it also changes whether `MOVE_GUARD` is safer or merely slower

## 3. Economy Questions

### 3.1 Resource depletion visibility

Resolved:

- `/map` now includes `remaining` for resource nodes
- `/map` is refreshed over time rather than treated as immutable

Follow-up:

- validate whether current visibility is sufficient for agent planning during longer matches

### 3.2 Depleted node behavior

Resolved:

- when a resource node reaches `0`, the tile becomes `plain`
- depleted nodes are no longer valid `GATHER` targets

Follow-up:

- validate whether converting to `plain` gives clear enough player feedback in all UI modes

### 3.3 Population reservation on queued units

Current direction:

- [ECONOMY.md](./ECONOMY.md) says queued units reserve population immediately

Open questions:

- should reserved population be visible in `/state`
- can players cancel a queue and recover that reserved population
- if a building is destroyed, what happens to reserved population and queued unit costs

Why this needs attention:

- population is one of the most important strategic resources in this game
- hidden reservations would make agent planning error-prone

### 3.4 Gather-rate versus unit-cost balance

Current direction:

- `ECONOMY.md` proposes a first-pass parameter table

Open question:

- do the proposed gather rates and unit costs create too much snowball, especially for early villager production

Why this needs attention:

- small population games are highly sensitive to payback timing
- if villagers repay too quickly, greedy openings become dominant
- if villagers repay too slowly, economy becomes a trap

## 4. Production and Building Questions

### 4.1 Build completion timing

Current direction:

- villagers can build
- `ECONOMY.md` defines building cost, but not build time

Open question:

- are buildings created instantly when `BUILD` succeeds, or should they require one or more ticks of construction time

Why this needs attention:

- instant buildings simplify rules
- but they remove the tactical vulnerability that normally comes with expansion

### 4.2 Queue blocking when spawn tiles are full

Current direction:

- if a unit cannot spawn, it remains queued at the front

Open question:

- should the building continue progressing later queue items, or is the whole queue blocked by the front unit

Likely current answer:

- the whole queue is blocked

Why this needs attention:

- this affects whether spawn blocking is a tactical trick or a full denial mechanic

### 4.3 Town Center buildability

Current direction:

- existing docs emphasize one Town Center per team
- villager-buildable structures currently focus on military production buildings

Open question:

- should Town Centers ever be rebuildable after destruction

Why this needs attention:

- this is a major win-condition and comeback design decision

## 5. API and Information Exposure Questions

### 5.1 Economic information missing from API

Open question:

- should `/state` later expose:
  - current population used
  - reserved population
  - production queues
  - remaining resource on visible nodes

Why this needs attention:

- if these are not exposed, AI agents will need to reconstruct too much hidden state themselves

### 5.2 Command shape alignment

Current direction:

- some documentation historically described `PRODUCE` using `unit_id` as the building ID
- newer command models may separate `unit_id` and `building_id`

Open question:

- what is the final stable API shape for production commands

Why this needs attention:

- command schema drift is expensive for agent clients
- this should be resolved before external integrations grow

### 5.3 Visibility of persistent intent

Open question:

- should the API expose the currently active persistent command for a unit, such as a locked `ATTACK` target

Why this needs attention:

- without it, agents may not know whether a unit is continuing prior intent or is effectively idle

## 6. Balance and Meta Risks

### 6.1 Archer pressure under tick-based combat

Risk:

- in a long-tick game, range advantage can be disproportionately strong

Why:

- each combat phase is chunky
- losing one full retaliation window is costly

Possible improvement:

- verify whether `archer` damage, range, or cost needs further moderation

### 6.2 Cavalry mobility versus forest denial

Risk:

- cavalry cannot enter `forest`, but still has the highest speed elsewhere

Open question:

- is this enough terrain-based counterplay, or does cavalry still overperform on most generated maps

Why this needs attention:

- map structure and lane openness will heavily influence cavalry value

### 6.3 Villager vulnerability in a 20-pop game

Risk:

- each villager lost is a major percentage of the team's total economic capacity

Open question:

- is this desirable strategic tension, or does it make early raids too decisive

Why this needs attention:

- low-pop games can become swingy very quickly

## 7. Document Alignment Risks

### 7.1 "Specified" does not always mean "proven fun"

Some rules are now specified in the knowledge docs, but still need practical validation through playtesting:

- simultaneous mutual kills
- immediate resource payment on queue
- `carry-and-return` gathering with deposit beside `town_center`
- one worker per resource tile
- spawn blocking as a denial mechanic

Why this matters:

- a rule can be internally consistent and still produce bad gameplay

### 7.2 Knowledge docs still need future synchronization

As implementation grows, these files must stay aligned:

- [MECHANISMS.md](./MECHANISMS.md)
- [UNITS.md](./UNITS.md)
- [ECONOMY.md](./ECONOMY.md)
- [API.md](./API.md)

The highest-risk areas for drift are:

- command payload shape
- resource visibility in API responses
- production timing
- population accounting
- whether a rule is intended design or already implemented fact

## Suggested Next Review Topics

If this project continues refining design before more implementation work, the most important next questions to settle are:

1. Confirm whether simultaneous damage and mutual kills are the intended combat feel.
2. Define the final API shape for economy visibility and production commands.
3. Decide whether building construction should be instant or require time.
4. Validate the first-pass economy numbers against likely early-game build orders.
5. Clarify movement edge cases such as cross-swaps and chained friendly movement.

This file should be updated whenever a previously uncertain item becomes a locked rule.
