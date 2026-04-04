package world

import (
	"testing"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
)

func TestGatherAtCurrentTile_VillagerCarriesThenDepositsResources(t *testing.T) {
	w := NewWorld(42)
	villager := w.UnitsByTeam(entity.Team1)[0]
	start := w.GetResources(entity.Team1)
	tc := mustTownCenterPos(t, w, entity.Team1)

	w.WriteFunc(func() {
		villager.SetPosition(hex.Coord{Q: 3, R: 8})
		w.Tiles[hex.Coord{Q: 3, R: 8}] = terrain.Tile{Coord: hex.Coord{Q: 3, R: 8}, Terrain: terrain.GoldMine}
		w.ResourceRemaining[hex.Coord{Q: 3, R: 8}] = 15
	})

	if !w.GatherAtCurrentTile(villager.ID()) {
		t.Fatalf("expected villager gather to succeed")
	}
	if villager.CarryType() != terrain.ResourceGold || villager.CarryAmount() != 12 {
		t.Fatalf("carry = (%q, %d), want (%q, %d)", villager.CarryType(), villager.CarryAmount(), terrain.ResourceGold, 12)
	}

	w.WriteFunc(func() {
		villager.SetPosition(hex.Coord{Q: tc.Q + 1, R: tc.R})
	})
	if !w.GatherAtCurrentTile(villager.ID()) {
		t.Fatalf("expected villager deposit to succeed")
	}

	after := w.GetResources(entity.Team1)
	if after.Gold != start.Gold+12 {
		t.Fatalf("gold = %d, want %d", after.Gold, start.Gold+12)
	}
	if villager.CarryAmount() != 0 {
		t.Fatalf("expected carry to clear after deposit")
	}
}

func TestGatherAtCurrentTile_DepletesResourceTile(t *testing.T) {
	w := NewWorld(42)
	villager := w.UnitsByTeam(entity.Team1)[0]
	pos := hex.Coord{Q: 3, R: 8}

	w.WriteFunc(func() {
		villager.SetPosition(pos)
		w.Tiles[pos] = terrain.Tile{Coord: pos, Terrain: terrain.Deer}
		w.ResourceRemaining[pos] = 40
	})

	if !w.GatherAtCurrentTile(villager.ID()) {
		t.Fatalf("expected gather to succeed")
	}
	w.WriteFunc(func() {
		villager.ClearCarry()
	})
	if !w.GatherAtCurrentTile(villager.ID()) {
		t.Fatalf("expected second gather to succeed")
	}
	tile, _ := w.Tile(pos)
	if tile.Terrain != terrain.Plain {
		t.Fatalf("expected depleted tile to become plain, got %v", tile.Terrain)
	}
}

func TestBuildStructure_VillagerStartsBarracksConstruction(t *testing.T) {
	w := NewWorld(42)
	villager := w.UnitsByTeam(entity.Team1)[0]
	target := hex.Coord{Q: 6, R: 5}
	start := w.GetResources(entity.Team1)

	w.WriteFunc(func() {
		villager.SetPosition(hex.Coord{Q: 5, R: 5})
		w.Tiles[target] = terrain.Tile{Coord: target, Terrain: terrain.Plain}
	})

	if !w.BuildStructure(villager.ID(), entity.KindBarracks, target) {
		t.Fatalf("expected villager build to succeed")
	}

	found := false
	for _, b := range w.BuildingsByTeam(entity.Team1) {
		if b.Kind() == entity.KindBarracks && b.Position() == target {
			found = true
			if b.IsComplete() {
				t.Fatalf("expected new barracks to start under construction")
			}
		}
	}
	if !found {
		t.Fatalf("expected barracks at %v", target)
	}

	after := w.GetResources(entity.Team1)
	cost := entity.BuildingCost(entity.KindBarracks)
	if after.Wood != start.Wood-cost.Wood || after.Stone != start.Stone-cost.Stone {
		t.Fatalf("resources = %+v, want wood=%d stone=%d", after, start.Wood-cost.Wood, start.Stone-cost.Stone)
	}
}

