package entity

import "github.com/jason9075/agents_of_dynasties/internal/hex"

// UnitKind identifies the type of a unit.
type UnitKind uint8

const (
	KindVillager     UnitKind = iota
	KindInfantry              // produced by Barracks
	KindSpearman              // produced by Barracks
	KindScoutCavalry          // produced by Stable
	KindPaladin               // produced by Stable
	KindArcher                // produced by Archery Range
)

var unitKindNames = map[UnitKind]string{
	KindVillager:     "villager",
	KindInfantry:     "infantry",
	KindSpearman:     "spearman",
	KindScoutCavalry: "scout_cavalry",
	KindPaladin:      "paladin",
	KindArcher:       "archer",
}

func (k UnitKind) String() string {
	if s, ok := unitKindNames[k]; ok {
		return s
	}
	return "unknown_unit"
}

// UnitStats are the static per-kind numbers.
type UnitStats struct {
	MaxHP       int
	Attack      int
	Defense     int
	SpeedFast   int // hexes per tick (MOVE_FAST)
	SpeedGuard  int // hexes per tick (MOVE_GUARD)
	LOS         int // line-of-sight radius in hexes
}

// DefaultStats holds placeholder values; Phase 3 will tune them.
var DefaultStats = map[UnitKind]UnitStats{
	KindVillager:     {MaxHP: 25, Attack: 3, Defense: 0, SpeedFast: 2, SpeedGuard: 1, LOS: 2},
	KindInfantry:     {MaxHP: 40, Attack: 8, Defense: 3, SpeedFast: 3, SpeedGuard: 2, LOS: 3},
	KindSpearman:     {MaxHP: 45, Attack: 10, Defense: 4, SpeedFast: 2, SpeedGuard: 2, LOS: 3},
	KindScoutCavalry: {MaxHP: 30, Attack: 6, Defense: 1, SpeedFast: 5, SpeedGuard: 3, LOS: 4},
	KindPaladin:      {MaxHP: 70, Attack: 12, Defense: 6, SpeedFast: 3, SpeedGuard: 2, LOS: 3},
	KindArcher:       {MaxHP: 30, Attack: 9, Defense: 1, SpeedFast: 2, SpeedGuard: 2, LOS: 4},
}

// Unit represents a mobile entity on the map.
type Unit struct {
	id   EntityID
	team Team
	kind UnitKind
	pos  hex.Coord
	hp   int
}

func NewUnit(id EntityID, team Team, kind UnitKind, pos hex.Coord) *Unit {
	stats := DefaultStats[kind]
	return &Unit{
		id:   id,
		team: team,
		kind: kind,
		pos:  pos,
		hp:   stats.MaxHP,
	}
}

func (u *Unit) ID() EntityID       { return u.id }
func (u *Unit) Team() Team         { return u.team }
func (u *Unit) Position() hex.Coord { return u.pos }
func (u *Unit) IsAlive() bool      { return u.hp > 0 }
func (u *Unit) Kind() UnitKind     { return u.kind }
func (u *Unit) HP() int            { return u.hp }
func (u *Unit) MaxHP() int         { return DefaultStats[u.kind].MaxHP }
func (u *Unit) SetPosition(c hex.Coord) { u.pos = c }
func (u *Unit) SetHP(hp int)            { u.hp = hp }
func (u *Unit) Stats() UnitStats        { return DefaultStats[u.kind] }
