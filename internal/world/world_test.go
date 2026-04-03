package world

import (
	"testing"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
)

func TestNewWorld_TileCount(t *testing.T) {
	w := NewWorld(42)
	tiles := w.AllTiles()
	want := hex.GridSize * hex.GridSize // 20×20 = 400
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
		prev := tiles[i-1].Coord.Q*hex.GridSize + tiles[i-1].Coord.R
		cur := tiles[i].Coord.Q*hex.GridSize + tiles[i].Coord.R
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
