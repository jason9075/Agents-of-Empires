package entity

import "github.com/jason9075/agents_of_dynasties/internal/hex"

// BuildingKind identifies the type of a building.
type BuildingKind uint8

const (
	KindTownCenter   BuildingKind = iota
	KindBarracks
	KindStable
	KindArcheryRange
)

var buildingKindNames = map[BuildingKind]string{
	KindTownCenter:   "town_center",
	KindBarracks:     "barracks",
	KindStable:       "stable",
	KindArcheryRange: "archery_range",
}

func (k BuildingKind) String() string {
	if s, ok := buildingKindNames[k]; ok {
		return s
	}
	return "unknown_building"
}

// buildingStats holds the static HP for each building kind.
var buildingStats = map[BuildingKind]int{
	KindTownCenter:   600,
	KindBarracks:     400,
	KindStable:       400,
	KindArcheryRange: 350,
}

// Building represents a fixed structure on the map.
type Building struct {
	id    EntityID
	team  Team
	kind  BuildingKind
	pos   hex.Coord
	hp    int
	queue []UnitKind // produce queue; processed in Phase 3
}

func NewBuilding(id EntityID, team Team, kind BuildingKind, pos hex.Coord) *Building {
	return &Building{
		id:   id,
		team: team,
		kind: kind,
		pos:  pos,
		hp:   buildingStats[kind],
	}
}

func (b *Building) ID() EntityID        { return b.id }
func (b *Building) Team() Team          { return b.team }
func (b *Building) Position() hex.Coord { return b.pos }
func (b *Building) IsAlive() bool       { return b.hp > 0 }
func (b *Building) Kind() BuildingKind  { return b.kind }
func (b *Building) HP() int             { return b.hp }
func (b *Building) MaxHP() int          { return buildingStats[b.kind] }
func (b *Building) SetHP(hp int)        { b.hp = hp }

// Enqueue adds a unit kind to the production queue.
func (b *Building) Enqueue(k UnitKind) { b.queue = append(b.queue, k) }

// DequeueNext removes and returns the front of the queue (ok=false if empty).
func (b *Building) DequeueNext() (UnitKind, bool) {
	if len(b.queue) == 0 {
		return 0, false
	}
	k := b.queue[0]
	b.queue = b.queue[1:]
	return k, true
}

// QueueLen returns how many units are queued.
func (b *Building) QueueLen() int { return len(b.queue) }
