# Agents of Dynasties — Economy Design

This document defines the intended economy model for Agents of Dynasties.

It focuses on a small-scale strategy game with a hard population cap of `20` units per team. Because the game resolves in tick steps instead of continuous real-time simulation, the economy must stay readable, deterministic, and easy for AI agents to plan around.

This file now records the economy rules that are adopted in the current repository, plus the balance tables that were aligned into `internal/entity/catalog.go`.

## Adoption Review

The earlier draft in this file had two rules that were not kept as-is:

- The draft proposed direct stockpile deposit on gather. The current game keeps `carry-and-return` gathering because that behavior is already part of `UNITS.md`, `MECHANISMS.md`, the API payloads, and the frontend state display.
- The draft proposed leaving a depleted resource tile on the map with `0` yield. The current game converts a depleted resource tile into `plain`, because the live `/map` API and frontend now already depend on terrain changing when a node is exhausted.

The rest of the economy model was safe to adopt and is now applied in code:

- `20` population cap
- queued production reserving population
- deterministic per-tick gather amounts
- finite resource nodes
- the balance tables for gathering, unit cost, building cost, and production time

## Why the Economy Needs Explicit Design

In a traditional RTS, economy can rely on fine-grained worker routing, carry trips, drop-off timing, and constant micro-adjustment. That is a bad fit for this project for three reasons:

- commands resolve on fixed tick intervals instead of frame-by-frame
- the main player surface is an API used by agents, so rules must be easy to reason about
- each team only has `20` population, so every villager is a meaningful strategic tradeoff

Because of this, the economy should optimize for:

- clear opportunity cost between economy and army
- resource scarcity that creates map pressure
- predictable per-tick income
- simple accounting that can be exposed cleanly through the API

## Core Economic Principles

### 1. Population Is the Main Strategic Constraint

- Each team has a hard cap of `20` population.
- Only living and reserved unit slots count toward population.
- Buildings do not consume population.
- Villagers consume population just like military units.

Why:

- this keeps the game strategically tight
- a player cannot scale economy infinitely behind a static defense
- choosing more villagers means accepting a smaller army for some period of time

### 2. Economy Should Be Strong but Not Free

- Villagers are the only units that gather resources.
- Villagers are also the only units that construct new production buildings.
- Early villagers should pay back their cost if kept alive, but not so fast that full eco greed becomes automatic.

Why:

- this makes villager survival and positioning matter
- economy and expansion become visible, attackable strategic commitments
- the player must decide between short-term army and longer-term production

### 3. Resource Nodes Should Be Finite

- Gatherable map nodes have finite remaining yield.
- When a node reaches `0`, the tile becomes `plain`.

Why:

- map control should matter over time
- finite nodes force movement, contest, and expansion
- exhausted tiles should stop looking like active resource nodes in both the API and the frontend

### 4. Tick Income Must Be Deterministic

- Gathering uses fixed per-tick amounts per villager.
- Villagers carry gathered resources first, then deposit them beside a friendly `town_center`.
- The actual amount gathered from a node is `min(gather_rate, node_remaining_amount, carry_capacity)`.

Why:

- this fits the existing tick-based game loop
- it keeps income predictable even with `carry-and-return`
- agents can plan expected income exactly from known worker assignments and carry state

## Economy Model

### Starting State

Each team begins with:

- `1` Town Center
- `2` Villagers
- starting stockpile:
  - `Food: 200`
  - `Gold: 100`
  - `Stone: 100`
  - `Wood: 200`

Why:

- the opening should start immediately without a dead economic phase
- teams can choose between fast military teching, villager growth, or early infrastructure

### Population Accounting

Population is consumed by:

- living villagers
- living military units
- queued units whose production has already reserved a slot

Population is not consumed by:

- buildings
- dead units

Why reserve population on queue:

- it prevents hidden overproduction
- it gives agents a stable "future army size" signal
- it keeps queue behavior deterministic under a small `20`-pop cap

