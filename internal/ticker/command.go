package ticker

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
)

// CommandKind identifies the type of action an agent is requesting.
type CommandKind string

const (
	CmdMoveFast  CommandKind = "MOVE_FAST"
	CmdMoveGuard CommandKind = "MOVE_GUARD"
	CmdAttack    CommandKind = "ATTACK"
	CmdGather    CommandKind = "GATHER"
	CmdBuild     CommandKind = "BUILD"
	CmdProduce   CommandKind = "PRODUCE"
	CmdStop      CommandKind = "STOP"
)

// Command is an action submitted by an agent for a specific unit.
type Command struct {
	CommandID     uint64           `json:"command_id"`
	Team          entity.Team      `json:"team"`
	UnitID        entity.EntityID  `json:"unit_id"`
	BuildingID    *entity.EntityID `json:"building_id,omitempty"`
	Kind          CommandKind      `json:"kind"`
	TargetCoord   *hex.Coord       `json:"target_coord,omitempty"`
	TargetID      *entity.EntityID `json:"target_id,omitempty"`
	BuildingKind  *string          `json:"building_kind,omitempty"` // for BUILD
	UnitKind      *string          `json:"unit_kind,omitempty"`     // for PRODUCE
	SubmittedTick uint64           `json:"submitted_tick"`
	ReceivedAt    time.Time        `json:"-"`
}

// Queue holds at most one pending command per actor (last-command-wins).
type Queue struct {
	mu      sync.Mutex
	pending map[string]Command
	nextID  atomic.Uint64
}

// NewQueue creates an empty command queue.
func NewQueue() *Queue {
	return &Queue{pending: make(map[string]Command)}
}

// Submit records cmd, replacing any prior command for the same unit.
func (q *Queue) Submit(cmd Command) Command {
	cmd.ReceivedAt = time.Now()
	if cmd.CommandID == 0 {
		cmd.CommandID = q.nextID.Add(1)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending[commandActorKey(cmd)] = cmd
	return cmd
}

// Drain atomically removes and returns all pending commands.
func (q *Queue) Drain() []Command {
	q.mu.Lock()
	old := q.pending
	q.pending = make(map[string]Command, len(old))
	q.mu.Unlock()

	cmds := make([]Command, 0, len(old))
	for _, c := range old {
		cmds = append(cmds, c)
	}
	sortCommands(cmds)
	return cmds
}

// Snapshot returns the current pending commands in deterministic actor order.
func (q *Queue) Snapshot() []Command {
	q.mu.Lock()
	defer q.mu.Unlock()

	cmds := make([]Command, 0, len(q.pending))
	for _, c := range q.pending {
		cmds = append(cmds, c)
	}
	sortCommands(cmds)
	return cmds
}

func commandActorKey(cmd Command) string {
	if cmd.BuildingID != nil {
		return fmt.Sprintf("b:%d", *cmd.BuildingID)
	}
	return fmt.Sprintf("u:%d", cmd.UnitID)
}

func sortCommands(cmds []Command) {
	sort.Slice(cmds, func(i, j int) bool {
		iType, iID := commandActorSortKey(cmds[i])
		jType, jID := commandActorSortKey(cmds[j])
		if iType != jType {
			return iType < jType
		}
		return iID < jID
	})
}

func commandActorSortKey(cmd Command) (int, entity.EntityID) {
	if cmd.BuildingID != nil {
		return 0, *cmd.BuildingID
	}
	return 1, cmd.UnitID
}
