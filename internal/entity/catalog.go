package entity

import "github.com/jason9075/agents_of_dynasties/internal/terrain"

// Cost is the resource requirement for training a unit or constructing a building.
type Cost struct {
	Food  int
	Gold  int
	Stone int
	Wood  int
}

// UnitStats are the static per-kind numbers.
type UnitStats struct {
	MaxHP       int
	Attack      int
	Defense     int
	SpeedFast   int
	SpeedGuard  int
	LOS         int
	AttackRange int
}

type UnitSpec struct {
	Name           string
	Stats          UnitStats
	Cost           Cost
	Population     int
	Producer       BuildingKind
	Gatherer       bool
	Builder        bool
	ForestPassable bool
	CarryCapacity  int
	BuildTicks     int
	TrainTicks     int
}

type BuildingSpec struct {
	Name        string
	MaxHP       int
	Cost        Cost
	Buildable   bool
	BuildTicks  int
	TrainsUnits []UnitKind
}

const PopulationCap = 20

var UnitSpecs = map[UnitKind]UnitSpec{
	KindVillager: {
		Name:           "villager",
		Stats:          UnitStats{MaxHP: 25, Attack: 3, Defense: 0, SpeedFast: 2, SpeedGuard: 1, LOS: 2, AttackRange: 1},
		Cost:           Cost{Food: 50},
		Population:     1,
		Producer:       KindTownCenter,
		Gatherer:       true,
		Builder:        true,
		ForestPassable: true,
		CarryCapacity:  24,
		BuildTicks:     0,
		TrainTicks:     1,
	},
	KindInfantry: {
		Name:           "infantry",
		Stats:          UnitStats{MaxHP: 40, Attack: 8, Defense: 3, SpeedFast: 3, SpeedGuard: 2, LOS: 3, AttackRange: 1},
		Cost:           Cost{Food: 60, Wood: 20},
		Population:     1,
		Producer:       KindBarracks,
		ForestPassable: true,
		TrainTicks:     1,
	},
	KindSpearman: {
		Name:           "spearman",
		Stats:          UnitStats{MaxHP: 45, Attack: 10, Defense: 4, SpeedFast: 2, SpeedGuard: 2, LOS: 3, AttackRange: 1},
		Cost:           Cost{Food: 50, Gold: 10, Wood: 20},
		Population:     1,
		Producer:       KindBarracks,
		ForestPassable: true,
		TrainTicks:     1,
	},
	KindScoutCavalry: {
		Name:           "scout_cavalry",
		Stats:          UnitStats{MaxHP: 30, Attack: 6, Defense: 1, SpeedFast: 5, SpeedGuard: 3, LOS: 4, AttackRange: 1},
		Cost:           Cost{Food: 70, Gold: 20, Wood: 20},
		Population:     1,
		Producer:       KindStable,
		ForestPassable: false,
		TrainTicks:     2,
	},
	KindPaladin: {
		Name:           "paladin",
		Stats:          UnitStats{MaxHP: 70, Attack: 12, Defense: 6, SpeedFast: 3, SpeedGuard: 2, LOS: 3, AttackRange: 1},
		Cost:           Cost{Food: 90, Gold: 45},
		Population:     1,
		Producer:       KindStable,
		ForestPassable: false,
		TrainTicks:     2,
	},
	KindArcher: {
		Name:           "archer",
		Stats:          UnitStats{MaxHP: 30, Attack: 9, Defense: 1, SpeedFast: 2, SpeedGuard: 2, LOS: 4, AttackRange: 2},
		Cost:           Cost{Food: 40, Gold: 15, Wood: 30},
		Population:     1,
		Producer:       KindArcheryRange,
		ForestPassable: true,
		TrainTicks:     1,
	},
}

