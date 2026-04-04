package ticker

import (
	"testing"
	"time"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

func TestStep_AppliesGatherBuildAndProduce(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	villagers := w.UnitsByTeam(entity.Team1)
	builder := villagers[0]
	gatherer := villagers[1]
	buildPos := hex.Coord{Q: 6, R: 5}
	resourcePos := hex.Coord{Q: 5, R: 5}
	startRes := w.GetResources(entity.Team1)

	w.WriteFunc(func() {
		builder.SetPosition(resourcePos)
		gatherer.SetPosition(resourcePos)
		w.Tiles[resourcePos] = terrain.Tile{Coord: resourcePos, Terrain: terrain.Orchard}
		w.ResourceRemaining[resourcePos] = 18
		w.Tiles[buildPos] = terrain.Tile{Coord: buildPos, Terrain: terrain.Plain}
	})

	q.Submit(Command{
		Team:   entity.Team1,
		UnitID: gatherer.ID(),
		Kind:   CmdGather,
	})
	tk.step()

	afterGather := w.GetResources(entity.Team1)
	if afterGather.Food != startRes.Food {
		t.Fatalf("expected carry-only gather before deposit, got %+v from %+v", afterGather, startRes)
	}
	tc := hex.Coord{Q: 4, R: 4}
	w.WriteFunc(func() {
		gatherer.SetPosition(hex.Coord{Q: tc.Q + 1, R: tc.R})
	})
	q.Submit(Command{
		Team:   entity.Team1,
		UnitID: gatherer.ID(),
		Kind:   CmdGather,
	})
	tk.step()
	afterDeposit := w.GetResources(entity.Team1)
	if afterDeposit.Food != startRes.Food+18 {
		t.Fatalf("expected deposit to add food, got %+v from %+v", afterDeposit, startRes)
	}

	buildingKind := "barracks"
	q.Submit(Command{
		Team:         entity.Team1,
		UnitID:       builder.ID(),
		Kind:         CmdBuild,
		TargetCoord:  &buildPos,
		BuildingKind: &buildingKind,
	})
	tk.step()

	var barracksID entity.EntityID
	for _, b := range w.BuildingsByTeam(entity.Team1) {
		if b.Kind() == entity.KindBarracks && b.Position() == buildPos {
			barracksID = b.ID()
		}
	}
	if barracksID == 0 {
		t.Fatalf("expected barracks to be built")
	}
	barracks := w.GetBuilding(barracksID)
	if barracks == nil || barracks.IsComplete() {
		t.Fatalf("expected barracks to start under construction")
	}

	tk.step()
	if barracks := w.GetBuilding(barracksID); barracks == nil || !barracks.IsComplete() {
		t.Fatalf("expected barracks to complete on the next tick")
	}

	unitKind := "infantry"
	q.Submit(Command{
		Team:       entity.Team1,
		BuildingID: &barracksID,
		Kind:       CmdProduce,
		UnitKind:   &unitKind,
	})

	before := len(w.UnitsByTeam(entity.Team1))
	tk.step()
	after := len(w.UnitsByTeam(entity.Team1))

	if after != before+1 {
		t.Fatalf("expected produced infantry, units before=%d after=%d", before, after)
	}
}

func TestStep_CavalryCannotEnterForest(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	scout := w.SpawnUnit(entity.Team1, entity.KindScoutCavalry, hex.Coord{Q: 8, R: 7})
	target := hex.Coord{Q: 9, R: 7}
	w.WriteFunc(func() {
		w.Tiles[target] = terrain.Tile{Coord: target, Terrain: terrain.Forest}
	})

	q.Submit(Command{
		Team: entity.Team1, UnitID: scout.ID(), Kind: CmdMoveFast, TargetCoord: &target,
	})
	tk.step()

	if got := w.GetUnit(scout.ID()).Position(); got != (hex.Coord{Q: 8, R: 7}) {
		t.Fatalf("scout cavalry moved into forest: %v", got)
	}
}

func TestStep_SameHexConflictBlocksBothUnits(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	u1 := w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 7, R: 7})
	u2 := w.SpawnUnit(entity.Team2, entity.KindInfantry, hex.Coord{Q: 9, R: 7})
	target := hex.Coord{Q: 8, R: 7}

	q.Submit(Command{Team: entity.Team1, UnitID: u1.ID(), Kind: CmdMoveFast, TargetCoord: &target})
	q.Submit(Command{Team: entity.Team2, UnitID: u2.ID(), Kind: CmdMoveFast, TargetCoord: &target})
	tk.step()

	if got := w.GetUnit(u1.ID()).Position(); got != (hex.Coord{Q: 7, R: 7}) {
		t.Fatalf("unit 1 should stay in place, got %v", got)
	}
	if got := w.GetUnit(u2.ID()).Position(); got != (hex.Coord{Q: 9, R: 7}) {
		t.Fatalf("unit 2 should stay in place, got %v", got)
	}
}

