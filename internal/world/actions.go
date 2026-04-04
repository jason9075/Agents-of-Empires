package world

import (
	"sort"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
)

type BuildTargetStatus uint8

const (
	BuildTargetInvalid BuildTargetStatus = iota
	BuildTargetCreate
	BuildTargetResume
	BuildTargetBlocked
)

type BuildActionResult uint8

const (
	BuildActionInvalid BuildActionResult = iota
	BuildActionBlocked
	BuildActionWorking
	BuildActionComplete
)

type ProductionEnqueueResult uint8

const (
	ProductionEnqueueQueued ProductionEnqueueResult = iota
	ProductionEnqueueProducerUnavailable
	ProductionEnqueueBuildingUnderConstruction
	ProductionEnqueueInvalidProducer
	ProductionEnqueueInsufficientResources
	ProductionEnqueuePopulationCapReached
)

func isAdjacentToFriendlyTownCenter(buildings map[entity.EntityID]*entity.Building, team entity.Team, pos hex.Coord) bool {
	for _, b := range buildings {
		if !b.IsAlive() || !b.IsComplete() || b.Team() != team || b.Kind() != entity.KindTownCenter {
			continue
		}
		if hex.Distance(pos, b.Position()) <= 1 {
			return true
		}
	}
	return false
}

func buildingAtLocked(buildings map[entity.EntityID]*entity.Building, c hex.Coord) *entity.Building {
	for _, b := range buildings {
		if b.IsAlive() && b.Position() == c {
			return b
		}
	}
	return nil
}

// PreviewMoveStep returns the first step on a shortest legal path to target, if any.
func (w *World) PreviewMoveStep(unitID entity.EntityID, target hex.Coord) (hex.Coord, bool) {
	return w.PreviewMoveStepToAny(unitID, []hex.Coord{target})
}

// PreviewMoveStepToAny returns the first step on a shortest legal path to any target, if any.
func (w *World) PreviewMoveStepToAny(unitID entity.EntityID, targets []hex.Coord) (hex.Coord, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	u := w.Units[unitID]
	if u == nil || !u.IsAlive() {
		return hex.Coord{}, false
	}

	cur := u.Position()
	goalSet := make(map[hex.Coord]struct{}, len(targets))
	for _, target := range targets {
		if !hex.InBounds(target) {
			continue
		}
		goalSet[target] = struct{}{}
	}
	if len(goalSet) == 0 {
		return hex.Coord{}, false
	}
	if _, ok := goalSet[cur]; ok {
		return hex.Coord{}, false
	}

	queue := []hex.Coord{cur}
	visited := map[hex.Coord]bool{cur: true}
	parent := make(map[hex.Coord]hex.Coord, len(goalSet))

	for head := 0; head < len(queue); head++ {
		pos := queue[head]
		for _, next := range pos.Neighbors() {
			if !hex.InBounds(next) || visited[next] {
				continue
			}
			if !w.canUnitOccupyLocked(u.Kind(), next, unitID, 0) {
				continue
			}

			visited[next] = true
			parent[next] = pos
			if _, ok := goalSet[next]; ok {
				step := next
				for parent[step] != cur {
					step = parent[step]
				}
				return step, true
			}
			queue = append(queue, next)
		}
	}

	return hex.Coord{}, false
}

// ApplyUnitMoves applies accepted movement results simultaneously.
func (w *World) ApplyUnitMoves(moves map[entity.EntityID]hex.Coord) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for id, pos := range moves {
		if u := w.Units[id]; u != nil && u.IsAlive() {
			u.SetPosition(pos)
		}
	}
}

// MoveUnitToward moves the unit toward target up to speed steps using shortest-path routing.
func (w *World) MoveUnitToward(unitID entity.EntityID, target hex.Coord, speed int) bool {
	if speed <= 0 {
		return false
	}

	moved := false
	for step := 0; step < speed; step++ {
		next, ok := w.PreviewMoveStep(unitID, target)
		if !ok {
			break
		}
		w.ApplyUnitMoves(map[entity.EntityID]hex.Coord{unitID: next})
		moved = true
	}

	return moved
}

