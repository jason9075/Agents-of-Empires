package entity

import (
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
)

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

func (k UnitKind) String() string {
	if spec, ok := UnitSpecs[k]; ok {
		return spec.Name
	}
	return "unknown_unit"
}

// Unit represents a mobile entity on the map.
type Unit struct {
	id             EntityID
	team           Team
	kind           UnitKind
	pos            hex.Coord
	hp             int
	carryType      terrain.ResourceType
	carryAmount    int
	attackTargetID *EntityID
}

func NewUnit(id EntityID, team Team, kind UnitKind, pos hex.Coord) *Unit {
	stats := UnitSpecs[kind].Stats
	return &Unit{
		id:   id,
		team: team,
		kind: kind,
		pos:  pos,
		hp:   stats.MaxHP,
	}
}

func (u *Unit) ID() EntityID                    { return u.id }
func (u *Unit) Team() Team                      { return u.team }
func (u *Unit) Position() hex.Coord             { return u.pos }
func (u *Unit) IsAlive() bool                   { return u.hp > 0 }
func (u *Unit) Kind() UnitKind                  { return u.kind }
func (u *Unit) HP() int                         { return u.hp }
func (u *Unit) MaxHP() int                      { return UnitSpecs[u.kind].Stats.MaxHP }
func (u *Unit) CarryType() terrain.ResourceType { return u.carryType }
func (u *Unit) CarryAmount() int                { return u.carryAmount }
func (u *Unit) AttackTargetID() (EntityID, bool) {
	if u.attackTargetID == nil {
		return 0, false
	}
	return *u.attackTargetID, true
}
func (u *Unit) SetPosition(c hex.Coord) { u.pos = c }
func (u *Unit) SetHP(hp int)            { u.hp = hp }
func (u *Unit) SetCarry(rt terrain.ResourceType, amount int) {
	u.carryType = rt
	u.carryAmount = amount
}
func (u *Unit) ClearCarry() { u.carryType, u.carryAmount = terrain.ResourceNone, 0 }
func (u *Unit) SetAttackTarget(id EntityID) {
	targetID := id
	u.attackTargetID = &targetID
}
func (u *Unit) ClearAttackTarget() { u.attackTargetID = nil }
func (u *Unit) Stats() UnitStats   { return UnitSpecs[u.kind].Stats }