var BuildingSpecs = map[BuildingKind]BuildingSpec{
	KindTownCenter: {
		Name:        "town_center",
		MaxHP:       600,
		TrainsUnits: []UnitKind{KindVillager},
	},
	KindBarracks: {
		Name:        "barracks",
		MaxHP:       400,
		Cost:        Cost{Stone: 60, Wood: 120},
		Buildable:   true,
		BuildTicks:  2,
		TrainsUnits: []UnitKind{KindInfantry, KindSpearman},
	},
	KindStable: {
		Name:        "stable",
		MaxHP:       400,
		Cost:        Cost{Gold: 20, Stone: 80, Wood: 140},
		Buildable:   true,
		BuildTicks:  2,
		TrainsUnits: []UnitKind{KindScoutCavalry, KindPaladin},
	},
	KindArcheryRange: {
		Name:        "archery_range",
		MaxHP:       350,
		Cost:        Cost{Stone: 60, Wood: 130},
		Buildable:   true,
		BuildTicks:  2,
		TrainsUnits: []UnitKind{KindArcher},
	},
}

var CounterBonusTable = map[UnitKind]map[UnitKind]int{
	KindSpearman: {
		KindScoutCavalry: 8,
		KindPaladin:      8,
	},
	KindArcher: {
		KindSpearman: 4,
	},
	KindScoutCavalry: {
		KindArcher: 4,
	},
}

var ResourceGatherRates = map[terrain.Type]int{
	terrain.Forest:    20,
	terrain.Orchard:   18,
	terrain.Deer:      24,
	terrain.GoldMine:  12,
	terrain.StoneMine: 10,
}

var ResourceNodeCapacity = map[terrain.Type]int{
	terrain.Forest:    300,
	terrain.Orchard:   240,
	terrain.Deer:      120,
	terrain.GoldMine:  400,
	terrain.StoneMine: 350,
}

func ParseUnitKind(s string) (UnitKind, bool) {
	for kind, spec := range UnitSpecs {
		if spec.Name == s {
			return kind, true
		}
	}
	return 0, false
}

func ParseBuildingKind(s string) (BuildingKind, bool) {
	for kind, spec := range BuildingSpecs {
		if spec.Name == s {
			return kind, true
		}
	}
	return 0, false
}

func UnitProducer(kind UnitKind) BuildingKind {
	return UnitSpecs[kind].Producer
}

func AttackRange(kind UnitKind) int {
	return UnitSpecs[kind].Stats.AttackRange
}

func CounterBonus(attacker, defender UnitKind) int {
	return CounterBonusTable[attacker][defender]
}

func UnitCost(kind UnitKind) Cost {
	return UnitSpecs[kind].Cost
}

func UnitPopulation(kind UnitKind) int {
	return UnitSpecs[kind].Population
}

func BuildingCost(kind BuildingKind) Cost {
	return BuildingSpecs[kind].Cost
}

func BuildingMaxHP(kind BuildingKind) int {
	return BuildingSpecs[kind].MaxHP
}

func UnitCanGather(kind UnitKind) bool {
	return UnitSpecs[kind].Gatherer
}

func UnitCanBuild(kind UnitKind) bool {
	return UnitSpecs[kind].Builder
}

func UnitCanEnterTerrain(kind UnitKind, tile terrain.Type) bool {
	if tile == terrain.Mountain || tile == terrain.Lake {
		return false
	}
	if tile == terrain.Forest && !UnitSpecs[kind].ForestPassable {
		return false
	}
	return true
}

func BuildingCanTrain(buildingKind BuildingKind, unitKind UnitKind) bool {
	for _, candidate := range BuildingSpecs[buildingKind].TrainsUnits {
		if candidate == unitKind {
			return true
		}
	}
	return false
}

func UnitCarryCapacity(kind UnitKind) int {
	return UnitSpecs[kind].CarryCapacity
}

func UnitTrainTicks(kind UnitKind) int {
	return UnitSpecs[kind].TrainTicks
}

func BuildingBuildTicks(kind BuildingKind) int {
	return BuildingSpecs[kind].BuildTicks
}

func ResourceGatherAmount(kind terrain.Type) int {
	return ResourceGatherRates[kind]
}

func ResourceCapacity(kind terrain.Type) int {
	return ResourceNodeCapacity[kind]
}
