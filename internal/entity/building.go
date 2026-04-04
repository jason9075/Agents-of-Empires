package entity

import "github.com/jason9075/agents_of_dynasties/internal/hex"

// BuildingKind identifies the type of a building.
type BuildingKind uint8

const (
	KindTownCenter BuildingKind = iota
	KindBarracks
	KindStable
	KindArcheryRange
)

func (k BuildingKind) String() string {
	if spec, ok := BuildingSpecs[k]; ok {
		return spec.Name
	}
	return "unknown_building"
}

// Building represents a fixed structure on the map.
type Building struct {
	id              EntityID
	team            Team
	kind            BuildingKind
	pos             hex.Coord
	hp              int
	buildProgress   int
	buildTicksTotal int
	queue           []ProductionItem
}

type ProductionItem struct {
	Kind           UnitKind
	TicksRemaining int
}

func NewBuilding(id EntityID, team Team, kind BuildingKind, pos hex.Coord) *Building {
	return &Building{
		id:              id,
		team:            team,
		kind:            kind,
		pos:             pos,
		hp:              BuildingMaxHP(kind),
		buildProgress:   BuildingBuildTicks(kind),
		buildTicksTotal: BuildingBuildTicks(kind),
	}
}

func NewConstruction(id EntityID, team Team, kind BuildingKind, pos hex.Coord) *Building {
	return &Building{
		id:              id,
		team:            team,
		kind:            kind,
		pos:             pos,
		hp:              BuildingMaxHP(kind),
		buildProgress:   0,
		buildTicksTotal: BuildingBuildTicks(kind),
	}
}

func (b *Building) ID() EntityID         { return b.id }
func (b *Building) Team() Team           { return b.team }
func (b *Building) Position() hex.Coord  { return b.pos }
func (b *Building) IsAlive() bool        { return b.hp > 0 }
func (b *Building) Kind() BuildingKind   { return b.kind }
func (b *Building) HP() int              { return b.hp }
func (b *Building) MaxHP() int           { return BuildingMaxHP(b.kind) }
func (b *Building) IsComplete() bool     { return b.buildProgress >= b.buildTicksTotal }
func (b *Building) BuildProgress() int   { return b.buildProgress }
func (b *Building) BuildTicksTotal() int { return b.buildTicksTotal }
func (b *Building) SetHP(hp int)         { b.hp = hp }
func (b *Building) AdvanceConstruction() {
	if b.IsComplete() {
		return
	}
	b.buildProgress++
}

// Enqueue adds a unit kind to the production queue.
func (b *Building) Enqueue(k UnitKind) {
	b.queue = append(b.queue, ProductionItem{
		Kind:           k,
		TicksRemaining: UnitTrainTicks(k),
	})
}

// DequeueNext removes and returns the front of the queue (ok=false if empty).
func (b *Building) DequeueNext() (UnitKind, bool) {
	if len(b.queue) == 0 {
		return 0, false
	}
	k := b.queue[0].Kind
	b.queue = b.queue[1:]
	return k, true
}

// QueueLen returns how many units are queued.
func (b *Building) QueueLen() int { return len(b.queue) }

func (b *Building) ReservedPopulation() int {
	total := 0
	for _, item := range b.queue {
		total += UnitPopulation(item.Kind)
	}
	return total
}

func (b *Building) QueueTicksRemaining() int {
	if len(b.queue) == 0 {
		return 0
	}
	return b.queue[0].TicksRemaining
}

func (b *Building) AdvanceQueue() bool {
	if len(b.queue) == 0 {
		return false
	}
	if b.queue[0].TicksRemaining > 0 {
		b.queue[0].TicksRemaining--
	}
	return b.queue[0].TicksRemaining <= 0
}
