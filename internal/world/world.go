package world

import (
	"sort"
	"strings"
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

type PopulationSummary struct {
	Used     int `json:"used"`
	Reserved int `json:"reserved"`
	Cap      int `json:"cap"`
}

type TeamAppearance struct {
	Faction string `json:"faction"`
	Variant string `json:"variant"`
}

type CommandFailure struct {
	CommandID     uint64           `json:"command_id"`
	Team          entity.Team      `json:"team"`
	UnitID        *entity.EntityID `json:"unit_id,omitempty"`
	BuildingID    *entity.EntityID `json:"building_id,omitempty"`
	Kind          string           `json:"kind"`
	TargetCoord   *hex.Coord       `json:"target_coord,omitempty"`
	TargetID      *entity.EntityID `json:"target_id,omitempty"`
	BuildingKind  *string          `json:"building_kind,omitempty"`
	UnitKind      *string          `json:"unit_kind,omitempty"`
	SubmittedTick uint64           `json:"submitted_tick"`
	ResolvedTick  uint64           `json:"resolved_tick"`
	Code          string           `json:"code"`
	Reason        string           `json:"reason"`
}

type ContestedHex struct {
	Coord        hex.Coord         `json:"coord"`
	Team1UnitIDs []entity.EntityID `json:"team1_unit_ids,omitempty"`
	Team2UnitIDs []entity.EntityID `json:"team2_unit_ids,omitempty"`
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
	mu                sync.RWMutex
	Tiles             map[hex.Coord]terrain.Tile
	ResourceRemaining map[hex.Coord]int
	Units             map[entity.EntityID]*entity.Unit
	Buildings         map[entity.EntityID]*entity.Building
	TeamRes           map[entity.Team]Resources
	TeamAppearance    map[entity.Team]TeamAppearance
	LastTickFailures  map[entity.Team][]CommandFailure
	LastTickContests  []ContestedHex
	Tick              uint64
	idCounter         atomic.Uint64
	GameOver          bool
	Winner            string
}

// NewWorld creates and seeds a new world using the given seed.
func NewWorld(seed int64) *World {
	w := &World{
		Tiles:             make(map[hex.Coord]terrain.Tile, hex.GridWidth*hex.GridHeight),
		ResourceRemaining: make(map[hex.Coord]int),
		Units:             make(map[entity.EntityID]*entity.Unit),
		Buildings:         make(map[entity.EntityID]*entity.Building),
		TeamRes: map[entity.Team]Resources{
			entity.Team1: StartingResources,
			entity.Team2: StartingResources,
		},
		TeamAppearance: map[entity.Team]TeamAppearance{
			entity.Team1: defaultAppearanceForTeam(entity.Team1),
			entity.Team2: defaultAppearanceForTeam(entity.Team2),
		},
		LastTickFailures: map[entity.Team][]CommandFailure{
			entity.Team1: nil,
			entity.Team2: nil,
		},
		LastTickContests: nil,
		GameOver:         false,
		Winner:           "",
	}
	generate(w, seed)
	return w
}

func defaultAppearanceForTeam(team entity.Team) TeamAppearance {
	switch team {
	case entity.Team1:
		return TeamAppearance{Faction: "linux", Variant: "blue"}
	case entity.Team2:
		return TeamAppearance{Faction: "microsoft", Variant: "red"}
	default:
		return TeamAppearance{Faction: "neutral", Variant: "neutral"}
	}
}

func normalizeAppearance(team entity.Team, appearance TeamAppearance) TeamAppearance {
	out := TeamAppearance{
		Faction: strings.ToLower(strings.TrimSpace(appearance.Faction)),
		Variant: strings.ToLower(strings.TrimSpace(appearance.Variant)),
	}
	def := defaultAppearanceForTeam(team)
	if out.Faction == "" {
		out.Faction = def.Faction
	}
	if out.Variant == "" {
		out.Variant = def.Variant
	}
	return out
}

func (w *World) GetTeamAppearance(team entity.Team) TeamAppearance {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if appearance, ok := w.TeamAppearance[team]; ok {
		return normalizeAppearance(team, appearance)
	}
	return defaultAppearanceForTeam(team)
}

func (w *World) SetTeamAppearance(team entity.Team, appearance TeamAppearance) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.TeamAppearance == nil {
		w.TeamAppearance = make(map[entity.Team]TeamAppearance)
	}
	w.TeamAppearance[team] = normalizeAppearance(team, appearance)
}

// Tile returns the terrain tile at coord c.
func (w *World) Tile(c hex.Coord) (terrain.Tile, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	t, ok := w.Tiles[c]
	return t, ok
}

