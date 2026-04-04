package ticker

import (
	"log/slog"
	"sort"
	"time"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

const DefaultInterval = 10 * time.Second

// Ticker drives the game loop, processing commands every interval.
type Ticker struct {
	world    *world.World
	queue    *Queue
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

type movementPlan struct {
	goals   []hex.Coord
	goalSet map[hex.Coord]struct{}
	speed   int
	phase   entity.UnitStatusPhase
}

// New creates a Ticker. Call Start to begin the game loop.
func New(w *world.World, q *Queue, interval time.Duration) *Ticker {
	return &Ticker{
		world:    w,
		queue:    q,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start launches the game loop in a background goroutine.
func (t *Ticker) Start() {
	go t.loop()
}

// Step resolves exactly one game tick synchronously.
// Useful for tests and server-side sandbox simulations.
func (t *Ticker) Step() {
	t.step()
}

// Stop signals the game loop to exit and waits for it to finish.
func (t *Ticker) Stop() {
	close(t.stop)
	<-t.done
}

func (t *Ticker) loop() {
	defer close(t.done)
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.step()
		case <-t.stop:
			return
		}
	}
}

func (t *Ticker) step() {
	cmds := t.queue.Drain()

	tick := t.world.GetTick() + 1
	slog.Info("tick", "tick", tick, "commands", len(cmds))

	failures := t.applySubmittedCommands(cmds, tick)
	contests, contestDamage := t.resolveMovement()
	t.resolveGuardTransitions()
	t.resolveCombat(contestDamage)
	t.resolveEconomy()
	t.world.ProcessProduction()
	t.cleanupAttackStatuses()
	t.world.SetLastTickCommandFailures(entity.Team1, failures[entity.Team1])
	t.world.SetLastTickCommandFailures(entity.Team2, failures[entity.Team2])
	t.world.SetLastTickContestedHexes(contests)
	t.world.IncrementTick()
}

func (t *Ticker) applySubmittedCommands(cmds []Command, tick uint64) map[entity.Team][]world.CommandFailure {
	failures := map[entity.Team][]world.CommandFailure{
		entity.Team1: nil,
		entity.Team2: nil,
	}
	for _, cmd := range cmds {
		slog.Debug("command",
			"tick", tick,
			"team", cmd.Team,
			"unit_id", cmd.UnitID,
			"building_id", cmd.BuildingID,
			"kind", cmd.Kind,
		)

		switch cmd.Kind {
		case CmdProduce:
			if cmd.BuildingID == nil || cmd.UnitKind == nil {
				continue
			}
			kind, ok := entity.ParseUnitKind(*cmd.UnitKind)
			if !ok {
				continue
			}
			switch t.world.TryEnqueueProduction(*cmd.BuildingID, kind) {
			case world.ProductionEnqueueQueued:
			case world.ProductionEnqueueProducerUnavailable:
				failures[cmd.Team] = append(failures[cmd.Team], commandFailure(cmd, tick, "producer_unavailable", "producer building is unavailable at resolution"))
			case world.ProductionEnqueueBuildingUnderConstruction:
				failures[cmd.Team] = append(failures[cmd.Team], commandFailure(cmd, tick, "producer_unavailable", "producer building is still under construction"))
			case world.ProductionEnqueueInvalidProducer:
				failures[cmd.Team] = append(failures[cmd.Team], commandFailure(cmd, tick, "invalid_producer", "this building cannot produce the requested unit kind"))
			case world.ProductionEnqueueInsufficientResources:
				failures[cmd.Team] = append(failures[cmd.Team], commandFailure(cmd, tick, "insufficient_resources_at_resolution", "team cannot afford this unit at resolution"))
			case world.ProductionEnqueuePopulationCapReached:
				failures[cmd.Team] = append(failures[cmd.Team], commandFailure(cmd, tick, "population_cap_reached_at_resolution", "team population cap would be exceeded at resolution"))
			}
		case CmdStop:
			if u := t.world.GetUnit(cmd.UnitID); u != nil {
				u.ClearStatus()
			}
		case CmdMoveFast:
			if cmd.TargetCoord == nil {
				continue
			}
			if failure, failed := t.validateMoveAtResolution(cmd, tick); failed {
				failures[cmd.Team] = append(failures[cmd.Team], failure)
				continue
			}
			if u := t.world.GetUnit(cmd.UnitID); u != nil {
				u.SetMoveStatus(entity.StatusMovingFast, *cmd.TargetCoord)
			}
		case CmdMoveGuard:
			if cmd.TargetCoord == nil {
				continue
			}
			if failure, failed := t.validateMoveAtResolution(cmd, tick); failed {
				failures[cmd.Team] = append(failures[cmd.Team], failure)
				continue
			}
			if u := t.world.GetUnit(cmd.UnitID); u != nil {
				u.SetMoveStatus(entity.StatusMovingGuard, *cmd.TargetCoord)
			}
		case CmdAttack:
			if cmd.TargetID == nil {
				continue
			}
			if failure, failed := t.validateAttackAtResolution(cmd, tick); failed {
				failures[cmd.Team] = append(failures[cmd.Team], failure)
				continue
			}
			if u := t.world.GetUnit(cmd.UnitID); u != nil {
				u.SetAttackStatus(*cmd.TargetID)
			}
		case CmdGather:
			if cmd.TargetCoord == nil {
				continue
			}
			if !t.world.IsGatherableResource(*cmd.TargetCoord) {
				failures[cmd.Team] = append(failures[cmd.Team], commandFailure(cmd, tick, "resource_node_depleted", "target resource node is no longer gatherable at resolution"))
				continue
			}
			if u := t.world.GetUnit(cmd.UnitID); u != nil {
				u.SetGatherStatus(*cmd.TargetCoord)
			}
		case CmdBuild:
			if cmd.TargetCoord == nil || cmd.BuildingKind == nil {
				continue
			}
			kind, ok := entity.ParseBuildingKind(*cmd.BuildingKind)
			if !ok {
				continue
			}
			if failure, failed := t.validateBuildAtResolution(cmd, tick, kind); failed {
				failures[cmd.Team] = append(failures[cmd.Team], failure)
				continue
			}
			if u := t.world.GetUnit(cmd.UnitID); u != nil {
				u.SetBuildStatus(*cmd.TargetCoord, kind)
			}
		}
	}
	return failures
}

func (t *Ticker) resolveMovement() ([]world.ContestedHex, map[entity.EntityID]int) {
	movePlans := make(map[entity.EntityID]movementPlan)
	remaining := make(map[entity.EntityID]int)
	maxSteps := 0
	contestDamage := make(map[entity.EntityID]int)
	var contests []world.ContestedHex

	for _, u := range t.allUnits() {
		plan, ok := t.movementDirective(u)
		if !ok || plan.speed <= 0 || len(plan.goals) == 0 {
			continue
		}
		u.SetStatusPhase(plan.phase)
		movePlans[u.ID()] = plan
		remaining[u.ID()] = plan.speed
		if plan.speed > maxSteps {
			maxSteps = plan.speed
		}
	}

	stopped := make(map[entity.EntityID]bool)
	for step := 0; step < maxSteps; step++ {
		proposals := make(map[hex.Coord][]entity.EntityID)
		destByUnit := make(map[entity.EntityID]hex.Coord)

		for unitID, plan := range movePlans {
			if stopped[unitID] || remaining[unitID] <= 0 {
				continue
			}
			next, ok := t.world.PreviewMoveStepToAny(unitID, plan.goals)
			if !ok {
				stopped[unitID] = true
				continue
			}
			proposals[next] = append(proposals[next], unitID)
			destByUnit[unitID] = next
		}

		accepted := make(map[entity.EntityID]hex.Coord)
		for dest, unitIDs := range proposals {
			if len(unitIDs) != 1 {
				if contest, ok := t.buildContestedHex(dest, unitIDs); ok {
					contests = append(contests, contest)
					for targetID, amount := range t.world.PreviewContestDamage(unitIDs) {
						contestDamage[targetID] += amount
					}
				}
				for _, unitID := range unitIDs {
					stopped[unitID] = true
				}
				continue
			}
			accepted[unitIDs[0]] = dest
		}

		if len(accepted) == 0 {
			break
		}

		t.world.ApplyUnitMoves(accepted)
		for unitID := range accepted {
			remaining[unitID]--
			if remaining[unitID] <= 0 || movePlans[unitID].isGoal(destByUnit[unitID]) {
				stopped[unitID] = true
			}
		}
	}

	return mergeContestedHexes(contests), contestDamage
}

func (t *Ticker) resolveGuardTransitions() {
	for _, u := range t.allUnits() {
		if u.Status() != entity.StatusMovingGuard {
			continue
		}
		if targetID, ok := t.world.FindAutoAttackTarget(u.ID()); ok {
			u.SetAttackStatus(targetID)
			u.SetStatusPhase(entity.PhaseAttacking)
		}
	}
}

func (t *Ticker) resolveCombat(extraDamage map[entity.EntityID]int) {
	damage := make(map[entity.EntityID]int)
	for targetID, amount := range extraDamage {
		damage[targetID] += amount
	}
	for _, u := range t.allUnits() {
		if u.Status() != entity.StatusAttacking {
			continue
		}
		targetID, ok := u.StatusTargetID()
		if !ok {
			u.ClearStatus()
			continue
		}
		if amount, ok := t.world.PreviewAttackDamage(u.ID(), targetID); ok {
			u.SetStatusPhase(entity.PhaseAttacking)
			damage[targetID] += amount
			continue
		}
		if t.targetExists(targetID) {
			u.SetStatusPhase(entity.PhaseClosingToAttack)
			continue
		}
		u.ClearStatus()
	}

	if len(damage) > 0 {
		t.world.ApplyDamage(damage)
	}
}

func (t *Ticker) resolveEconomy() {
	for _, u := range t.allUnits() {
		switch u.Status() {
		case entity.StatusGathering:
			t.resolveGatherStatus(u)
		case entity.StatusBuilding:
			t.resolveBuildStatus(u)
		case entity.StatusMovingFast, entity.StatusMovingGuard:
			if target, ok := u.StatusTargetCoord(); ok && u.Position() == target {
				u.ClearStatus()
			}
		}
	}
}

func (t *Ticker) resolveGatherStatus(u *entity.Unit) {
	target, ok := u.StatusTargetCoord()
	if !ok {
		u.ClearStatus()
		return
	}

	if u.CarryAmount() > 0 {
		if t.world.CanDepositCarry(u.ID()) {
			if t.world.GatherAtCurrentTile(u.ID()) {
				if !t.world.IsGatherableResource(target) {
					u.ClearStatus()
					return
				}
				u.SetStatusPhase(entity.PhaseMovingToResource)
			}
			return
		}
		if _, ok := t.world.FindNearestFriendlyTownCenter(u.Team(), u.Position()); ok {
			u.SetStatusPhase(entity.PhaseReturning)
			return
		}
		u.ClearStatus()
		return
	}

	if !t.world.IsGatherableResource(target) {
		u.ClearStatus()
		return
	}
	if u.Position() != target {
		u.SetStatusPhase(entity.PhaseMovingToResource)
		return
	}
	if t.world.GatherAtCurrentTile(u.ID()) {
		if u.CarryAmount() > 0 {
			u.SetStatusPhase(entity.PhaseReturning)
			return
		}
	}
	if !t.world.IsGatherableResource(target) {
		u.ClearStatus()
	}
}

func (t *Ticker) resolveBuildStatus(u *entity.Unit) {
	target, ok := u.StatusTargetCoord()
	if !ok {
		u.ClearStatus()
		return
	}
	kind, ok := u.StatusBuildingKind()
	if !ok {
		u.ClearStatus()
		return
	}

	building := t.world.BuildingAt(target)
	if building != nil && building.Team() == u.Team() && building.Kind() == kind && building.IsComplete() {
		u.ClearStatus()
		return
	}
	if hex.Distance(u.Position(), target) > 1 {
		u.SetStatusPhase(entity.PhaseMovingToBuild)
		return
	}

	switch t.world.WorkOnBuild(u.ID(), kind, target) {
	case world.BuildActionWorking:
		u.SetStatusPhase(entity.PhaseConstructing)
	case world.BuildActionComplete:
		u.ClearStatus()
	case world.BuildActionBlocked:
		u.SetStatusPhase(entity.PhaseMovingToBuild)
	default:
		u.ClearStatus()
	}
}

func (t *Ticker) cleanupAttackStatuses() {
	for _, u := range t.allUnits() {
		if u.Status() != entity.StatusAttacking {
			continue
		}
		targetID, ok := u.StatusTargetID()
		if !ok || !t.targetExists(targetID) {
			u.ClearStatus()
		}
	}
}

func (t *Ticker) movementDirective(u *entity.Unit) (movementPlan, bool) {
	switch u.Status() {
	case entity.StatusMovingFast:
		target, ok := u.StatusTargetCoord()
		if !ok || u.Position() == target {
			return movementPlan{}, false
		}
		if occupant := t.world.UnitAt(target); occupant != nil && occupant.ID() != u.ID() {
			return newMovementPlan(t.guardApproachTargets(u, target), u.Stats().SpeedFast, entity.PhaseMovingToTarget)
		}
		return newMovementPlan([]hex.Coord{target}, u.Stats().SpeedFast, entity.PhaseMovingToTarget)
	case entity.StatusMovingGuard:
		if _, ok := t.world.FindAutoAttackTarget(u.ID()); ok {
			return movementPlan{}, false
		}
		target, ok := u.StatusTargetCoord()
		if !ok || u.Position() == target {
			return movementPlan{}, false
		}
		if occupant := t.world.UnitAt(target); occupant != nil && occupant.ID() != u.ID() {
			return newMovementPlan(t.guardApproachTargets(u, target), u.Stats().SpeedGuard, entity.PhaseMovingToTarget)
		}
		return newMovementPlan([]hex.Coord{target}, u.Stats().SpeedGuard, entity.PhaseMovingToTarget)
	case entity.StatusAttacking:
		targetID, ok := u.StatusTargetID()
		if !ok {
			return movementPlan{}, false
		}
		if _, ok := t.world.PreviewAttackDamage(u.ID(), targetID); ok {
			return movementPlan{}, false
		}
		targetPos, ok := t.targetPosition(targetID)
		if !ok {
			return movementPlan{}, false
		}
		return newMovementPlan(t.attackApproachTargets(u, targetPos), u.Stats().SpeedGuard, entity.PhaseClosingToAttack)
	case entity.StatusGathering:
		target, ok := u.StatusTargetCoord()
		if !ok {
			return movementPlan{}, false
		}
		if u.CarryAmount() > 0 {
			if t.world.CanDepositCarry(u.ID()) {
				return movementPlan{}, false
			}
			depositTarget, ok := t.bestGatherDepositTarget(u, target)
			if !ok {
				return movementPlan{}, false
			}
			return newMovementPlan([]hex.Coord{depositTarget}, u.Stats().SpeedFast, entity.PhaseReturning)
		}
		if !t.world.IsGatherableResource(target) || u.Position() == target {
			return movementPlan{}, false
		}
		return newMovementPlan([]hex.Coord{target}, u.Stats().SpeedFast, entity.PhaseMovingToResource)
	case entity.StatusBuilding:
		target, ok := u.StatusTargetCoord()
		if !ok {
			return movementPlan{}, false
		}
		if hex.Distance(u.Position(), target) <= 1 {
			return movementPlan{}, false
		}
		return newMovementPlan(t.buildApproachTargets(u, target), u.Stats().SpeedFast, entity.PhaseMovingToBuild)
	default:
		return movementPlan{}, false
	}
}

func (t *Ticker) targetPosition(id entity.EntityID) (hex.Coord, bool) {
	if u := t.world.GetUnit(id); u != nil && u.IsAlive() {
		return u.Position(), true
	}
	if b := t.world.GetBuilding(id); b != nil && b.IsAlive() {
		return b.Position(), true
	}
	return hex.Coord{}, false
}

func (t *Ticker) targetExists(id entity.EntityID) bool {
	_, ok := t.targetPosition(id)
	return ok
}

func (t *Ticker) allUnits() []*entity.Unit {
	units := append(t.world.UnitsByTeam(entity.Team1), t.world.UnitsByTeam(entity.Team2)...)
	sort.Slice(units, func(i, j int) bool { return units[i].ID() < units[j].ID() })
	return units
}

func newMovementPlan(goals []hex.Coord, speed int, phase entity.UnitStatusPhase) (movementPlan, bool) {
	goalSet := make(map[hex.Coord]struct{}, len(goals))
	filtered := make([]hex.Coord, 0, len(goals))
	for _, goal := range goals {
		if !hex.InBounds(goal) {
			continue
		}
		if _, exists := goalSet[goal]; exists {
			continue
		}
		goalSet[goal] = struct{}{}
		filtered = append(filtered, goal)
	}
	if speed <= 0 || len(filtered) == 0 {
		return movementPlan{}, false
	}
	return movementPlan{
		goals:   filtered,
		goalSet: goalSet,
		speed:   speed,
		phase:   phase,
	}, true
}

func (p movementPlan) isGoal(c hex.Coord) bool {
	_, ok := p.goalSet[c]
	return ok
}

func (t *Ticker) attackApproachTargets(u *entity.Unit, target hex.Coord) []hex.Coord {
	var out []hex.Coord
	for _, candidate := range hex.Circle(target, entity.AttackRange(u.Kind())) {
		if t.world.CanUnitOccupy(u.Kind(), candidate, u.ID()) {
			out = append(out, candidate)
		}
	}
	return out
}

func (t *Ticker) depositTargets(u *entity.Unit) []hex.Coord {
	var out []hex.Coord
	for _, b := range t.world.BuildingsByTeam(u.Team()) {
		if !b.IsAlive() || !b.IsComplete() || b.Kind() != entity.KindTownCenter {
			continue
		}
		for _, candidate := range hex.Ring(b.Position(), 1) {
			if candidate == u.Position() || t.world.CanUnitOccupy(u.Kind(), candidate, u.ID()) {
				out = append(out, candidate)
			}
		}
	}
	return out
}

func (t *Ticker) bestGatherDepositTarget(u *entity.Unit, resourceTarget hex.Coord) (hex.Coord, bool) {
	candidates := t.depositTargets(u)
	best := hex.Coord{}
	bestTotal := 0
	bestToDeposit := 0
	bestToResource := 0
	found := false

	for _, candidate := range candidates {
		toDeposit, ok := t.world.ShortestStaticPathDistance(u.Kind(), u.Position(), candidate)
		if !ok {
			continue
		}
		toResource, ok := t.world.ShortestStaticPathDistance(u.Kind(), candidate, resourceTarget)
		if !ok {
			continue
		}

		total := toDeposit + toResource
		if !found ||
			total < bestTotal ||
			(total == bestTotal && toDeposit < bestToDeposit) ||
			(total == bestTotal && toDeposit == bestToDeposit && toResource < bestToResource) ||
			(total == bestTotal && toDeposit == bestToDeposit && toResource == bestToResource && coordLess(candidate, best)) {
			best = candidate
			bestTotal = total
			bestToDeposit = toDeposit
			bestToResource = toResource
			found = true
		}
	}

	return best, found
}

func (t *Ticker) buildApproachTargets(u *entity.Unit, target hex.Coord) []hex.Coord {
	var out []hex.Coord
	for _, candidate := range hex.Ring(target, 1) {
		if candidate == u.Position() || t.world.CanUnitOccupy(u.Kind(), candidate, u.ID()) {
			out = append(out, candidate)
		}
	}
	return out
}

func (t *Ticker) guardApproachTargets(u *entity.Unit, target hex.Coord) []hex.Coord {
	var out []hex.Coord
	for _, candidate := range hex.Ring(target, 1) {
		if candidate == u.Position() || t.world.CanUnitOccupy(u.Kind(), candidate, u.ID()) {
			out = append(out, candidate)
		}
	}
	return out
}

func coordLess(a, b hex.Coord) bool {
	if a.Q != b.Q {
		return a.Q < b.Q
	}
	return a.R < b.R
}

func (t *Ticker) buildContestedHex(dest hex.Coord, unitIDs []entity.EntityID) (world.ContestedHex, bool) {
	contest := world.ContestedHex{Coord: dest}
	for _, unitID := range unitIDs {
		unit := t.world.GetUnit(unitID)
		if unit == nil || !unit.IsAlive() {
			continue
		}
		switch unit.Team() {
		case entity.Team1:
			contest.Team1UnitIDs = append(contest.Team1UnitIDs, unitID)
		case entity.Team2:
			contest.Team2UnitIDs = append(contest.Team2UnitIDs, unitID)
		}
	}
	sort.Slice(contest.Team1UnitIDs, func(i, j int) bool { return contest.Team1UnitIDs[i] < contest.Team1UnitIDs[j] })
	sort.Slice(contest.Team2UnitIDs, func(i, j int) bool { return contest.Team2UnitIDs[i] < contest.Team2UnitIDs[j] })
	return contest, len(contest.Team1UnitIDs) > 0 && len(contest.Team2UnitIDs) > 0
}

func mergeContestedHexes(contests []world.ContestedHex) []world.ContestedHex {
	byCoord := make(map[hex.Coord]*world.ContestedHex, len(contests))
	for _, contest := range contests {
		existing, ok := byCoord[contest.Coord]
		if !ok {
			c := world.ContestedHex{Coord: contest.Coord}
			c.Team1UnitIDs = append(c.Team1UnitIDs, contest.Team1UnitIDs...)
			c.Team2UnitIDs = append(c.Team2UnitIDs, contest.Team2UnitIDs...)
			byCoord[contest.Coord] = &c
			continue
		}
		existing.Team1UnitIDs = append(existing.Team1UnitIDs, contest.Team1UnitIDs...)
		existing.Team2UnitIDs = append(existing.Team2UnitIDs, contest.Team2UnitIDs...)
	}

	out := make([]world.ContestedHex, 0, len(byCoord))
	for _, contest := range byCoord {
		contest.Team1UnitIDs = uniqueSortedEntityIDs(contest.Team1UnitIDs)
		contest.Team2UnitIDs = uniqueSortedEntityIDs(contest.Team2UnitIDs)
		out = append(out, *contest)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Coord.R != out[j].Coord.R {
			return out[i].Coord.R < out[j].Coord.R
		}
		return out[i].Coord.Q < out[j].Coord.Q
	})
	return out
}

func uniqueSortedEntityIDs(ids []entity.EntityID) []entity.EntityID {
	if len(ids) == 0 {
		return nil
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := ids[:1]
	for _, id := range ids[1:] {
		if id != out[len(out)-1] {
			out = append(out, id)
		}
	}
	return out
}

func (t *Ticker) validateMoveAtResolution(cmd Command, tick uint64) (world.CommandFailure, bool) {
	tile, ok := t.world.Tile(*cmd.TargetCoord)
	unit := t.world.GetUnit(cmd.UnitID)
	if unit == nil || !ok || !entity.UnitCanEnterTerrain(unit.Kind(), tile.Terrain) {
		return commandFailure(cmd, tick, "target_not_enterable", "target hex is no longer enterable at resolution"), true
	}
	if building := t.world.BuildingAt(*cmd.TargetCoord); building != nil && building.IsAlive() {
		return commandFailure(cmd, tick, "target_building_occupied", "target hex is occupied by a building at resolution"), true
	}
	return world.CommandFailure{}, false
}

func (t *Ticker) validateAttackAtResolution(cmd Command, tick uint64) (world.CommandFailure, bool) {
	attacker := t.world.GetUnit(cmd.UnitID)
	if attacker == nil {
		return commandFailure(cmd, tick, "target_not_found", "attacking unit no longer exists at resolution"), true
	}
	if targetUnit := t.world.GetUnit(*cmd.TargetID); targetUnit != nil {
		if targetUnit.Team() == attacker.Team() {
			return commandFailure(cmd, tick, "target_became_friendly", "attack target is now friendly at resolution"), true
		}
		return world.CommandFailure{}, false
	}
	if targetBuilding := t.world.GetBuilding(*cmd.TargetID); targetBuilding != nil {
		if targetBuilding.Team() == attacker.Team() {
			return commandFailure(cmd, tick, "target_became_friendly", "attack target is now friendly at resolution"), true
		}
		return world.CommandFailure{}, false
	}
	return commandFailure(cmd, tick, "target_not_found", "attack target no longer exists at resolution"), true
}

func (t *Ticker) validateBuildAtResolution(cmd Command, tick uint64, kind entity.BuildingKind) (world.CommandFailure, bool) {
	switch t.world.BuildTargetStatus(cmd.Team, kind, *cmd.TargetCoord) {
	case world.BuildTargetInvalid:
		if building := t.world.BuildingAt(*cmd.TargetCoord); building != nil && building.IsAlive() {
			return commandFailure(cmd, tick, "incompatible_existing_building", "build target is occupied by an incompatible building at resolution"), true
		}
		return commandFailure(cmd, tick, "invalid_build_tile", "build target is not valid for this building at resolution"), true
	case world.BuildTargetCreate, world.BuildTargetBlocked:
		if !t.world.CanAfford(cmd.Team, entity.BuildingCost(kind)) {
			return commandFailure(cmd, tick, "insufficient_resources_at_resolution", "team cannot afford this building at resolution"), true
		}
	}
	return world.CommandFailure{}, false
}

func commandFailure(cmd Command, tick uint64, code, reason string) world.CommandFailure {
	var unitID *entity.EntityID
	if cmd.BuildingID == nil {
		id := cmd.UnitID
		unitID = &id
	}
	var buildingID *entity.EntityID
	if cmd.BuildingID != nil {
		id := *cmd.BuildingID
		buildingID = &id
	}
	var targetCoord *hex.Coord
	if cmd.TargetCoord != nil {
		coord := *cmd.TargetCoord
		targetCoord = &coord
	}
	var targetID *entity.EntityID
	if cmd.TargetID != nil {
		id := *cmd.TargetID
		targetID = &id
	}
	var buildingKind *string
	if cmd.BuildingKind != nil {
		kind := *cmd.BuildingKind
		buildingKind = &kind
	}
	var unitKind *string
	if cmd.UnitKind != nil {
		kind := *cmd.UnitKind
		unitKind = &kind
	}
	return world.CommandFailure{
		CommandID:     cmd.CommandID,
		Team:          cmd.Team,
		UnitID:        unitID,
		BuildingID:    buildingID,
		Kind:          string(cmd.Kind),
		TargetCoord:   targetCoord,
		TargetID:      targetID,
		BuildingKind:  buildingKind,
		UnitKind:      unitKind,
		SubmittedTick: cmd.SubmittedTick,
		ResolvedTick:  tick,
		Code:          code,
		Reason:        reason,
	}
}
