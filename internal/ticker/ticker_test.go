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
		Team:        entity.Team1,
		UnitID:      gatherer.ID(),
		Kind:        CmdGather,
		TargetCoord: &resourcePos,
	})
	tk.step()

	afterGather := w.GetResources(entity.Team1)
	if afterGather.Food != startRes.Food {
		t.Fatalf("expected carry-only gather before deposit, got %+v from %+v", afterGather, startRes)
	}
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

func TestStep_AttackPersistsAndClosesDistance(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	attacker := w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 2, R: 2})
	target := w.SpawnUnit(entity.Team2, entity.KindArcher, hex.Coord{Q: 7, R: 2})

	q.Submit(Command{Team: entity.Team1, UnitID: attacker.ID(), Kind: CmdAttack, TargetID: ptrID(target.ID())})
	startDist := hex.Distance(attacker.Position(), target.Position())
	tk.step()
	afterFirst := w.GetUnit(attacker.ID())
	if afterFirst == nil || afterFirst.Status() != entity.StatusAttacking {
		t.Fatalf("expected attacker to stay in attacking status while chasing")
	}
	if got := hex.Distance(afterFirst.Position(), target.Position()); got >= startDist {
		t.Fatalf("expected attacker to close distance, start=%d got=%d", startDist, got)
	}

	for i := 0; i < 4; i++ {
		tk.step()
		if w.GetUnit(target.ID()) == nil {
			break
		}
	}
	if w.GetUnit(target.ID()) != nil && w.GetUnit(target.ID()).HP() == w.GetUnit(target.ID()).MaxHP() {
		t.Fatalf("expected target to eventually take damage")
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
	if _, ok := w.GetUnit(attacker.ID()).AttackTargetID(); ok {
		t.Fatalf("expected attack target to clear once target is dead")
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

func TestStep_MoveFastPersistsAcrossTicksUntilArrival(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	unit := w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 2, R: 2})
	target := hex.Coord{Q: 7, R: 2}

	q.Submit(Command{Team: entity.Team1, UnitID: unit.ID(), Kind: CmdMoveFast, TargetCoord: &target})
	tk.step()
	firstPos := w.GetUnit(unit.ID()).Position()
	if firstPos == (hex.Coord{Q: 2, R: 2}) {
		t.Fatalf("expected unit to start moving on first tick")
	}
	if w.GetUnit(unit.ID()).Status() != entity.StatusMovingFast {
		t.Fatalf("expected status to persist after first tick")
	}

	tk.step()
	secondPos := w.GetUnit(unit.ID()).Position()
	if hex.Distance(secondPos, target) >= hex.Distance(firstPos, target) {
		t.Fatalf("expected unit to keep moving toward target, first=%v second=%v target=%v", firstPos, secondPos, target)
	}

	for i := 0; i < 4; i++ {
		tk.step()
	}
	if got := w.GetUnit(unit.ID()).Position(); got != target {
		t.Fatalf("expected unit to arrive at target, got %v want %v", got, target)
	}
	if w.GetUnit(unit.ID()).Status() != entity.StatusIdle {
		t.Fatalf("expected unit to become idle after arrival")
	}
}

func TestStep_MoveFastRoutesAroundLakeBarrier(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	unit := w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 2, R: 5})
	target := hex.Coord{Q: 8, R: 5}

	w.WriteFunc(func() {
		for q := 1; q <= 9; q++ {
			for r := 2; r <= 8; r++ {
				c := hex.Coord{Q: q, R: r}
				w.Tiles[c] = terrain.Tile{Coord: c, Terrain: terrain.Plain}
			}
		}
		for _, c := range []hex.Coord{
			{Q: 4, R: 3},
			{Q: 4, R: 4},
			{Q: 4, R: 5},
			{Q: 4, R: 6},
		} {
			w.Tiles[c] = terrain.Tile{Coord: c, Terrain: terrain.Lake}
		}
	})

	q.Submit(Command{Team: entity.Team1, UnitID: unit.ID(), Kind: CmdMoveFast, TargetCoord: &target})
	for i := 0; i < 4; i++ {
		tk.step()
	}

	if got := w.GetUnit(unit.ID()).Position(); got != target {
		t.Fatalf("expected unit to route around barrier and reach %v, got %v", target, got)
	}
}

