package terrain

import (
	"encoding/json"
	"fmt"

	"github.com/jason9075/agents_of_dynasties/internal/hex"
)

// ResourceType identifies a gatherable resource.
type ResourceType string

const (
	ResourceNone  ResourceType = ""
	ResourceFood  ResourceType = "food"
	ResourceGold  ResourceType = "gold"
	ResourceStone ResourceType = "stone"
	ResourceWood  ResourceType = "wood"
)

// Type identifies the terrain variant of a map cell.
type Type uint8

const (
	Plain     Type = iota
	Forest         // yields Wood
	Mountain       // impassable
	Lake           // impassable
	GoldMine       // yields Gold
	StoneMine      // yields Stone
	Orchard        // yields Food
	Deer           // yields Food
)

var typeNames = map[Type]string{
	Plain:     "plain",
	Forest:    "forest",
	Mountain:  "mountain",
	Lake:      "lake",
	GoldMine:  "gold_mine",
	StoneMine: "stone_mine",
	Orchard:   "orchard",
	Deer:      "deer",
}

func (t Type) String() string {
	if s, ok := typeNames[t]; ok {
		return s
	}
	return fmt.Sprintf("terrain(%d)", int(t))
}

func (t Type) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// Passable reports whether units may enter this terrain.
func (t Type) Passable() bool {
	return t != Lake && t != Mountain
}

// ResourceYield returns the resource produced when a Villager gathers here.
func (t Type) ResourceYield() ResourceType {
	switch t {
	case Forest:
		return ResourceWood
	case GoldMine:
		return ResourceGold
	case StoneMine:
		return ResourceStone
	case Orchard, Deer:
		return ResourceFood
	default:
		return ResourceNone
	}
}

// Tile is a single cell in the map grid. Immutable after generation.
type Tile struct {
	Coord   hex.Coord `json:"coord"`
	Terrain Type      `json:"terrain"`
}