// ResourceAt returns the remaining resource on a tile, or 0 if none.
func (w *World) ResourceAt(c hex.Coord) int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.ResourceRemaining[c]
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
		for _, dest := range hex.Circle(u.Position(), u.Stats().LOS) {
			for _, c := range hex.Linedraw(u.Position(), dest) {
				if !hex.InBounds(c) {
					break
				}
				visible[c] = true
				if c != u.Position() {
					if t, ok := w.Tiles[c]; ok && t.Terrain.BlocksLOS() {
						break
					}
				}
			}
		}
	}
	for _, b := range w.Buildings {
		if b.Team() != team || !b.IsAlive() || !b.IsComplete() {
			continue
		}
		// Buildings have a fixed LOS of 3.
		for _, dest := range hex.Circle(b.Position(), 3) {
			for _, c := range hex.Linedraw(b.Position(), dest) {
				if !hex.InBounds(c) {
					break
				}
				visible[c] = true
				if c != b.Position() {
					if t, ok := w.Tiles[c]; ok && t.Terrain.BlocksLOS() {
						break
					}
				}
			}
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

func (w *World) IsGameOver() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.GameOver
}

func (w *World) GetWinner() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Winner
}

func (w *World) EvaluateWinCondition() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.GameOver {
		return
	}

	team1HasTC := false
	team2HasTC := false

	for _, b := range w.Buildings {
		if b.IsAlive() && b.Kind() == entity.KindTownCenter {
			if b.Team() == entity.Team1 {
				team1HasTC = true
			} else if b.Team() == entity.Team2 {
				team2HasTC = true
			}
		}
	}

	if !team1HasTC && !team2HasTC {
		w.GameOver = true
		w.Winner = "draw"
	} else if !team1HasTC {
		w.GameOver = true
		w.Winner = string(entity.Team2)
	} else if !team2HasTC {
		w.GameOver = true
		w.Winner = string(entity.Team1)
	}
}

// GetResources returns a copy of the given team's resources.
func (w *World) GetResources(team entity.Team) Resources {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.TeamRes[team]
}

func (w *World) GetLastTickCommandFailures(team entity.Team) []CommandFailure {
	w.mu.RLock()
	defer w.mu.RUnlock()

	failures := w.LastTickFailures[team]
	out := make([]CommandFailure, 0, len(failures))
	for _, failure := range failures {
		out = append(out, cloneCommandFailure(failure))
	}
	return out
}

func (w *World) SetLastTickCommandFailures(team entity.Team, failures []CommandFailure) {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := make([]CommandFailure, 0, len(failures))
	for _, failure := range failures {
		out = append(out, cloneCommandFailure(failure))
	}
	w.LastTickFailures[team] = out
}

func (w *World) GetLastTickContestedHexes() []ContestedHex {
	w.mu.RLock()
	defer w.mu.RUnlock()

	out := make([]ContestedHex, 0, len(w.LastTickContests))
	for _, contest := range w.LastTickContests {
		out = append(out, cloneContestedHex(contest))
	}
	return out
}

func (w *World) GetVisibleLastTickContestedHexes(team entity.Team) []ContestedHex {
	w.mu.RLock()
	defer w.mu.RUnlock()

	visible := w.losCircle(team)
	out := make([]ContestedHex, 0, len(w.LastTickContests))
	for _, contest := range w.LastTickContests {
		if visible[contest.Coord] {
			out = append(out, cloneContestedHex(contest))
		}
	}
	return out
}

func (w *World) SetLastTickContestedHexes(contests []ContestedHex) {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := make([]ContestedHex, 0, len(contests))
	for _, contest := range contests {
		out = append(out, cloneContestedHex(contest))
	}
	w.LastTickContests = out
}

// GetPopulationSummary returns current living and reserved population for a team.
func (w *World) GetPopulationSummary(team entity.Team) PopulationSummary {
	w.mu.RLock()
	defer w.mu.RUnlock()

	summary := PopulationSummary{Cap: entity.PopulationCap}
	for _, u := range w.Units {
		if u.IsAlive() && u.Team() == team {
			summary.Used += entity.UnitPopulation(u.Kind())
		}
	}
	for _, b := range w.Buildings {
		if b.IsAlive() && b.Team() == team {
			summary.Reserved += b.ReservedPopulation()
		}
	}
	return summary
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

func cloneCommandFailure(in CommandFailure) CommandFailure {
	out := in
	if in.UnitID != nil {
		id := *in.UnitID
		out.UnitID = &id
	}
	if in.BuildingID != nil {
		id := *in.BuildingID
		out.BuildingID = &id
	}
	if in.TargetCoord != nil {
		coord := *in.TargetCoord
		out.TargetCoord = &coord
	}
	if in.TargetID != nil {
		id := *in.TargetID
		out.TargetID = &id
	}
	if in.BuildingKind != nil {
		kind := *in.BuildingKind
		out.BuildingKind = &kind
	}
	if in.UnitKind != nil {
		kind := *in.UnitKind
		out.UnitKind = &kind
	}
	return out
}

func cloneContestedHex(in ContestedHex) ContestedHex {
	out := ContestedHex{Coord: in.Coord}
	if len(in.Team1UnitIDs) > 0 {
		out.Team1UnitIDs = append([]entity.EntityID(nil), in.Team1UnitIDs...)
	}
	if len(in.Team2UnitIDs) > 0 {
		out.Team2UnitIDs = append([]entity.EntityID(nil), in.Team2UnitIDs...)
	}
	return out
}
