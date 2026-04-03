package ticker

import (
	"log/slog"
	"time"

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
// Phase 1: commands are logged but not yet simulated (movement/combat added in Phase 2+).
func (t *Ticker) step() {
	cmds := t.queue.Drain()

	t.world.WriteFunc(func() {
		tick := t.world.Tick + 1
		slog.Info("tick", "tick", tick, "commands", len(cmds))
		for _, cmd := range cmds {
			slog.Debug("command",
				"tick", tick,
				"team", cmd.Team,
				"unit_id", cmd.UnitID,
				"kind", cmd.Kind,
			)
		}
		t.world.Tick = tick
	})
}