// GatherAtCurrentTile lets a villager harvest the resource on its current tile.
func (w *World) GatherAtCurrentTile(unitID entity.EntityID) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	u := w.Units[unitID]
	if u == nil || !u.IsAlive() || !entity.UnitCanGather(u.Kind()) {
		return false
	}

	tile, ok := w.Tiles[u.Position()]
	if !ok {
		return false
	}

	if u.CarryAmount() > 0 {
		if !isAdjacentToFriendlyTownCenter(w.Buildings, u.Team(), u.Position()) {
			return false
		}
		res := w.TeamRes[u.Team()]
		switch u.CarryType() {
		case terrain.ResourceFood:
			res.Food += u.CarryAmount()
		case terrain.ResourceGold:
			res.Gold += u.CarryAmount()
		case terrain.ResourceStone:
			res.Stone += u.CarryAmount()
		case terrain.ResourceWood:
			res.Wood += u.CarryAmount()
		default:
			return false
		}
		w.TeamRes[u.Team()] = res
		u.ClearCarry()
		return true
	}

	yield := tile.Terrain.ResourceYield()
	if yield == terrain.ResourceNone {
		return false
	}
	remaining := w.ResourceRemaining[u.Position()]
	if remaining <= 0 {
		return false
	}

	amount := entity.ResourceGatherAmount(tile.Terrain)
	capacity := entity.UnitCarryCapacity(u.Kind())
	if amount > capacity {
		amount = capacity
	}
	if amount > remaining {
		amount = remaining
	}
	u.SetCarry(yield, amount)
	w.ResourceRemaining[u.Position()] = remaining - amount
	if w.ResourceRemaining[u.Position()] <= 0 {
		delete(w.ResourceRemaining, u.Position())
		w.Tiles[u.Position()] = terrain.Tile{Coord: u.Position(), Terrain: terrain.Plain}
	}
	return true
}

// BuildStructure creates a structure at target if the villager is allowed and the team can pay the cost.
func (w *World) BuildStructure(builderID entity.EntityID, kind entity.BuildingKind, target hex.Coord) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	u := w.Units[builderID]
	if u == nil || !u.IsAlive() || !entity.UnitCanBuild(u.Kind()) {
		return false
	}
	if kind == entity.KindTownCenter {
		return false
	}
	if !hex.InBounds(target) || hex.Distance(u.Position(), target) > 1 {
		return false
	}
	tile, ok := w.Tiles[target]
	if !ok || tile.Terrain != terrain.Plain {
		return false
	}
	if !w.canTileBeOccupiedLocked(target, 0, 0) {
		return false
	}

	cost := entity.BuildingCost(kind)
	if !w.canAffordLocked(u.Team(), cost) {
		return false
	}
	w.payLocked(u.Team(), cost)

	b := entity.NewConstruction(w.nextID(), u.Team(), kind, target)
	w.Buildings[b.ID()] = b
	return true
}

func (w *World) BuildTargetStatus(team entity.Team, kind entity.BuildingKind, target hex.Coord) BuildTargetStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if !hex.InBounds(target) {
		return BuildTargetInvalid
	}
	if existing := buildingAtLocked(w.Buildings, target); existing != nil {
		if existing.Team() == team && existing.Kind() == kind && !existing.IsComplete() {
			return BuildTargetResume
		}
		return BuildTargetInvalid
	}
	tile, ok := w.Tiles[target]
	if !ok || tile.Terrain != terrain.Plain {
		return BuildTargetInvalid
	}
	for _, u := range w.Units {
		if u.IsAlive() && u.Position() == target {
			return BuildTargetBlocked
		}
	}
	return BuildTargetCreate
}

func (w *World) WorkOnBuild(builderID entity.EntityID, kind entity.BuildingKind, target hex.Coord) BuildActionResult {
	w.mu.Lock()
	defer w.mu.Unlock()

	u := w.Units[builderID]
	if u == nil || !u.IsAlive() || !entity.UnitCanBuild(u.Kind()) || kind == entity.KindTownCenter {
		return BuildActionInvalid
	}
	if !hex.InBounds(target) || hex.Distance(u.Position(), target) > 1 {
		return BuildActionInvalid
	}

	if existing := buildingAtLocked(w.Buildings, target); existing != nil {
		if existing.Team() != u.Team() || existing.Kind() != kind {
			return BuildActionInvalid
		}
		if existing.IsComplete() {
			return BuildActionComplete
		}
		existing.AdvanceConstruction()
		if existing.IsComplete() {
			return BuildActionComplete
		}
		return BuildActionWorking
	}

	tile, ok := w.Tiles[target]
	if !ok || tile.Terrain != terrain.Plain {
		return BuildActionInvalid
	}
	for _, other := range w.Units {
		if other.IsAlive() && other.Position() == target {
			return BuildActionBlocked
		}
	}

	cost := entity.BuildingCost(kind)
	if !w.canAffordLocked(u.Team(), cost) {
		return BuildActionBlocked
	}
	w.payLocked(u.Team(), cost)

	b := entity.NewConstruction(w.nextID(), u.Team(), kind, target)
	b.AdvanceConstruction()
	w.Buildings[b.ID()] = b
	if b.IsComplete() {
		return BuildActionComplete
	}
	return BuildActionWorking
}

