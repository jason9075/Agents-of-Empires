package world

import (
	"sort"
	"sync"
	"sync/atomic"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
)

// Resources tracks a team's current stockpile.
type Resources struct {
	Food  int `json:"food"`
	Gold  int `json:"gold"`
	Stone int `json:"stone"`
	Wood  int `json:"wood"`
}

// StartingResources is given to each team at T=0.
var StartingResources = Resources{
	Food:  200,
	Gold:  100,
	Stone: 100,
	Wood:  200,
}

// World holds all mutable game state. All fields are protected by mu.
type World struct {
	mu        sync.RWMutex
	Tiles     map[hex.Coord]terrain.Tile
	Units     map[entity.EntityID]*entity.Unit
	Buildings map[entity.EntityID]*entity.Building
	TeamRes   map[entity.Team]Resources
	Tick      uint64
	idCounter atomic.Uint64
}

// NewWorld creates and seeds a new world using the given seed.
func NewWorld(seed int64) *World {
	w := &World{
		Tiles:     make(map[hex.Coord]terrain.Tile, hex.GridWidth*hex.GridHeight),
		Units:     make(map[entity.EntityID]*entity.Unit),
		Buildings: make(map[entity.EntityID]*entity.Building),
		TeamRes: map[entity.Team]Resources{
			entity.Team1: StartingResources,
			entity.Team2: StartingResources,
		},
	}
	generate(w, seed)
	return w
}

// Tile returns the terrain tile at coord c.
func (w *World) Tile(c hex.Coord) (terrain.Tile, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	t, ok := w.Tiles[c]
	return t, ok
}

// AllTiles returns a stable-ordered slice of all tiles (sorted by Q*GridHeight+R).
func (w *World) AllTiles() []terrain.Tile {
	w.mu.RLock()
	defer w.mu.RUnlock()
	tiles := make([]terrain.Tile, 0, len(w.Tiles))
	for _, t := range w.Tiles {
		tiles = append(tiles, t)
	}
	sort.Slice(tiles, func(i, j int) bool {
		ii := tiles[i].Coord.Q*hex.GridHeight + tiles[i].Coord.R
		jj := tiles[j].Coord.Q*hex.GridHeight + tiles[j].Coord.R
		return ii < jj
	})
	return tiles
}

// UnitsByTeam returns all living units belonging to team t.
func (w *World) UnitsByTeam(t entity.Team) []*entity.Unit {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var out []*entity.Unit
	for _, u := range w.Units {
		if u.Team() == t && u.IsAlive() {
			out = append(out, u)
		}
	}
	return out
}

// BuildingsByTeam returns all living buildings belonging to team t.
func (w *World) BuildingsByTeam(t entity.Team) []*entity.Building {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var out []*entity.Building
	for _, b := range w.Buildings {
		if b.Team() == t && b.IsAlive() {
			out = append(out, b)
		}
	}
	return out
}

// VisibleTo returns own entities and visible enemy entities for the given team.
// Phase 1: returns all entities (LOS masking added in Phase 2).
func (w *World) VisibleTo(team entity.Team) (ownUnits []*entity.Unit, ownBuildings []*entity.Building, enemyUnits []*entity.Unit, enemyBuildings []*entity.Building) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Build friendly LOS circles (Phase 2 will use this).
	losCoords := w.losCircle(team)

	for _, u := range w.Units {
		if !u.IsAlive() {
			continue
		}
		if u.Team() == team {
			ownUnits = append(ownUnits, u)
		} else if losCoords[u.Position()] {
			enemyUnits = append(enemyUnits, u)
		}
	}
	for _, b := range w.Buildings {
		if !b.IsAlive() {
			continue
		}
		if b.Team() == team {
			ownBuildings = append(ownBuildings, b)
		} else if losCoords[b.Position()] {
			enemyBuildings = append(enemyBuildings, b)
		}
	}
	return
}

// losCircle returns the set of coords visible to the given team.
// Must be called with at least RLock held.
func (w *World) losCircle(team entity.Team) map[hex.Coord]bool {
	visible := make(map[hex.Coord]bool)
	for _, u := range w.Units {
		if u.Team() != team || !u.IsAlive() {
			continue
		}
		for _, c := range hex.Circle(u.Position(), u.Stats().LOS) {
			visible[c] = true
		}
	}
	for _, b := range w.Buildings {
		if b.Team() != team || !b.IsAlive() {
			continue
		}
		// Buildings have a fixed LOS of 3.
		for _, c := range hex.Circle(b.Position(), 3) {
			visible[c] = true
		}
	}
	return visible
}

// GetTick returns the current tick number.
func (w *World) GetTick() uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Tick
}

// IncrementTick advances the tick counter. Called by the Ticker under write lock.
func (w *World) IncrementTick() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Tick++
}

// GetResources returns a copy of the given team's resources.
func (w *World) GetResources(team entity.Team) Resources {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.TeamRes[team]
}

// GetUnit returns the unit with the given ID, or nil.
func (w *World) GetUnit(id entity.EntityID) *entity.Unit {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Units[id]
}

// GetBuilding returns the building with the given ID, or nil.
func (w *World) GetBuilding(id entity.EntityID) *entity.Building {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Buildings[id]
}

// WriteFunc runs f under a write lock. Used by the Ticker to batch-update state.
func (w *World) WriteFunc(f func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	f()
}

// nextID generates a new unique EntityID. Safe for concurrent use.
func (w *World) nextID() entity.EntityID {
	return entity.EntityID(w.idCounter.Add(1))
}

// addUnit registers a unit (called during generation, no lock needed).
func (w *World) addUnit(u *entity.Unit) {
	w.Units[u.ID()] = u
}

// addBuilding registers a building (called during generation, no lock needed).
func (w *World) addBuilding(b *entity.Building) {
	w.Buildings[b.ID()] = b
}

// SpawnUnit adds a unit under the write lock.
func (w *World) SpawnUnit(team entity.Team, kind entity.UnitKind, pos hex.Coord) *entity.Unit {
	w.mu.Lock()
	defer w.mu.Unlock()
	u := entity.NewUnit(w.nextID(), team, kind, pos)
	w.Units[u.ID()] = u
	return u
}

// SpawnBuilding adds a building under the write lock.
func (w *World) SpawnBuilding(team entity.Team, kind entity.BuildingKind, pos hex.Coord) *entity.Building {
	w.mu.Lock()
	defer w.mu.Unlock()
	b := entity.NewBuilding(w.nextID(), team, kind, pos)
	w.Buildings[b.ID()] = b
	return b
}
