# Agents of Dynasties Map Generation

This document records the current map-generation rules and the implementation choices used to enforce them in code.

## Core Rules

The map is generated randomly, but the generator must always satisfy the following constraints:

1. `gold_mine`, `stone_mine`, `orchard`, and `deer` must not be adjacent to each other.
2. Each Town Center must have at least one `gold_mine`, one `stone_mine`, one `orchard`, and one `deer` within 5 hexes.
3. There must be at least three guaranteed `plain` routes between the two Town Centers: a top lane, a middle lane, and a bottom lane.
4. Mountains, lakes, and forests must not block any of the guaranteed Town Center lanes.
5. Strategic resources (`gold_mine`, `stone_mine`, `orchard`, `deer`) must not spawn inside forests or directly adjacent to forests.

## Current Implementation Strategy

The generator in `internal/world/world_gen.go` enforces the rules by generation order instead of relying on post-generation fixes.

### 1. Start from an all-plain map

The generator first fills the entire 20x15 map with `plain` tiles.

This matters because it creates a known valid baseline:

- every tile is passable
- no strategic resource starts inside a forest
- the corridor and starter areas can be carved deterministically before random terrain is added

### 2. Reserve protected zones before random terrain placement

The generator keeps a `reserved` coordinate set. Reserved tiles cannot be overwritten by random terrain clusters.

Reserved zones include:

- the cleared area around each Town Center
- the guaranteed plain corridor between Town Centers
- every tile used by guaranteed or extra strategic resources

This is the main technique that prevents later terrain placement from breaking earlier guarantees.

### 3. Carve the three Town Center lanes first

The generator creates three reserved `plain` paths between Team 1's Town Center and Team 2's Town Center:

- top lane
- middle lane
- bottom lane

Each lane is built by routing through a dedicated waypoint so the three lanes stay spatially separated through different parts of the map.

Every tile on every lane is marked as:

- `plain`
- reserved

Because forest, mountain, and lake clusters are only allowed to write onto non-reserved tiles, the corridor cannot be blocked later.

### 4. Place starter strategic resources before terrain clusters

For each Town Center, the generator places one of each strategic resource type:

- `gold_mine`
- `stone_mine`
- `orchard`
- `deer`

Placement rules:

- distance must be between 3 and 5 hexes from the Town Center
- the tile must still be `plain`
- the tile must not already be reserved
- the tile must not be adjacent to another strategic resource

After placement, the tile is marked reserved.

This guarantees nearby opening resources without allowing them to overlap each other or get replaced later.

### 5. Add random blocking terrain only outside reserved tiles

After protected zones are established, the generator places random clusters for:

- `forest`
- `mountain`
- `lake`

Cluster placement skips every reserved coordinate.

This is the mechanism that prevents:

- forests from swallowing starter resources
- forests from being painted directly next to protected starter resources
- mountains or lakes from cutting off the Town Center route
- random terrain from overwriting protected start areas

### 6. Add extra strategic resources only on plain tiles

After forests, mountains, and lakes are placed, the generator fills the remaining strategic-resource quotas.

A strategic resource can only be placed if:

- the tile is in bounds
- the tile is not reserved
- the tile is still `plain`
- no adjacent tile is `forest`
- no adjacent tile already contains a strategic resource

Requiring the tile to still be `plain` prevents strategic resources from spawning on forest tiles, and the forest-adjacency check prevents them from appearing visually embedded in forest clusters.

## Why This Avoids the Known Failure Modes

### Strategic resources touching each other

Prevented by checking the full radius-1 neighborhood before placing each strategic resource.

### Missing nearby opening resources

Prevented by explicitly placing one of each strategic resource type around each Town Center before general terrain generation.

### Town Centers being cut off by terrain

Prevented by carving and reserving three plain lanes before forest, mountain, and lake generation.

### Resources appearing inside forests

Prevented by only placing strategic resources on tiles whose current terrain is exactly `plain`, and by rejecting any strategic-resource placement adjacent to `forest`.

### Resources looking embedded inside forests

Prevented in two directions:

- strategic resources are never placed adjacent to forest
- forest cluster tiles are never painted adjacent to an existing strategic resource

## Validation and Tests

The world-generation tests in `internal/world/world_test.go` verify:

- each Town Center has all four strategic resource types within 5 tiles
- strategic resources are not adjacent
- strategic resources are not adjacent to forest
- a plain-only top lane exists between both Town Centers
- a plain-only middle lane exists between both Town Centers
- a plain-only bottom lane exists between both Town Centers
- the guarantees remain valid across multiple seeds

When map-generation rules change, update both:

- this document
- the world-generation tests