// AttackTarget applies one attack from attacker to a target entity if in range.
func (w *World) AttackTarget(attackerID, targetID entity.EntityID) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	attacker := w.Units[attackerID]
	if attacker == nil || !attacker.IsAlive() {
		return false
	}

	targetUnit := w.Units[targetID]
	if targetUnit != nil && targetUnit.IsAlive() {
		if targetUnit.Team() == attacker.Team() {
			return false
		}
		if hex.Distance(attacker.Position(), targetUnit.Position()) > entity.AttackRange(attacker.Kind()) {
			return false
		}

		damage := attacker.Stats().Attack + entity.CounterBonus(attacker.Kind(), targetUnit.Kind()) - targetUnit.Stats().Defense
		if damage < 1 {
			damage = 1
		}
		targetUnit.SetHP(targetUnit.HP() - damage)
		if !targetUnit.IsAlive() {
			delete(w.Units, targetID)
		}
		return true
	}

	targetBuilding := w.Buildings[targetID]
	if targetBuilding == nil || !targetBuilding.IsAlive() || targetBuilding.Team() == attacker.Team() {
		return false
	}
	if hex.Distance(attacker.Position(), targetBuilding.Position()) > entity.AttackRange(attacker.Kind()) {
		return false
	}

	damage := attacker.Stats().Attack
	if damage < 1 {
		damage = 1
	}
	targetBuilding.SetHP(targetBuilding.HP() - damage)
	if !targetBuilding.IsAlive() {
		delete(w.Buildings, targetID)
	}
	return true
}

// PreviewAttackDamage returns the damage an attacker would deal if the target is valid and in range.
func (w *World) PreviewAttackDamage(attackerID, targetID entity.EntityID) (int, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	attacker := w.Units[attackerID]
	if attacker == nil || !attacker.IsAlive() {
		return 0, false
	}

	if targetUnit := w.Units[targetID]; targetUnit != nil && targetUnit.IsAlive() {
		if targetUnit.Team() == attacker.Team() {
			return 0, false
		}
		if hex.Distance(attacker.Position(), targetUnit.Position()) > entity.AttackRange(attacker.Kind()) {
			return 0, false
		}
		damage := attacker.Stats().Attack + entity.CounterBonus(attacker.Kind(), targetUnit.Kind()) - targetUnit.Stats().Defense
		if damage < 1 {
			damage = 1
		}
		return damage, true
	}

	if targetBuilding := w.Buildings[targetID]; targetBuilding != nil && targetBuilding.IsAlive() {
		if targetBuilding.Team() == attacker.Team() {
			return 0, false
		}
		if hex.Distance(attacker.Position(), targetBuilding.Position()) > entity.AttackRange(attacker.Kind()) {
			return 0, false
		}
		damage := attacker.Stats().Attack
		if damage < 1 {
			damage = 1
		}
		return damage, true
	}

	return 0, false
}