func TestStep_GatherAutoShuttlesUntilDeposit(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	villager := w.UnitsByTeam(entity.Team1)[0]
	resourcePos := hex.Coord{Q: 5, R: 5}
	startRes := w.GetResources(entity.Team1)

	w.WriteFunc(func() {
		villager.SetPosition(resourcePos)
		w.Tiles[resourcePos] = terrain.Tile{Coord: resourcePos, Terrain: terrain.Orchard}
		w.ResourceRemaining[resourcePos] = 36
	})

	q.Submit(Command{Team: entity.Team1, UnitID: villager.ID(), Kind: CmdGather, TargetCoord: &resourcePos})
	tk.step()
	if villager.CarryAmount() != 18 {
		t.Fatalf("expected villager to carry after gather, got %d", villager.CarryAmount())
	}
	tk.step()
	after := w.GetResources(entity.Team1)
	if after.Food != startRes.Food+18 {
		t.Fatalf("expected auto deposit after second tick, got %+v want food=%d", after, startRes.Food+18)
	}
	if villager.Status() != entity.StatusGathering {
		t.Fatalf("expected gather status to persist while node remains")
	}
}

func TestStep_BuildPersistsUntilConstructionCompletes(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	builder := w.UnitsByTeam(entity.Team1)[0]
	target := hex.Coord{Q: 6, R: 5}
	buildingKind := "barracks"

	w.WriteFunc(func() {
		builder.SetPosition(hex.Coord{Q: 5, R: 5})
		w.Tiles[target] = terrain.Tile{Coord: target, Terrain: terrain.Plain}
	})

	q.Submit(Command{
		Team:         entity.Team1,
		UnitID:       builder.ID(),
		Kind:         CmdBuild,
		TargetCoord:  &target,
		BuildingKind: &buildingKind,
	})
	tk.step()
	if builder.Status() != entity.StatusBuilding {
		t.Fatalf("expected builder to stay in building status after site creation")
	}
	tk.step()
	if builder.Status() != entity.StatusIdle {
		t.Fatalf("expected builder to become idle after completion")
	}
}

func TestStep_BuildAutoMovesThenConstructs(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	builder := w.UnitsByTeam(entity.Team1)[0]
	teamUnits := w.UnitsByTeam(entity.Team1)
	target := hex.Coord{Q: 6, R: 5}
	buildingKind := "barracks"

	w.WriteFunc(func() {
		builder.SetPosition(hex.Coord{Q: 2, R: 5})
		offset := 0
		for _, u := range teamUnits {
			if u.ID() == builder.ID() {
				continue
			}
			u.SetPosition(hex.Coord{Q: offset, R: 14})
			offset++
		}
		for q := 2; q <= 6; q++ {
			c := hex.Coord{Q: q, R: 5}
			w.Tiles[c] = terrain.Tile{Coord: c, Terrain: terrain.Plain}
		}
		w.Tiles[target] = terrain.Tile{Coord: target, Terrain: terrain.Plain}
		for _, c := range hex.Circle(target, 1) {
			if hex.InBounds(c) {
				w.Tiles[c] = terrain.Tile{Coord: c, Terrain: terrain.Plain}
			}
		}
	})

	q.Submit(Command{
		Team:         entity.Team1,
		UnitID:       builder.ID(),
		Kind:         CmdBuild,
		TargetCoord:  &target,
		BuildingKind: &buildingKind,
	})

	tk.step()
	if builder.Status() != entity.StatusBuilding {
		t.Fatalf("expected builder to keep building status while moving")
	}
	if hex.Distance(builder.Position(), target) <= 1 {
		t.Fatalf("expected builder to still be approaching after first tick")
	}

	for i := 0; i < 4; i++ {
		tk.step()
	}

	var found bool
	for _, b := range w.BuildingsByTeam(entity.Team1) {
		if b.Kind() == entity.KindBarracks && b.Position() == target {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected builder to eventually create barracks at %v, builder_pos=%v status=%s phase=%s", target, builder.Position(), builder.Status(), builder.StatusPhase())
	}
}

func TestStep_StopClearsPersistentUnitStatus(t *testing.T) {
	w := world.NewWorld(42)
	q := NewQueue()
	tk := New(w, q, time.Second)

	unit := w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 2, R: 2})
	target := hex.Coord{Q: 7, R: 2}

	q.Submit(Command{Team: entity.Team1, UnitID: unit.ID(), Kind: CmdMoveFast, TargetCoord: &target})
	tk.step()
	if w.GetUnit(unit.ID()).Status() != entity.StatusMovingFast {
		t.Fatalf("expected unit to be moving before stop")
	}

	q.Submit(Command{Team: entity.Team1, UnitID: unit.ID(), Kind: CmdStop})
	tk.step()
	if w.GetUnit(unit.ID()).Status() != entity.StatusIdle {
		t.Fatalf("expected stop to clear unit status")
	}
}

func ptrID(id entity.EntityID) *entity.EntityID {
	return &id
}