func TestStep_AttackPersistsAcrossTicks(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	attacker := w.SpawnUnit(entity.Team1, entity.KindArcher, hex.Coord{Q: 7, R: 7})
	target := w.SpawnUnit(entity.Team2, entity.KindSpearman, hex.Coord{Q: 9, R: 7})

	q.Submit(Command{Team: entity.Team1, UnitID: attacker.ID(), Kind: CmdAttack, TargetID: ptrID(target.ID())})
	tk.step()
	if targetID, ok := w.GetUnit(attacker.ID()).AttackTargetID(); !ok || targetID != target.ID() {
		t.Fatalf("expected attacker to expose active target after first tick")
	}
	hpAfterFirst := w.GetUnit(target.ID()).HP()
	tk.step()
	hpAfterSecond := w.GetUnit(target.ID()).HP()

	if hpAfterSecond >= hpAfterFirst {
		t.Fatalf("expected persistent attack to continue, hp1=%d hp2=%d", hpAfterFirst, hpAfterSecond)
	}
}

func TestStep_AttackTargetClearsAfterTargetDies(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	attacker := w.SpawnUnit(entity.Team1, entity.KindArcher, hex.Coord{Q: 7, R: 7})
	target := w.SpawnUnit(entity.Team2, entity.KindSpearman, hex.Coord{Q: 9, R: 7})
	w.WriteFunc(func() {
		target.SetHP(1)
	})

	q.Submit(Command{Team: entity.Team1, UnitID: attacker.ID(), Kind: CmdAttack, TargetID: ptrID(target.ID())})
	tk.step()
	if _, ok := w.GetUnit(attacker.ID()).AttackTargetID(); !ok {
		t.Fatalf("expected attacker to still show target during kill tick")
	}
	tk.step()
	if _, ok := w.GetUnit(attacker.ID()).AttackTargetID(); ok {
		t.Fatalf("expected attack target to clear after target is gone")
	}
}

func TestStep_SimultaneousCombatAllowsMutualKill(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	u1 := w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 7, R: 7})
	u2 := w.SpawnUnit(entity.Team2, entity.KindInfantry, hex.Coord{Q: 8, R: 7})
	w.WriteFunc(func() {
		u1.SetHP(1)
		u2.SetHP(1)
	})

	q.Submit(Command{Team: entity.Team1, UnitID: u1.ID(), Kind: CmdAttack, TargetID: ptrID(u2.ID())})
	q.Submit(Command{Team: entity.Team2, UnitID: u2.ID(), Kind: CmdAttack, TargetID: ptrID(u1.ID())})
	tk.step()

	if w.GetUnit(u1.ID()) != nil || w.GetUnit(u2.ID()) != nil {
		t.Fatalf("expected mutual kill, got u1=%v u2=%v", w.GetUnit(u1.ID()), w.GetUnit(u2.ID()))
	}
}

func ptrID(id entity.EntityID) *entity.EntityID {
	return &id
}