func TestProcessConstruction_CompletesBuildingAfterTicks(t *testing.T) {
	w := NewWorld(42)
	villager := w.UnitsByTeam(entity.Team1)[0]
	target := hex.Coord{Q: 6, R: 5}

	w.WriteFunc(func() {
		villager.SetPosition(hex.Coord{Q: 5, R: 5})
		w.Tiles[target] = terrain.Tile{Coord: target, Terrain: terrain.Plain}
	})
	if !w.BuildStructure(villager.ID(), entity.KindBarracks, target) {
		t.Fatalf("expected build start to succeed")
	}

	var barracks *entity.Building
	for _, b := range w.BuildingsByTeam(entity.Team1) {
		if b.Kind() == entity.KindBarracks && b.Position() == target {
			barracks = b
		}
	}
	if barracks == nil {
		t.Fatalf("missing barracks")
	}
	w.ProcessConstruction()
	if barracks.IsComplete() {
		t.Fatalf("barracks should still be under construction after 1 tick")
	}
	w.ProcessConstruction()
	if !barracks.IsComplete() {
		t.Fatalf("barracks should be complete after 2 ticks")
	}
}

func TestAttackTarget_SpearmanCountersPaladin(t *testing.T) {
	w := NewWorld(42)
	attacker := w.SpawnUnit(entity.Team1, entity.KindSpearman, hex.Coord{Q: 8, R: 7})
	target := w.SpawnUnit(entity.Team2, entity.KindPaladin, hex.Coord{Q: 9, R: 7})

	if !w.AttackTarget(attacker.ID(), target.ID()) {
		t.Fatalf("expected attack to succeed")
	}

	got := w.GetUnit(target.ID()).HP()
	want := entity.UnitSpecs[entity.KindPaladin].Stats.MaxHP - (entity.UnitSpecs[entity.KindSpearman].Stats.Attack + 8 - entity.UnitSpecs[entity.KindPaladin].Stats.Defense)
	if got != want {
		t.Fatalf("paladin hp = %d, want %d", got, want)
	}
}

func TestEnqueueProduction_AndProcessProduction(t *testing.T) {
	w := NewWorld(42)
	barracks := w.SpawnBuilding(entity.Team1, entity.KindBarracks, hex.Coord{Q: 8, R: 7})
	startUnits := len(w.UnitsByTeam(entity.Team1))
	start := w.GetResources(entity.Team1)

	if !w.EnqueueProduction(barracks.ID(), entity.KindInfantry) {
		t.Fatalf("expected enqueue production to succeed")
	}

	w.ProcessProduction()

	afterUnits := len(w.UnitsByTeam(entity.Team1))
	if afterUnits != startUnits+1 {
		t.Fatalf("units = %d, want %d", afterUnits, startUnits+1)
	}

	after := w.GetResources(entity.Team1)
	cost := entity.UnitCost(entity.KindInfantry)
	if after.Food != start.Food-cost.Food || after.Wood != start.Wood-cost.Wood {
		t.Fatalf("resources after production = %+v, want food=%d wood=%d", after, start.Food-cost.Food, start.Wood-cost.Wood)
	}
}

func TestEnqueueProduction_RespectsMultiTickTraining(t *testing.T) {
	w := NewWorld(42)
	stable := w.SpawnBuilding(entity.Team1, entity.KindStable, hex.Coord{Q: 8, R: 7})
	startUnits := len(w.UnitsByTeam(entity.Team1))

	if !w.EnqueueProduction(stable.ID(), entity.KindPaladin) {
		t.Fatalf("expected enqueue to succeed")
	}

	w.ProcessProduction()
	if len(w.UnitsByTeam(entity.Team1)) != startUnits {
		t.Fatalf("paladin should not spawn on first production tick")
	}
	w.ProcessProduction()
	if len(w.UnitsByTeam(entity.Team1)) != startUnits+1 {
		t.Fatalf("paladin should spawn on second production tick")
	}
}

func TestEnqueueProduction_RejectsWhenPopulationCapReached(t *testing.T) {
	w := NewWorld(42)
	stable := w.SpawnBuilding(entity.Team1, entity.KindStable, hex.Coord{Q: 8, R: 7})

	for i := len(w.UnitsByTeam(entity.Team1)); i < entity.PopulationCap; i++ {
		w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 10 + (i % 5), R: 5 + (i / 5)})
	}

	if w.EnqueueProduction(stable.ID(), entity.KindScoutCavalry) {
		t.Fatalf("expected production enqueue to fail at population cap")
	}
}
