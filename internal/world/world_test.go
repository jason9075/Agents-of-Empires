package world

import (
	"fmt"
	"testing"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
)

func TestNewWorld_TileCount(t *testing.T) {
	w := NewWorld(42)
	tiles := w.AllTiles()
	want := hex.GridWidth * hex.GridHeight // 20×15 = 300
	if len(tiles) != want {
		t.Fatalf("expected %d tiles, got %d", want, len(tiles))
	}
}

func TestNewWorld_StartingResources(t *testing.T) {
	w := NewWorld(42)
	for _, team := range []entity.Team{entity.Team1, entity.Team2} {
		res := w.GetResources(team)
		if res.Food != StartingResources.Food {
			t.Errorf("team %d food = %d, want %d", team, res.Food, StartingResources.Food)
		}
	}
}

func TestNewWorld_StartingUnits(t *testing.T) {
	w := NewWorld(42)
	for _, team := range []entity.Team{entity.Team1, entity.Team2} {
		units := w.UnitsByTeam(team)
		if len(units) != 2 {
			t.Errorf("team %d: expected 2 starting villagers, got %d", team, len(units))
		}
		tc := mustTownCenterPos(t, w, team)
		seen := map[hex.Coord]bool{}
		for _, u := range units {
			if !hex.InBounds(u.Position()) {
				t.Fatalf("team %d villager out of bounds at %v", team, u.Position())
			}
			tile, ok := w.Tile(u.Position())
			if !ok || !tile.Terrain.Passable() {
				t.Fatalf("team %d villager spawned on non-passable tile %v", team, u.Position())
			}
			if hex.Distance(tc, u.Position()) > 3 {
				t.Fatalf("team %d villager spawned too far from town center: tc=%v villager=%v", team, tc, u.Position())
			}
			if seen[u.Position()] {
				t.Fatalf("team %d has duplicate villager spawn at %v", team, u.Position())
			}
			seen[u.Position()] = true
		}
	}
}

func TestNewWorld_StartingBuildings(t *testing.T) {
	w := NewWorld(42)
	for _, team := range []entity.Team{entity.Team1, entity.Team2} {
		buildings := w.BuildingsByTeam(team)
		if len(buildings) != 1 {
			t.Errorf("team %d: expected 1 town center, got %d", team, len(buildings))
		}
		if buildings[0].Kind() != entity.KindTownCenter {
			t.Errorf("team %d: expected TownCenter, got %v", team, buildings[0].Kind())
		}
	}
}

func TestAllTiles_Ordered(t *testing.T) {
	w := NewWorld(42)
	tiles := w.AllTiles()
	for i := 1; i < len(tiles); i++ {
		prev := tiles[i-1].Coord.Q*hex.GridHeight + tiles[i-1].Coord.R
		cur := tiles[i].Coord.Q*hex.GridHeight + tiles[i].Coord.R
		if cur <= prev {
			t.Fatalf("tiles not sorted at index %d: %v >= %v", i, tiles[i-1].Coord, tiles[i].Coord)
		}
	}
}

func TestVisibleTo_OwnEntitiesAlwaysVisible(t *testing.T) {
	w := NewWorld(42)
	ownUnits, ownBuildings, _, _ := w.VisibleTo(entity.Team1)
	if len(ownUnits) == 0 {
		t.Error("expected own units to be visible")
	}
	if len(ownBuildings) == 0 {
		t.Error("expected own buildings to be visible")
	}
}

func TestNewWorld_StrategicResourcesNearTownCenters(t *testing.T) {
	w := NewWorld(42)

	for _, team := range []entity.Team{entity.Team1, entity.Team2} {
		tc := mustTownCenterPos(t, w, team)
		for _, kind := range []terrain.Type{terrain.GoldMine, terrain.StoneMine, terrain.Orchard, terrain.Deer} {
			if !hasTerrainWithin(w, tc, 5, kind) {
				t.Fatalf("team %d town center at %v missing %s within 5 tiles", team, tc, kind.String())
			}
		}
	}
}

func TestNewWorld_StrategicResourcesAreNotAdjacent(t *testing.T) {
	w := NewWorld(42)

	var resources []terrain.Tile
	for _, tile := range w.AllTiles() {
		switch tile.Terrain {
		case terrain.GoldMine, terrain.StoneMine, terrain.Orchard, terrain.Deer:
			resources = append(resources, tile)
		}
	}

	for i := range resources {
		for j := i + 1; j < len(resources); j++ {
			if hex.Distance(resources[i].Coord, resources[j].Coord) <= 1 {
				t.Fatalf("strategic resources adjacent: %v at %v and %v at %v",
					resources[i].Terrain, resources[i].Coord, resources[j].Terrain, resources[j].Coord)
			}
		}
	}
}

