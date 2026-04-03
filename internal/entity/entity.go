package entity

import "github.com/jason9075/agents_of_dynasties/internal/hex"

// EntityID uniquely identifies any entity in the world.
type EntityID uint64

// Team identifies which side an entity belongs to.
type Team uint8

const (
	TeamNeutral Team = 0
	Team1       Team = 1
	Team2       Team = 2
)

// Entity is the common interface implemented by Unit and Building.
type Entity interface {
	ID() EntityID
	Team() Team
	Position() hex.Coord
	IsAlive() bool
}