// FindAutoAttackTarget selects the nearest valid enemy, then lowest HP, then lowest entity ID.
func (w *World) FindAutoAttackTarget(attackerID entity.EntityID) (entity.EntityID, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	attacker := w.Units[attackerID]
	if attacker == nil || !attacker.IsAlive() {
		return 0, false
	}

	type candidate struct {
		id   entity.EntityID
		dist int
		hp   int
	}
	var candidates []candidate
	for id, u := range w.Units {
		if !u.IsAlive() || u.Team() == attacker.Team() {
			continue
		}
		dist := hex.Distance(attacker.Position(), u.Position())
		if dist > entity.AttackRange(attacker.Kind()) {
			continue
		}
		candidates = append(candidates, candidate{id: id, dist: dist, hp: u.HP()})
	}
	for id, b := range w.Buildings {
		if !b.IsAlive() || b.Team() == attacker.Team() {
			continue
		}
		dist := hex.Distance(attacker.Position(), b.Position())
		if dist > entity.AttackRange(attacker.Kind()) {
			continue
		}
		candidates = append(candidates, candidate{id: id, dist: dist, hp: b.HP()})
	}
	if len(candidates) == 0 {
		return 0, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].dist != candidates[j].dist {
			return candidates[i].dist < candidates[j].dist
		}
		if candidates[i].hp != candidates[j].hp {
			return candidates[i].hp < candidates[j].hp
		}
		return candidates[i].id < candidates[j].id
	})
	return candidates[0].id, true
}

// ApplyDamage applies simultaneous combat damage and removes dead entities after damage is accumulated.
func (w *World) ApplyDamage(damage map[entity.EntityID]int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for id, amount := range damage {
		if amount <= 0 {
			continue
		}
		if u := w.Units[id]; u != nil && u.IsAlive() {
			u.SetHP(u.HP() - amount)
		}
		if b := w.Buildings[id]; b != nil && b.IsAlive() {
			b.SetHP(b.HP() - amount)
		}
	}

	for id, u := range w.Units {
		if !u.IsAlive() {
			delete(w.Units, id)
		}
	}
	for id, b := range w.Buildings {
		if !b.IsAlive() {
			delete(w.Buildings, id)
		}
	}
}

// EnqueueProduction adds a unit to a building queue if the team can pay and the producer matches.
func (w *World) EnqueueProduction(buildingID entity.EntityID, kind entity.UnitKind) bool {
	return w.TryEnqueueProduction(buildingID, kind) == ProductionEnqueueQueued
}

// TryEnqueueProduction adds a unit to a building queue if the producer is valid.
func (w *World) TryEnqueueProduction(buildingID entity.EntityID, kind entity.UnitKind) ProductionEnqueueResult {
	w.mu.Lock()
	defer w.mu.Unlock()

	b := w.Buildings[buildingID]
	if b == nil || !b.IsAlive() {
		return ProductionEnqueueProducerUnavailable
	}
	if !b.IsComplete() {
		return ProductionEnqueueBuildingUnderConstruction
	}
	if !entity.BuildingCanTrain(b.Kind(), kind) {
		return ProductionEnqueueInvalidProducer
	}
	cost := entity.UnitCost(kind)
	if !w.canAffordLocked(b.Team(), cost) {
		return ProductionEnqueueInsufficientResources
	}
	if w.populationUsedLocked(b.Team())+w.populationReservedLocked(b.Team())+entity.UnitPopulation(kind) > entity.PopulationCap {
		return ProductionEnqueuePopulationCapReached
	}

	w.payLocked(b.Team(), cost)
	b.Enqueue(kind)
	return ProductionEnqueueQueued
}

// ProcessProduction spawns at most one queued unit per building each tick.
func (w *World) ProcessProduction() {
	w.mu.Lock()
	defer w.mu.Unlock()

	occupied := occupiedCoords(w)
	for _, b := range w.Buildings {
		if !b.IsAlive() || !b.IsComplete() || b.QueueLen() == 0 {
			continue
		}
		if !b.AdvanceQueue() {
			continue
		}
		spawn, ok := findFirstOpenSpawnCoord(w, b.Position(), occupied)
		if !ok {
			continue
		}
		kind, ok := b.DequeueNext()
		if !ok {
			continue
		}
		u := entity.NewUnit(w.nextID(), b.Team(), kind, spawn)
		w.Units[u.ID()] = u
		occupied[spawn] = true
	}
}

// ProcessConstruction advances buildings under construction by one tick.
func (w *World) ProcessConstruction() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, b := range w.Buildings {
		if !b.IsAlive() || b.IsComplete() {
			continue
		}
		b.AdvanceConstruction()
	}
}

func (w *World) FindNearestFriendlyTownCenter(team entity.Team, from hex.Coord) (hex.Coord, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	best := hex.Coord{}
	bestDist := 0
	found := false
	for _, b := range w.Buildings {
		if !b.IsAlive() || !b.IsComplete() || b.Team() != team || b.Kind() != entity.KindTownCenter {
			continue
		}
		dist := hex.Distance(from, b.Position())
		if !found || dist < bestDist {
			best = b.Position()
			bestDist = dist
			found = true
		}
	}
	return best, found
}

