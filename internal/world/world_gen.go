package world

import (
	"math/rand"
	"slices"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
)

type clusterSpec struct {
	kind    terrain.Type
	count   int
	radius  int
}

var clusters = []clusterSpec{
	{terrain.Forest, 8, 2},
	{terrain.Mountain, 4, 2},
	{terrain.Lake, 3, 1},
}

var strategicResourceTargets = map[terrain.Type]int{
	terrain.GoldMine:  4,
	terrain.StoneMine: 4,
	terrain.Orchard:   5,
	terrain.Deer:      4,
}

var strategicResourceKinds = []terrain.Type{
	terrain.GoldMine,
	terrain.StoneMine,
	terrain.Orchard,
	terrain.Deer,
}

var laneWaypoints = [][]hex.Coord{
	{{Q: 10, R: 1}},
	{{Q: 10, R: 7}},
	{{Q: 10, R: 13}},
}

// generate fills w.Tiles and populates starting entities for both teams.
func generate(w *World, seed int64) {
	rng := rand.New(rand.NewSource(seed))

	tc1Pos := hex.Coord{Q: 4, R: 4}
	tc2Pos := hex.Coord{Q: 15, R: 10}

	// Fill with plains.
	for q := 0; q < hex.GridWidth; q++ {
		for r := 0; r < hex.GridHeight; r++ {
			c := hex.Coord{Q: q, R: r}
			w.Tiles[c] = terrain.Tile{Coord: c, Terrain: terrain.Plain}
		}
	}

	reserved := make(map[hex.Coord]bool)
	clearAndReserveArea(w, reserved, tc1Pos, 2)
	clearAndReserveArea(w, reserved, tc2Pos, 2)
	carvePlainLanes(w, reserved, tc1Pos, tc2Pos)

	for _, tc := range []hex.Coord{tc1Pos, tc2Pos} {
		placeStartingResourceRing(w, rng, reserved, tc)
	}

	// Place random blocking terrain outside reserved start/corridor/resource areas.
	for _, spec := range clusters {
		for i := 0; i < spec.count; i++ {
			center := randomCoord(rng)
			for _, c := range hex.Circle(center, spec.radius) {
				if hex.InBounds(c) && !reserved[c] && canPlaceClusterTile(w, spec.kind, c) {
					w.Tiles[c] = terrain.Tile{Coord: c, Terrain: spec.kind}
				}
			}
		}
	}

	placeExtraStrategicResources(w, rng, reserved)

	// Spawn Team 1 starting entities.
	tc1 := entity.NewBuilding(w.nextID(), entity.Team1, entity.KindTownCenter, tc1Pos)
	w.addBuilding(tc1)
	spawnStartingVillagers(w, entity.Team1, tc1Pos, 2)

	// Spawn Team 2 starting entities.
	tc2 := entity.NewBuilding(w.nextID(), entity.Team2, entity.KindTownCenter, tc2Pos)
	w.addBuilding(tc2)
	spawnStartingVillagers(w, entity.Team2, tc2Pos, 2)
}

func clearAndReserveArea(w *World, reserved map[hex.Coord]bool, center hex.Coord, radius int) {
	for _, c := range hex.Circle(center, radius) {
		if hex.InBounds(c) {
			reserved[c] = true
			w.Tiles[c] = terrain.Tile{Coord: c, Terrain: terrain.Plain}
		}
	}
}

func carvePlainLanes(w *World, reserved map[hex.Coord]bool, from, to hex.Coord) {
	for _, waypoints := range laneWaypoints {
		cur := from
		for _, waypoint := range append(waypoints, to) {
			cur = carvePlainPath(w, reserved, cur, waypoint)
		}
	}
}

func carvePlainPath(w *World, reserved map[hex.Coord]bool, from, to hex.Coord) hex.Coord {
	cur := from
	for {
		reserved[cur] = true
		w.Tiles[cur] = terrain.Tile{Coord: cur, Terrain: terrain.Plain}
		if cur == to {
			return cur
		}

		next := cur
		bestDist := hex.Distance(cur, to)
		for _, candidate := range cur.Neighbors() {
			if !hex.InBounds(candidate) {
				continue
			}
			if dist := hex.Distance(candidate, to); dist < bestDist {
				bestDist = dist
				next = candidate
			}
		}
		if next == cur {
			panic("failed to carve plain path")
		}
		cur = next
	}
}

