package ticker

import (
	"log/slog"
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
	intents  map[entity.EntityID]Command
}

// New creates a Ticker. Call Start to begin the game loop.
func New(w *world.World, q *Queue, interval time.Duration) *Ticker {
	return &Ticker{
		world:    w,
		queue:    q,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		intents:  make(map[entity.EntityID]Command),
	}
}

// Start launches the game loop in a background goroutine.
func (t *Ticker) Start() {
	go t.loop()
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

// step processes one game tick: drains the command queue, applies commands,
// then increments the world tick counter.
func (t *Ticker) step() {
	cmds := t.queue.Drain()

	tick := t.world.GetTick() + 1
	slog.Info("tick", "tick", tick, "commands", len(cmds))

	for _, cmd := range cmds {
		slog.Debug("command",
			"tick", tick,
			"team", cmd.Team,
			"unit_id", cmd.UnitID,
			"building_id", cmd.BuildingID,
			"kind", cmd.Kind,
		)
		t.recordIntent(cmd)
	}

	t.resolveMovement(cmds)
	t.resolveCombat(cmds)
	t.resolveEconomy(cmds)
	t.world.ProcessConstruction()
	t.world.ProcessProduction()
	t.world.IncrementTick()
}

func (t *Ticker) recordIntent(cmd Command) {
	if cmd.Kind == CmdAttack {
		t.intents[cmd.UnitID] = cmd
		if cmd.TargetID != nil {
			t.world.SetUnitAttackTarget(cmd.UnitID, *cmd.TargetID)
		}
		return
	}
	delete(t.intents, cmd.UnitID)
	t.world.ClearUnitAttackTarget(cmd.UnitID)
}

func (t *Ticker) resolveMovement(cmds []Command) {
	moveCmds := make(map[entity.EntityID]Command)
	remaining := make(map[entity.EntityID]int)
	maxSteps := 0

	for _, cmd := range cmds {
		switch cmd.Kind {
		case CmdMoveFast, CmdMoveGuard:
			if cmd.TargetCoord == nil {
				continue
			}
			u := t.world.GetUnit(cmd.UnitID)
			if u == nil {
				continue
			}
			speed := u.Stats().SpeedFast
			if cmd.Kind == CmdMoveGuard {
				speed = u.Stats().SpeedGuard
			}
			moveCmds[cmd.UnitID] = cmd
			remaining[cmd.UnitID] = speed
			if speed > maxSteps {
				maxSteps = speed
			}
		}
	}

	stopped := make(map[entity.EntityID]bool)
	for step := 0; step < maxSteps; step++ {
		proposals := make(map[hex.Coord][]entity.EntityID)
		destByUnit := make(map[entity.EntityID]hex.Coord)

		for unitID, cmd := range moveCmds {
			if stopped[unitID] || remaining[unitID] <= 0 {
				continue
			}
			next, ok := t.world.PreviewMoveStep(unitID, *cmd.TargetCoord)
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
			if remaining[unitID] <= 0 || destByUnit[unitID] == *moveCmds[unitID].TargetCoord {
				stopped[unitID] = true
			}
		}
	}
}

func (t *Ticker) resolveCombat(cmds []Command) {
	damage := make(map[entity.EntityID]int)
	for unitID, cmd := range t.intents {
		if cmd.TargetID == nil {
			delete(t.intents, unitID)
			t.world.ClearUnitAttackTarget(unitID)
			continue
		}
		attacker := t.world.GetUnit(unitID)
		if attacker == nil {
			delete(t.intents, unitID)
			continue
		}
		if amount, ok := t.world.PreviewAttackDamage(unitID, *cmd.TargetID); ok {
			damage[*cmd.TargetID] += amount
			continue
		}
		if t.world.GetUnit(*cmd.TargetID) == nil && t.world.GetBuilding(*cmd.TargetID) == nil {
			delete(t.intents, unitID)
			t.world.ClearUnitAttackTarget(unitID)
		}
	}

	for _, cmd := range cmds {
		if cmd.Kind != CmdMoveGuard {
			continue
		}
		if targetID, ok := t.world.FindAutoAttackTarget(cmd.UnitID); ok {
			targetIDCopy := targetID
			t.intents[cmd.UnitID] = Command{
				Team: cmd.Team, UnitID: cmd.UnitID, Kind: CmdAttack, TargetID: &targetIDCopy,
			}
			t.world.SetUnitAttackTarget(cmd.UnitID, targetID)
			if amount, ok := t.world.PreviewAttackDamage(cmd.UnitID, targetID); ok {
				damage[targetID] += amount
			}
		}
	}

	if len(damage) > 0 {
		t.world.ApplyDamage(damage)
	}
}

func (t *Ticker) resolveEconomy(cmds []Command) {
	for _, cmd := range cmds {
		switch cmd.Kind {
		case CmdGather:
			t.world.GatherAtCurrentTile(cmd.UnitID)
		case CmdBuild:
			if cmd.TargetCoord == nil || cmd.BuildingKind == nil {
				continue
			}
			kind, ok := entity.ParseBuildingKind(*cmd.BuildingKind)
			if !ok {
				continue
			}
			t.world.BuildStructure(cmd.UnitID, kind, *cmd.TargetCoord)
		case CmdProduce:
			if cmd.BuildingID == nil || cmd.UnitKind == nil {
				continue
			}
			kind, ok := entity.ParseUnitKind(*cmd.UnitKind)
			if !ok {
				continue
			}
			t.world.EnqueueProduction(*cmd.BuildingID, kind)
		}
	}
}