### Gathering Rules

- Only `villager` may gather.
- A villager must stand on the current resource tile.
- One gatherer per tile is naturally enforced by single-unit occupancy.
- Gathered resources are first stored on the villager.
- A villager deposits carried resources by using `GATHER` while adjacent to a friendly `town_center`.
- If a node has less than one full gather tick remaining, the villager harvests only the remainder.

Why this design:

- standing on the tile matches the previously defined occupancy model
- one worker per tile makes resource map geometry matter
- `carry-and-return` preserves worker positioning and raiding value without adding free-form path micro

### Spending Rules

Resource cost is committed as soon as the economic action actually enters world state:

- `BUILD`: pay when the build action starts and the construction site is created
- `PRODUCE`: pay when the unit is successfully added to a building queue

Command validation also accounts for already pending same-window `BUILD` and `PRODUCE` commands from the same team, so agents cannot over-commit resources before the next tick.

If the team lacks resources, the action fails and nothing is deducted.

Why:

- immediate payment avoids temporary overspending
- queues become stable commitments instead of soft intentions
- this is easier for agent planning than deferred payment on completion

### Production Rules

- Each production building has its own FIFO queue.
- A building processes at most one unit completion event at a time.
- A queued unit reserves population immediately once it is successfully enqueued.
- If no legal adjacent spawn tile is free when production completes, the unit stays at the front of the queue until a future tick frees space.

Why:

- per-building queues keep throughput predictable
- reserved population prevents queue abuse
- blocked spawns create tactical value around production structures

## Recommended Parameter Tables

These values are intended as the first balancing pass. They should later live in a dedicated parameter source such as Go constants, JSON, or YAML.

### Global Economy Parameters

| Parameter | Recommended Value | Why |
| --- | ---: | --- |
| Tick duration | `10s` default | Server default; configurable via `--tick` for faster local iteration |
| Population cap | `20` | Keeps the game small, readable, and tactical |
| Starting villagers | `2` | Gives each side an active opening without immediate saturation |
| Starting food | `200` | Supports early villager or first military investment |
| Starting wood | `200` | Enables early building decisions |
| Starting gold | `100` | Allows access to advanced unit paths without immediate map lock |
| Starting stone | `100` | Supports building timing choices without making expansion free |

### Gather Rates Per Villager Per Tick

| Resource Source | Resource Gained / Tick | Why |
| --- | ---: | --- |
| `forest` | `20 wood` | Wood should be the most reliable structural resource |
| `orchard` | `18 food` | Safe, steady food source |
| `deer` | `24 food` | Higher burst food to reward map presence |
| `gold_mine` | `12 gold` | Gold should feel more constrained than food/wood |
| `stone_mine` | `10 stone` | Stone should be the slowest strategic resource |

Design intent:

- `food` supports population growth and core unit flow
- `wood` supports infrastructure and ranged/unit production
- `gold` gates higher-value military power
- `stone` slows down unchecked structure spam

### Resource Node Capacity Per Tile

| Terrain | Total Remaining Resource | Why |
| --- | ---: | --- |
| `forest` | `300 wood` | Long-lasting but still exhaustible |
| `orchard` | `240 food` | Stable food income for sustained eco |
| `deer` | `120 food` | Fast but quickly depleted forward food |
| `gold_mine` | `400 gold` | Enough for multiple advanced units, but still contestable |
| `stone_mine` | `350 stone` | Supports some building investment, not endless turtling |

Design intent:

- `deer` is the fastest early food but the least durable
- `orchard` is safer and longer-term
- `forest` lasts long enough to support baseline infrastructure
- `gold` and `stone` are intentionally finite enough to create map pressure

### Building Costs