func placeStartingResourceRing(w *World, rng *rand.Rand, reserved map[hex.Coord]bool, tc hex.Coord) {
	candidates := hex.Circle(tc, 5)
	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	for _, kind := range strategicResourceKinds {
		placed := false
		for _, c := range candidates {
			if hex.Distance(tc, c) < 3 || hex.Distance(tc, c) > 5 {
				continue
			}
			if canPlaceStrategicResource(w, reserved, c) {
				placeStrategicResource(w, reserved, c, kind)
				placed = true
				break
			}
		}
		if !placed {
			panic("failed to place starting strategic resources near town center")
		}
	}
}

func placeExtraStrategicResources(w *World, rng *rand.Rand, reserved map[hex.Coord]bool) {
	for _, kind := range strategicResourceKinds {
		need := strategicResourceTargets[kind] - countTerrain(w, kind)
		for i := 0; i < need; i++ {
			placed := false
			for attempts := 0; attempts < 200; attempts++ {
				c := randomCoord(rng)
				if canPlaceStrategicResource(w, reserved, c) {
					placeStrategicResource(w, reserved, c, kind)
					placed = true
					break
				}
			}
			if !placed {
				panic("failed to place extra strategic resources")
			}
		}
	}
}

func canPlaceStrategicResource(w *World, reserved map[hex.Coord]bool, c hex.Coord) bool {
	if !hex.InBounds(c) || reserved[c] {
		return false
	}
	if w.Tiles[c].Terrain != terrain.Plain {
		return false
	}
	for _, neighbor := range hex.Circle(c, 1) {
		if neighbor == c || !hex.InBounds(neighbor) {
			continue
		}
		if w.Tiles[neighbor].Terrain == terrain.Forest {
			return false
		}
		if isStrategicResource(w.Tiles[neighbor].Terrain) {
			return false
		}
	}
	return true
}

func canPlaceClusterTile(w *World, kind terrain.Type, c hex.Coord) bool {
	if kind != terrain.Forest {
		return true
	}

	for _, neighbor := range hex.Circle(c, 1) {
		if !hex.InBounds(neighbor) {
			continue
		}
		if isStrategicResource(w.Tiles[neighbor].Terrain) {
			return false
		}
	}
	return true
}

func placeStrategicResource(w *World, reserved map[hex.Coord]bool, c hex.Coord, kind terrain.Type) {
	reserved[c] = true
	w.Tiles[c] = terrain.Tile{Coord: c, Terrain: kind}
}

func spawnStartingVillagers(w *World, team entity.Team, tcPos hex.Coord, count int) {
	occupied := occupiedCoords(w)
	for _, pos := range findOpenSpawnCoords(w, tcPos, occupied, count) {
		u := entity.NewUnit(w.nextID(), team, entity.KindVillager, pos)
		w.addUnit(u)
	}
}

func occupiedCoords(w *World) map[hex.Coord]bool {
	occupied := make(map[hex.Coord]bool, len(w.Buildings)+len(w.Units))
	for _, b := range w.Buildings {
		occupied[b.Position()] = true
	}
	for _, u := range w.Units {
		occupied[u.Position()] = true
	}
	return occupied
}

func findOpenSpawnCoords(w *World, origin hex.Coord, occupied map[hex.Coord]bool, count int) []hex.Coord {
	var out []hex.Coord
	for radius := 1; radius <= 3 && len(out) < count; radius++ {
		for _, c := range hex.Ring(origin, radius) {
			if !hex.InBounds(c) || occupied[c] {
				continue
			}
			tile, ok := w.Tiles[c]
			if !ok || !tile.Terrain.Passable() {
				continue
			}
			occupied[c] = true
			out = append(out, c)
			if len(out) == count {
				return out
			}
		}
	}
	panic("failed to place starting villagers near town center")
}

func countTerrain(w *World, kind terrain.Type) int {
	count := 0
	for _, tile := range w.Tiles {
		if tile.Terrain == kind {
			count++
		}
	}
	return count
}

func isStrategicResource(kind terrain.Type) bool {
	return slices.Contains(strategicResourceKinds, kind)
}

func randomCoord(rng *rand.Rand) hex.Coord {
	return hex.Coord{
		Q: rng.Intn(hex.GridWidth),
		R: rng.Intn(hex.GridHeight),
	}
}