func TestNewWorld_StrategicResourcesAreNotAdjacentToForest(t *testing.T) {
	w := NewWorld(42)

	for _, tile := range w.AllTiles() {
		if !isStrategicResource(tile.Terrain) {
			continue
		}
		for _, neighbor := range hex.Circle(tile.Coord, 1) {
			if neighbor == tile.Coord || !hex.InBounds(neighbor) {
				continue
			}
			neighborTile, ok := w.Tile(neighbor)
			if ok && neighborTile.Terrain == terrain.Forest {
				t.Fatalf("strategic resource %v at %v is adjacent to forest at %v", tile.Terrain, tile.Coord, neighbor)
			}
		}
	}
}

func TestNewWorld_TownCentersConnectedByPlainPath(t *testing.T) {
	w := NewWorld(42)
	start := mustTownCenterPos(t, w, entity.Team1)
	end := mustTownCenterPos(t, w, entity.Team2)

	for name, waypoint := range map[string]hex.Coord{
		"top":    {Q: 10, R: 1},
		"middle": {Q: 10, R: 7},
		"bottom": {Q: 10, R: 13},
	} {
		if !plainPathExistsVia(w, start, waypoint, end) {
			t.Fatalf("expected %s lane plain path between town centers %v and %v via %v", name, start, end, waypoint)
		}
	}
}

func mustTownCenterPos(t *testing.T, w *World, team entity.Team) hex.Coord {
	t.Helper()

	for _, b := range w.BuildingsByTeam(team) {
		if b.Kind() == entity.KindTownCenter {
			return b.Position()
		}
	}
	t.Fatalf("missing town center for team %d", team)
	return hex.Coord{}
}

func hasTerrainWithin(w *World, center hex.Coord, radius int, want terrain.Type) bool {
	for _, c := range hex.Circle(center, radius) {
		if tile, ok := w.Tile(c); ok && tile.Terrain == want {
			return true
		}
	}
	return false
}

func plainPathExists(w *World, start, end hex.Coord) bool {
	queue := []hex.Coord{start}
	seen := map[hex.Coord]bool{start: true}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == end {
			return true
		}

		for _, next := range cur.Neighbors() {
			if !hex.InBounds(next) || seen[next] {
				continue
			}
			tile, ok := w.Tile(next)
			if !ok || tile.Terrain != terrain.Plain {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	return false
}

func plainPathExistsVia(w *World, start, via, end hex.Coord) bool {
	return plainPathExists(w, start, via) && plainPathExists(w, via, end)
}

func TestNewWorld_RulesHoldAcrossSeeds(t *testing.T) {
	for seed := int64(1); seed <= 50; seed++ {
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			w := NewWorld(seed)

			for _, team := range []entity.Team{entity.Team1, entity.Team2} {
				tc := mustTownCenterPos(t, w, team)
				for _, kind := range []terrain.Type{terrain.GoldMine, terrain.StoneMine, terrain.Orchard, terrain.Deer} {
					if !hasTerrainWithin(w, tc, 5, kind) {
						t.Fatalf("seed %d team %d town center at %v missing %s within 5 tiles", seed, team, tc, kind.String())
					}
				}
			}

			for _, tile := range w.AllTiles() {
				if !isStrategicResource(tile.Terrain) {
					continue
				}
				for _, neighbor := range hex.Circle(tile.Coord, 1) {
					if neighbor == tile.Coord || !hex.InBounds(neighbor) {
						continue
					}
					neighborTile, ok := w.Tile(neighbor)
					if ok && neighborTile.Terrain == terrain.Forest {
						t.Fatalf("seed %d strategic resource %v at %v is adjacent to forest at %v", seed, tile.Terrain, tile.Coord, neighbor)
					}
				}
			}

			start := mustTownCenterPos(t, w, entity.Team1)
			end := mustTownCenterPos(t, w, entity.Team2)
			for name, waypoint := range map[string]hex.Coord{
				"top":    {Q: 10, R: 1},
				"middle": {Q: 10, R: 7},
				"bottom": {Q: 10, R: 13},
			} {
				if !plainPathExistsVia(w, start, waypoint, end) {
					t.Fatalf("seed %d missing %s lane path between town centers", seed, name)
				}
			}
		})
	}
}
