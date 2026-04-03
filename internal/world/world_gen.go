package world

import (
	"math/rand"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
)

type clusterSpec struct {
	kind    terrain.Type
	count   int
	radius  int
}

var clusters = []clusterSpec{
	{terrain.Forest, 8, 2},
	{terrain.Mountain, 4, 2},
	{terrain.Lake, 3, 1},
	{terrain.GoldMine, 4, 1},
	{terrain.StoneMine, 4, 1},
	{terrain.Orchard, 5, 1},
	{terrain.Deer, 4, 1},
}

// generate fills w.Tiles and populates starting entities for both teams.
func generate(w *World, seed int64) {
	rng := rand.New(rand.NewSource(seed))

	// Fill with plains.
	for q := 0; q < hex.GridSize; q++ {
		for r := 0; r < hex.GridSize; r++ {
			c := hex.Coord{Q: q, R: r}
			w.Tiles[c] = terrain.Tile{Coord: c, Terrain: terrain.Plain}
		}
	}

	// Place terrain clusters.
	for _, spec := range clusters {
		for i := 0; i < spec.count; i++ {
			center := randomCoord(rng)
			for _, c := range hex.Circle(center, spec.radius) {
				if hex.InBounds(c) {
					w.Tiles[c] = terrain.Tile{Coord: c, Terrain: spec.kind}
				}
			}
		}
	}

	// Starting positions: clear a small area around each Town Center.
	tc1Pos := hex.Coord{Q: 4, R: 4}
	tc2Pos := hex.Coord{Q: 16, R: 16}
	clearArea(w, tc1Pos, 2)
	clearArea(w, tc2Pos, 2)

	// Spawn Team 1 starting entities.
	tc1 := entity.NewBuilding(w.nextID(), entity.Team1, entity.KindTownCenter, tc1Pos)
	w.addBuilding(tc1)
	v1 := entity.NewUnit(w.nextID(), entity.Team1, entity.KindVillager, hex.Coord{Q: 5, R: 4})
	w.addUnit(v1)
	v2 := entity.NewUnit(w.nextID(), entity.Team1, entity.KindVillager, hex.Coord{Q: 4, R: 5})
	w.addUnit(v2)

	// Spawn Team 2 starting entities.
	tc2 := entity.NewBuilding(w.nextID(), entity.Team2, entity.KindTownCenter, tc2Pos)
	w.addBuilding(tc2)
	v3 := entity.NewUnit(w.nextID(), entity.Team2, entity.KindVillager, hex.Coord{Q: 15, R: 16})
	w.addUnit(v3)
	v4 := entity.NewUnit(w.nextID(), entity.Team2, entity.KindVillager, hex.Coord{Q: 16, R: 15})
	w.addUnit(v4)
}

func clearArea(w *World, center hex.Coord, radius int) {
	for _, c := range hex.Circle(center, radius) {
		if hex.InBounds(c) {
			w.Tiles[c] = terrain.Tile{Coord: c, Terrain: terrain.Plain}
		}
	}
}

func randomCoord(rng *rand.Rand) hex.Coord {
	return hex.Coord{
		Q: rng.Intn(hex.GridSize),
		R: rng.Intn(hex.GridSize),
	}
}