func (w *World) canAffordLocked(team entity.Team, cost entity.Cost) bool {
	res := w.TeamRes[team]
	return res.Food >= cost.Food &&
		res.Gold >= cost.Gold &&
		res.Stone >= cost.Stone &&
		res.Wood >= cost.Wood
}

func (w *World) populationUsedLocked(team entity.Team) int {
	total := 0
	for _, u := range w.Units {
		if u.IsAlive() && u.Team() == team {
			total += entity.UnitPopulation(u.Kind())
		}
	}
	return total
}

func (w *World) populationReservedLocked(team entity.Team) int {
	total := 0
	for _, b := range w.Buildings {
		if b.IsAlive() && b.Team() == team {
			total += b.ReservedPopulation()
		}
	}
	return total
}

// CanAfford reports whether the team can pay the given cost.
func (w *World) CanAfford(team entity.Team, cost entity.Cost) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.canAffordLocked(team, cost)
}

func (w *World) payLocked(team entity.Team, cost entity.Cost) {
	res := w.TeamRes[team]
	res.Food -= cost.Food
	res.Gold -= cost.Gold
	res.Stone -= cost.Stone
	res.Wood -= cost.Wood
	w.TeamRes[team] = res
}

func (w *World) canUnitOccupyLocked(kind entity.UnitKind, c hex.Coord, ignoreUnitID, ignoreBuildingID entity.EntityID) bool {
	tile, ok := w.Tiles[c]
	if !ok || !entity.UnitCanEnterTerrain(kind, tile.Terrain) {
		return false
	}
	return w.canTileBeOccupiedLocked(c, ignoreUnitID, ignoreBuildingID)
}

func (w *World) canTileBeOccupiedLocked(c hex.Coord, ignoreUnitID, ignoreBuildingID entity.EntityID) bool {
	for id, u := range w.Units {
		if id != ignoreUnitID && u.IsAlive() && u.Position() == c {
			return false
		}
	}
	for id, b := range w.Buildings {
		if id != ignoreBuildingID && b.IsAlive() && b.Position() == c {
			return false
		}
	}
	return true
}

// CanOccupy reports whether a coordinate is currently passable and unoccupied.
func (w *World) CanOccupy(c hex.Coord) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	tile, ok := w.Tiles[c]
	if !ok || !tile.Terrain.Passable() {
		return false
	}
	for _, u := range w.Units {
		if u.IsAlive() && u.Position() == c {
			return false
		}
	}
	for _, b := range w.Buildings {
		if b.IsAlive() && b.Position() == c {
			return false
		}
	}
	return true
}

// CanUnitOccupy reports whether the given unit kind may currently enter c.
func (w *World) CanUnitOccupy(kind entity.UnitKind, c hex.Coord, ignoreUnitID entity.EntityID) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if !hex.InBounds(c) {
		return false
	}
	return w.canUnitOccupyLocked(kind, c, ignoreUnitID, 0)
}

func (w *World) IsGatherableResource(c hex.Coord) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	tile, ok := w.Tiles[c]
	return ok && tile.Terrain.ResourceYield() != terrain.ResourceNone
}

func (w *World) BuildingAt(c hex.Coord) *entity.Building {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return buildingAtLocked(w.Buildings, c)
}

// CanDepositCarry reports whether a villager can deposit carried resources now.
func (w *World) CanDepositCarry(unitID entity.EntityID) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	u := w.Units[unitID]
	if u == nil || !u.IsAlive() || u.CarryAmount() <= 0 {
		return false
	}
	return isAdjacentToFriendlyTownCenter(w.Buildings, u.Team(), u.Position())
}

func findFirstOpenSpawnCoord(w *World, origin hex.Coord, occupied map[hex.Coord]bool) (hex.Coord, bool) {
	for radius := 1; radius <= 3; radius++ {
		for _, c := range hex.Ring(origin, radius) {
			if !hex.InBounds(c) || occupied[c] {
				continue
			}
			tile, ok := w.Tiles[c]
			if !ok || !tile.Terrain.Passable() {
				continue
			}
			return c, true
		}
	}
	return hex.Coord{}, false
}
