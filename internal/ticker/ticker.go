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

	t.applySubmittedCommands(cmds, tick)
	t.resolveMovement()
	t.resolveGuardTransitions()
	t.resolveCombat()
	t.resolveEconomy()
	t.world.ProcessProduction()
	t.cleanupAttackStatuses()
	t.world.IncrementTick()
}

func (t *Ticker) applySubmittedCommands(cmds []Command, tick uint64) {
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
			t.world.EnqueueProduction(*cmd.BuildingID, kind)
		case CmdStop:
			if u := t.world.GetUnit(cmd.UnitID); u != nil {
				u.ClearStatus()
			}
		case CmdMoveFast:
			if u := t.world.GetUnit(cmd.UnitID); u != nil && cmd.TargetCoord != nil {
				u.SetMoveStatus(entity.StatusMovingFast, *cmd.TargetCoord)
			}
		case CmdMoveGuard:
			if u := t.world.GetUnit(cmd.UnitID); u != nil && cmd.TargetCoord != nil {
				u.SetMoveStatus(entity.StatusMovingGuard, *cmd.TargetCoord)
			}
		case CmdAttack:
			if u := t.world.GetUnit(cmd.UnitID); u != nil && cmd.TargetID != nil {
				u.SetAttackStatus(*cmd.TargetID)
			}
		case CmdGather:
			if u := t.world.GetUnit(cmd.UnitID); u != nil && cmd.TargetCoord != nil {
				u.SetGatherStatus(*cmd.TargetCoord)
			}
		case CmdBuild:
			if u := t.world.GetUnit(cmd.UnitID); u != nil && cmd.TargetCoord != nil && cmd.BuildingKind != nil {
				kind, ok := entity.ParseBuildingKind(*cmd.BuildingKind)
				if !ok {
					continue
				}
				u.SetBuildStatus(*cmd.TargetCoord, kind)
			}
		}
	}
}

func (t *Ticker) resolveMovement() {
	movePlans := make(map[entity.EntityID]movementPlan)
	remaining := make(map[entity.EntityID]int)
	maxSteps := 0

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

func (t *Ticker) resolveCombat() {
	damage := make(map[entity.EntityID]int)
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
		return newMovementPlan([]hex.Coord{target}, u.Stats().SpeedFast, entity.PhaseMovingToTarget)
	case entity.StatusMovingGuard:
		if _, ok := t.world.FindAutoAttackTarget(u.ID()); ok {
			return movementPlan{}, false
		}
		target, ok := u.StatusTargetCoord()
		if !ok || u.Position() == target {
			return movementPlan{}, false
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
			return newMovementPlan(t.depositTargets(u), u.Stats().SpeedFast, entity.PhaseReturning)
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

func (t *Ticker) buildApproachTargets(u *entity.Unit, target hex.Coord) []hex.Coord {
	var out []hex.Coord
	for _, candidate := range hex.Ring(target, 1) {
		if candidate == u.Position() || t.world.CanUnitOccupy(u.Kind(), candidate, u.ID()) {
			out = append(out, candidate)
		}
	}
	return out
}