| Building | Food | Wood | Gold | Stone | Why |
| --- | ---: | ---: | ---: | ---: | --- |
| `barracks` | `0` | `120` | `0` | `60` | Earliest military building, mostly a wood commitment |
| `stable` | `0` | `140` | `20` | `80` | Cavalry tech should ask for a broader resource base |
| `archery_range` | `0` | `130` | `0` | `60` | Similar timing to barracks, but slightly wood-heavier |

Design intent:

- wood is the main "build infrastructure" cost
- stone slows down building spam
- stable is deliberately more expensive because cavalry has higher mobility and raid value

### Unit Production Costs

| Unit | Food | Wood | Gold | Stone | Population | Why |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| `villager` | `50` | `0` | `0` | `0` | `1` | Cheap enough to scale, expensive enough to be punishable |
| `infantry` | `60` | `20` | `0` | `0` | `1` | Baseline frontline unit |
| `spearman` | `50` | `20` | `10` | `0` | `1` | Accessible anti-cavalry answer |
| `archer` | `40` | `30` | `15` | `0` | `1` | Wood-heavy ranged pressure with light gold gating |
| `scout_cavalry` | `70` | `20` | `20` | `0` | `1` | Mobility should cost more than infantry |
| `paladin` | `90` | `0` | `45` | `0` | `1` | Premium power unit with strong gold dependence |

Design intent:

- villagers should be a meaningful but recoverable investment
- infantry should be the easiest military unit to mass
- spearman should be available before cavalry snowballs
- archers should pressure wood and gold together
- paladin should be strong, but the economy should visibly feel the cost

### Unit Production Time

Production time is expressed in ticks.

| Unit | Producer | Time | Why |
| --- | --- | ---: | --- |
| `villager` | `town_center` | `1` tick | Economy growth should be responsive |
| `infantry` | `barracks` | `1` tick | Core army flow should stay simple |
| `spearman` | `barracks` | `1` tick | Defensive response must not be too slow |
| `archer` | `archery_range` | `1` tick | Ranged tech should be available on time |
| `scout_cavalry` | `stable` | `2` ticks | Mobility deserves longer commitment |
| `paladin` | `stable` | `2` ticks | Premium unit should be both costly and slower to field |

## Strategic Consequences of This Economy

With this structure, the intended game flow is:

### Early Game

- open with a small number of villagers because each extra worker costs population
- contest nearby `deer`, `orchard`, and `forest`
- decide whether to spend wood on military production or economy expansion

Why this is good:

- the first few ticks create meaningful branch decisions
- both greed and aggression are viable

### Mid Game

- teams start feeling the limit of nearby safe resources
- gold and stone become more important because stronger units and building choices now matter
- raiding villagers has high leverage because total population is low

Why this is good:

- map control matters without needing a huge map
- attacks on economy are meaningful without being instantly decisive

### Late Game

- the hard `20` pop cap pushes teams to rebalance villagers versus army
- depleted nodes create relocation pressure
- stronger units are available, but only if the economy has been protected well enough to fund them

Why this is good:

- endgame is driven by tradeoffs, not just infinite production
- the player is rewarded for timing, denial, and efficient composition

## What Should Later Become Configurable

The following values should eventually be centralized in a parameter table rather than embedded across gameplay code:

- starting resources
- population cap
- gather rates by terrain type
- node capacity by terrain type
- building costs
- unit costs
- production time
- any future build time values

Why:

- balancing should not require rewriting core logic
- AI experiments can run against multiple economic profiles
- the game can later support "standard", "fast", or "scarce resources" modes cleanly

## Relationship to Other Knowledge Files

- [MECHANISMS.md](./MECHANISMS.md) defines tick resolution, occupancy, combat timing, and core interaction rules
- [UNITS.md](./UNITS.md) defines unit roles and stat baselines
- [MAP_GEN.md](./MAP_GEN.md) defines resource distribution on the map
- [API.md](./API.md) defines what economic state should later be visible to agents

If economy rules change, update this file together with any affected unit, mechanism, and API documentation.
