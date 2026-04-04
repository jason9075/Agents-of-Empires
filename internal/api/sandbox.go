package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
	"github.com/jason9075/agents_of_dynasties/internal/ticker"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

type sandboxPresetSummary struct {
	ID               string               `json:"id"`
	Name             string               `json:"name"`
	Description      string               `json:"description"`
	MaxTick          int                  `json:"max_tick"`
	DefaultPlaybackM int                  `json:"default_playback_ms"`
	Actors           []sandboxPresetActor `json:"actors"`
	DefaultTimeline  []sandboxTimelineRow `json:"default_timeline,omitempty"`
}

type sandboxPresetActor struct {
	ID    string      `json:"id"`
	Label string      `json:"label"`
	Team  entity.Team `json:"team"`
}

type sandboxTimelineRow struct {
	RowID        string             `json:"row_id,omitempty"`
	Tick         int                `json:"tick"`
	ActorID      string             `json:"actor_id"`
	Kind         ticker.CommandKind `json:"kind"`
	TargetCoord  *coordView         `json:"target_coord,omitempty"`
	BuildingKind *string            `json:"building_kind,omitempty"`
	UnitKind     *string            `json:"unit_kind,omitempty"`
}

type sandboxPresetsResponse struct {
	Presets []sandboxPresetSummary `json:"presets"`
}

type sandboxSimulationRequest struct {
	PresetID string               `json:"preset_id"`
	MaxTick  int                  `json:"max_tick,omitempty"`
	Timeline []sandboxTimelineRow `json:"timeline,omitempty"`
}

type sandboxValidationIssue struct {
	RowID  string `json:"row_id,omitempty"`
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

type sandboxSnapshot struct {
	Tick  uint64        `json:"tick"`
	Team1 fullStateTeam `json:"team1"`
	Team2 fullStateTeam `json:"team2"`
	Notes []string      `json:"notes,omitempty"`
}

type sandboxSimulationResponse struct {
	Preset           sandboxPresetSummary     `json:"preset"`
	Map              mapResponse              `json:"map"`
	Snapshots        []sandboxSnapshot        `json:"snapshots"`
	ValidationIssues []sandboxValidationIssue `json:"validation_issues,omitempty"`
}

type sandboxPresetsHandler struct{}

type sandboxSimulateHandler struct{}

type sandboxPresetDefinition struct {
	ID               string
	Name             string
	Description      string
	Width            int
	Height           int
	DefaultPlaybackM int
	MaxTick          int
	Tiles            map[hex.Coord]terrain.Type
	TeamResources    map[entity.Team]world.Resources
	Units            []sandboxUnitSeed
	Buildings        []sandboxBuildingSeed
	DefaultTimeline  []sandboxTimelineRow
}

const sandboxSimulationTickLimit = 2000

type sandboxUnitSeed struct {
	ActorID  string
	Label    string
	EntityID entity.EntityID
	Team     entity.Team
	Kind     entity.UnitKind
	Position hex.Coord
}

type sandboxBuildingSeed struct {
	EntityID entity.EntityID
	Team     entity.Team
	Kind     entity.BuildingKind
	Position hex.Coord
	Complete bool
}

type sandboxActorRef struct {
	ID     entity.EntityID
	Team   entity.Team
	IsUnit bool
}

var sandboxPresetRegistry = map[string]sandboxPresetDefinition{
	"villager_move_then_build": newVillagerMoveThenBuildPreset(),
}

func (h *sandboxPresetsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	ids := make([]string, 0, len(sandboxPresetRegistry))
	for id := range sandboxPresetRegistry {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	resp := sandboxPresetsResponse{Presets: make([]sandboxPresetSummary, 0, len(ids))}
	for _, id := range ids {
		resp.Presets = append(resp.Presets, sandboxPresetRegistry[id].summary())
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *sandboxSimulateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	var req sandboxSimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(req.PresetID) == "" {
		writeError(w, http.StatusBadRequest, "missing_preset_id", "preset_id is required")
		return
	}

	preset, ok := sandboxPresetRegistry[req.PresetID]
	if !ok {
		writeError(w, http.StatusNotFound, "preset_not_found", "sandbox preset does not exist")
		return
	}

	maxTick := preset.MaxTick
	if req.MaxTick > 0 {
		maxTick = req.MaxTick
	}
	if maxTick < 1 {
		writeError(w, http.StatusBadRequest, "invalid_max_tick", "max_tick must be at least 1")
		return
	}
	if maxTick > sandboxSimulationTickLimit {
		writeError(w, http.StatusBadRequest, "max_tick_too_large", fmt.Sprintf("max_tick must be <= %d", sandboxSimulationTickLimit))
		return
	}

	timeline := req.Timeline
	if len(timeline) == 0 {
		timeline = cloneSandboxTimeline(preset.DefaultTimeline)
	}

	resp := runSandboxSimulation(preset, timeline, maxTick)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func newVillagerMoveThenBuildPreset() sandboxPresetDefinition {
	tiles := make(map[hex.Coord]terrain.Type)
	for q := 0; q < 8; q++ {
		for r := 0; r < 6; r++ {
			tiles[hex.Coord{Q: q, R: r}] = terrain.Plain
		}
	}

	tiles[hex.Coord{Q: 0, R: 0}] = terrain.Forest
	tiles[hex.Coord{Q: 1, R: 0}] = terrain.Forest
	tiles[hex.Coord{Q: 2, R: 0}] = terrain.Forest
	tiles[hex.Coord{Q: 6, R: 0}] = terrain.GoldMine
	tiles[hex.Coord{Q: 7, R: 0}] = terrain.StoneMine
	tiles[hex.Coord{Q: 0, R: 4}] = terrain.Orchard
	tiles[hex.Coord{Q: 1, R: 4}] = terrain.Deer
	tiles[hex.Coord{Q: 6, R: 4}] = terrain.Lake
	tiles[hex.Coord{Q: 6, R: 5}] = terrain.Lake
	tiles[hex.Coord{Q: 7, R: 4}] = terrain.Mountain
	tiles[hex.Coord{Q: 7, R: 5}] = terrain.Mountain

	moveTarget := coordView{Q: 5, R: 2}
	buildTarget := coordView{Q: 6, R: 2}
	buildingKind := entity.KindBarracks.String()

	return sandboxPresetDefinition{
		ID:               "villager_move_then_build",
		Name:             "Villager move then build",
		Description:      "Uses live server movement and building logic: the villager receives MOVE_GUARD on ticks 1-3, then starts constructing a barracks on tick 4.",
		Width:            8,
		Height:           6,
		DefaultPlaybackM: 800,
		MaxTick:          6,
		Tiles:            tiles,
		TeamResources: map[entity.Team]world.Resources{
			entity.Team1: {Food: 200, Wood: 200, Gold: 100, Stone: 100},
			entity.Team2: {Food: 200, Wood: 200, Gold: 100, Stone: 100},
		},
		Units: []sandboxUnitSeed{
			{
				ActorID:  "villager_1",
				Label:    "villager_1 (villager)",
				EntityID: 1001,
				Team:     entity.Team1,
				Kind:     entity.KindVillager,
				Position: hex.Coord{Q: 2, R: 2},
			},
		},
		Buildings: []sandboxBuildingSeed{
			{
				EntityID: 2001,
				Team:     entity.Team1,
				Kind:     entity.KindTownCenter,
				Position: hex.Coord{Q: 1, R: 2},
				Complete: true,
			},
		},
		DefaultTimeline: []sandboxTimelineRow{
			{
				RowID:       "move_1",
				Tick:        1,
				ActorID:     "villager_1",
				Kind:        ticker.CmdMoveGuard,
				TargetCoord: &moveTarget,
			},
			{
				RowID:       "move_2",
				Tick:        2,
				ActorID:     "villager_1",
				Kind:        ticker.CmdMoveGuard,
				TargetCoord: &moveTarget,
			},
			{
				RowID:       "move_3",
				Tick:        3,
				ActorID:     "villager_1",
				Kind:        ticker.CmdMoveGuard,
				TargetCoord: &moveTarget,
			},
			{
				RowID:        "build_1",
				Tick:         4,
				ActorID:      "villager_1",
				Kind:         ticker.CmdBuild,
				TargetCoord:  &buildTarget,
				BuildingKind: &buildingKind,
			},
		},
	}
}

func (p sandboxPresetDefinition) summary() sandboxPresetSummary {
	return p.summaryForMaxTick(p.MaxTick)
}

func (p sandboxPresetDefinition) summaryForMaxTick(maxTick int) sandboxPresetSummary {
	actors := make([]sandboxPresetActor, 0, len(p.Units))
	for _, unit := range p.Units {
		if unit.ActorID == "" {
			continue
		}
		actors = append(actors, sandboxPresetActor{
			ID:    unit.ActorID,
			Label: unit.Label,
			Team:  unit.Team,
		})
	}
	sort.Slice(actors, func(i, j int) bool { return actors[i].ID < actors[j].ID })

	return sandboxPresetSummary{
		ID:               p.ID,
		Name:             p.Name,
		Description:      p.Description,
		MaxTick:          maxTick,
		DefaultPlaybackM: p.DefaultPlaybackM,
		Actors:           actors,
		DefaultTimeline:  cloneSandboxTimeline(p.DefaultTimeline),
	}
}

func cloneSandboxTimeline(rows []sandboxTimelineRow) []sandboxTimelineRow {
	out := make([]sandboxTimelineRow, 0, len(rows))
	for _, row := range rows {
		copyRow := row
		if row.TargetCoord != nil {
			target := *row.TargetCoord
			copyRow.TargetCoord = &target
		}
		if row.BuildingKind != nil {
			kind := *row.BuildingKind
			copyRow.BuildingKind = &kind
		}
		if row.UnitKind != nil {
			kind := *row.UnitKind
			copyRow.UnitKind = &kind
		}
		out = append(out, copyRow)
	}
	return out
}

func runSandboxSimulation(preset sandboxPresetDefinition, timeline []sandboxTimelineRow, maxTick int) sandboxSimulationResponse {
	w, actorRefs := newSandboxWorld(preset)
	q := ticker.NewQueue()
	tk := ticker.New(w, q, time.Second)
	cmdHandler := &commandHandler{w: w, q: q}

	rowsByTick := make(map[int][]sandboxTimelineRow)
	for _, row := range timeline {
		rowsByTick[row.Tick] = append(rowsByTick[row.Tick], row)
	}

	resp := sandboxSimulationResponse{
		Preset:    preset.summaryForMaxTick(maxTick),
		Map:       sandboxMapForPreset(preset),
		Snapshots: []sandboxSnapshot{{Tick: 0, Team1: snapshotTeamData(w, entity.Team1), Team2: snapshotTeamData(w, entity.Team2), Notes: []string{"Initial preset state."}}},
	}

	for tickNum := 1; tickNum <= maxTick; tickNum++ {
		issuedNotes := make([]string, 0)
		for _, row := range rowsByTick[tickNum] {
			cmd, issue, note := sandboxRowToCommand(preset, maxTick, actorRefs, row, cmdHandler)
			if issue != nil {
				resp.ValidationIssues = append(resp.ValidationIssues, *issue)
				continue
			}
			q.Submit(cmd)
			issuedNotes = append(issuedNotes, note)
		}

		before := resp.Snapshots[len(resp.Snapshots)-1]
		tk.Step()
		after := sandboxSnapshot{
			Tick:  uint64(tickNum),
			Team1: snapshotTeamData(w, entity.Team1),
			Team2: snapshotTeamData(w, entity.Team2),
		}
		after.Notes = buildSandboxNotes(before, after, issuedNotes)
		resp.Snapshots = append(resp.Snapshots, after)
	}

	sort.Slice(resp.ValidationIssues, func(i, j int) bool {
		if resp.ValidationIssues[i].RowID != resp.ValidationIssues[j].RowID {
			return resp.ValidationIssues[i].RowID < resp.ValidationIssues[j].RowID
		}
		return resp.ValidationIssues[i].Code < resp.ValidationIssues[j].Code
	})

	return resp
}

func newSandboxWorld(preset sandboxPresetDefinition) (*world.World, map[string]sandboxActorRef) {
	w := world.NewWorld(1)
	actorRefs := make(map[string]sandboxActorRef)

	w.WriteFunc(func() {
		w.Tiles = make(map[hex.Coord]terrain.Tile, hex.GridWidth*hex.GridHeight)
		w.ResourceRemaining = make(map[hex.Coord]int)
		w.Units = make(map[entity.EntityID]*entity.Unit)
		w.Buildings = make(map[entity.EntityID]*entity.Building)
		w.TeamRes = map[entity.Team]world.Resources{
			entity.Team1: preset.TeamResources[entity.Team1],
			entity.Team2: preset.TeamResources[entity.Team2],
		}
		w.Tick = 0

		for q := 0; q < hex.GridWidth; q++ {
			for r := 0; r < hex.GridHeight; r++ {
				coord := hex.Coord{Q: q, R: r}
				kind := terrain.Mountain
				if q < preset.Width && r < preset.Height {
					if presetKind, ok := preset.Tiles[coord]; ok {
						kind = presetKind
					} else {
						kind = terrain.Plain
					}
				}
				w.Tiles[coord] = terrain.Tile{Coord: coord, Terrain: kind}
				if capacity := entity.ResourceCapacity(kind); capacity > 0 {
					w.ResourceRemaining[coord] = capacity
				}
			}
		}

		for _, seed := range preset.Units {
			unit := entity.NewUnit(seed.EntityID, seed.Team, seed.Kind, seed.Position)
			w.Units[unit.ID()] = unit
			if seed.ActorID != "" {
				actorRefs[seed.ActorID] = sandboxActorRef{
					ID:     unit.ID(),
					Team:   seed.Team,
					IsUnit: true,
				}
			}
		}

		for _, seed := range preset.Buildings {
			var building *entity.Building
			if seed.Complete {
				building = entity.NewBuilding(seed.EntityID, seed.Team, seed.Kind, seed.Position)
			} else {
				building = entity.NewConstruction(seed.EntityID, seed.Team, seed.Kind, seed.Position)
			}
			w.Buildings[building.ID()] = building
		}
	})

	return w, actorRefs
}

func sandboxMapForPreset(preset sandboxPresetDefinition) mapResponse {
	tiles := make([]tileView, 0, preset.Width*preset.Height)
	for q := 0; q < preset.Width; q++ {
		for r := 0; r < preset.Height; r++ {
			coord := hex.Coord{Q: q, R: r}
			kind := terrain.Plain
			if presetKind, ok := preset.Tiles[coord]; ok {
				kind = presetKind
			}
			tiles = append(tiles, tileView{
				Coord:     coordView{Q: q, R: r},
				Terrain:   kind.String(),
				Remaining: entity.ResourceCapacity(kind),
			})
		}
	}
	return mapResponse{
		Width:  preset.Width,
		Height: preset.Height,
		Tiles:  tiles,
	}
}

func sandboxRowToCommand(preset sandboxPresetDefinition, maxTick int, refs map[string]sandboxActorRef, row sandboxTimelineRow, h *commandHandler) (ticker.Command, *sandboxValidationIssue, string) {
	if row.Tick < 1 || row.Tick > maxTick {
		return ticker.Command{}, &sandboxValidationIssue{
			RowID:  row.RowID,
			Code:   "invalid_tick",
			Reason: fmt.Sprintf("tick must be between 1 and %d", maxTick),
		}, ""
	}

	ref, ok := refs[row.ActorID]
	if !ok {
		return ticker.Command{}, &sandboxValidationIssue{
			RowID:  row.RowID,
			Code:   "unknown_actor",
			Reason: "actor_id does not exist in the selected preset",
		}, ""
	}

	cmd := ticker.Command{
		Team: ref.Team,
		Kind: row.Kind,
	}

	if ref.IsUnit {
		cmd.UnitID = ref.ID
	} else {
		cmd.BuildingID = &ref.ID
	}

	if row.TargetCoord != nil {
		if row.TargetCoord.Q < 0 || row.TargetCoord.Q >= preset.Width || row.TargetCoord.R < 0 || row.TargetCoord.R >= preset.Height {
			return ticker.Command{}, &sandboxValidationIssue{
				RowID:  row.RowID,
				Code:   "target_out_of_bounds",
				Reason: "target_coord is outside the preset map",
			}, ""
		}
		target := hex.Coord{Q: row.TargetCoord.Q, R: row.TargetCoord.R}
		cmd.TargetCoord = &target
	}

	if row.BuildingKind != nil {
		kind := strings.TrimSpace(*row.BuildingKind)
		cmd.BuildingKind = &kind
	}
	if row.UnitKind != nil {
		kind := strings.TrimSpace(*row.UnitKind)
		cmd.UnitKind = &kind
	}

	if ref.IsUnit {
		unit := h.w.GetUnit(cmd.UnitID)
		if unit == nil {
			return ticker.Command{}, &sandboxValidationIssue{
				RowID:  row.RowID,
				Code:   "unit_not_found",
				Reason: "referenced unit does not exist",
			}, ""
		}
		if unit.Team() != cmd.Team {
			return ticker.Command{}, &sandboxValidationIssue{
				RowID:  row.RowID,
				Code:   "unit_wrong_team",
				Reason: "unit does not belong to the actor team",
			}, ""
		}
	} else if cmd.BuildingID != nil {
		building := h.w.GetBuilding(*cmd.BuildingID)
		if building == nil {
			return ticker.Command{}, &sandboxValidationIssue{
				RowID:  row.RowID,
				Code:   "building_not_found",
				Reason: "referenced building does not exist",
			}, ""
		}
		if building.Team() != cmd.Team {
			return ticker.Command{}, &sandboxValidationIssue{
				RowID:  row.RowID,
				Code:   "building_wrong_team",
				Reason: "building does not belong to the actor team",
			}, ""
		}
	}

	if status, code, reason := h.validateCommand(cmd); status != 0 {
		return ticker.Command{}, &sandboxValidationIssue{
			RowID:  row.RowID,
			Code:   code,
			Reason: reason,
		}, ""
	}

	return cmd, nil, sandboxIssuedCommandNote(row)
}

func sandboxIssuedCommandNote(row sandboxTimelineRow) string {
	switch row.Kind {
	case ticker.CmdBuild:
		if row.TargetCoord == nil || row.BuildingKind == nil {
			return fmt.Sprintf("%s issues BUILD.", row.ActorID)
		}
		return fmt.Sprintf("%s issues BUILD %s at (%d, %d).", row.ActorID, *row.BuildingKind, row.TargetCoord.Q, row.TargetCoord.R)
	case ticker.CmdMoveFast, ticker.CmdMoveGuard:
		if row.TargetCoord == nil {
			return fmt.Sprintf("%s issues %s.", row.ActorID, row.Kind)
		}
		return fmt.Sprintf("%s issues %s toward (%d, %d).", row.ActorID, row.Kind, row.TargetCoord.Q, row.TargetCoord.R)
	case ticker.CmdGather:
		return fmt.Sprintf("%s issues GATHER.", row.ActorID)
	case ticker.CmdProduce:
		if row.UnitKind == nil {
			return fmt.Sprintf("%s issues PRODUCE.", row.ActorID)
		}
		return fmt.Sprintf("%s issues PRODUCE %s.", row.ActorID, *row.UnitKind)
	default:
		return fmt.Sprintf("%s issues %s.", row.ActorID, row.Kind)
	}
}

func snapshotTeamData(w *world.World, team entity.Team) fullStateTeam {
	units := w.UnitsByTeam(team)
	sort.Slice(units, func(i, j int) bool { return units[i].ID() < units[j].ID() })

	buildings := w.BuildingsByTeam(team)
	sort.Slice(buildings, func(i, j int) bool { return buildings[i].ID() < buildings[j].ID() })

	unitViews := make([]unitView, 0, len(units))
	for _, u := range units {
		pos := u.Position()
		var attackTargetID *entity.EntityID
		if id, ok := u.AttackTargetID(); ok {
			attackTargetID = &id
		}
		unitViews = append(unitViews, unitView{
			ID:             u.ID(),
			Kind:           u.Kind().String(),
			Team:           u.Team(),
			Position:       coordView{Q: pos.Q, R: pos.R},
			HP:             u.HP(),
			MaxHP:          u.MaxHP(),
			CarryResource:  string(u.CarryType()),
			CarryAmount:    u.CarryAmount(),
			AttackTargetID: attackTargetID,
			Friendly:       true,
		})
	}

	buildingViews := make([]buildingView, 0, len(buildings))
	for _, b := range buildings {
		pos := b.Position()
		buildingViews = append(buildingViews, buildingView{
			ID:                       b.ID(),
			Kind:                     b.Kind().String(),
			Team:                     b.Team(),
			Position:                 coordView{Q: pos.Q, R: pos.R},
			HP:                       b.HP(),
			MaxHP:                    b.MaxHP(),
			Complete:                 b.IsComplete(),
			BuildProgress:            b.BuildProgress(),
			BuildTicksTotal:          b.BuildTicksTotal(),
			ProductionQueueLen:       b.QueueLen(),
			ProductionTicksRemaining: b.QueueTicksRemaining(),
			Friendly:                 true,
		})
	}

	return fullStateTeam{
		Resources:  w.GetResources(team),
		Population: toPopulationView(w.GetPopulationSummary(team)),
		Units:      unitViews,
		Buildings:  buildingViews,
	}
}

func buildSandboxNotes(before, after sandboxSnapshot, issuedNotes []string) []string {
	notes := append([]string{}, issuedNotes...)

	notes = append(notes, diffResourceNotes("Team 1", before.Team1.Resources, after.Team1.Resources)...)
	notes = append(notes, diffResourceNotes("Team 2", before.Team2.Resources, after.Team2.Resources)...)
	notes = append(notes, diffUnitNotes(before.Team1.Units, after.Team1.Units)...)
	notes = append(notes, diffUnitNotes(before.Team2.Units, after.Team2.Units)...)
	notes = append(notes, diffBuildingNotes(before.Team1.Buildings, after.Team1.Buildings)...)
	notes = append(notes, diffBuildingNotes(before.Team2.Buildings, after.Team2.Buildings)...)

	if len(notes) == 0 {
		return []string{"No state change on this tick."}
	}
	return notes
}

func diffResourceNotes(label string, before, after world.Resources) []string {
	parts := make([]string, 0, 4)
	appendDelta := func(name string, before, after int) {
		if before == after {
			return
		}
		delta := after - before
		parts = append(parts, fmt.Sprintf("%s %+d", name, delta))
	}

	appendDelta("food", before.Food, after.Food)
	appendDelta("wood", before.Wood, after.Wood)
	appendDelta("gold", before.Gold, after.Gold)
	appendDelta("stone", before.Stone, after.Stone)

	if len(parts) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("%s resources changed: %s.", label, strings.Join(parts, ", "))}
}

func diffUnitNotes(beforeUnits, afterUnits []unitView) []string {
	beforeByID := make(map[entity.EntityID]unitView, len(beforeUnits))
	for _, unit := range beforeUnits {
		beforeByID[unit.ID] = unit
	}
	afterByID := make(map[entity.EntityID]unitView, len(afterUnits))
	for _, unit := range afterUnits {
		afterByID[unit.ID] = unit
	}

	notes := make([]string, 0)
	for _, unit := range afterUnits {
		if prev, ok := beforeByID[unit.ID]; ok {
			if prev.Position != unit.Position {
				notes = append(notes, fmt.Sprintf("Unit #%d moved to (%d, %d).", unit.ID, unit.Position.Q, unit.Position.R))
			}
			if prev.CarryAmount != unit.CarryAmount || prev.CarryResource != unit.CarryResource {
				if unit.CarryAmount > 0 {
					notes = append(notes, fmt.Sprintf("Unit #%d is now carrying %d %s.", unit.ID, unit.CarryAmount, unit.CarryResource))
				} else if prev.CarryAmount > 0 {
					notes = append(notes, fmt.Sprintf("Unit #%d deposited its carried resources.", unit.ID))
				}
			}
			continue
		}
		notes = append(notes, fmt.Sprintf("Unit #%d spawned at (%d, %d).", unit.ID, unit.Position.Q, unit.Position.R))
	}

	for _, unit := range beforeUnits {
		if _, ok := afterByID[unit.ID]; !ok {
			notes = append(notes, fmt.Sprintf("Unit #%d was removed.", unit.ID))
		}
	}

	return notes
}

func diffBuildingNotes(beforeBuildings, afterBuildings []buildingView) []string {
	beforeByID := make(map[entity.EntityID]buildingView, len(beforeBuildings))
	for _, building := range beforeBuildings {
		beforeByID[building.ID] = building
	}
	afterByID := make(map[entity.EntityID]buildingView, len(afterBuildings))
	for _, building := range afterBuildings {
		afterByID[building.ID] = building
	}

	notes := make([]string, 0)
	for _, building := range afterBuildings {
		prev, ok := beforeByID[building.ID]
		if !ok {
			if building.Complete {
				notes = append(notes, fmt.Sprintf("Building %s appeared at (%d, %d).", building.Kind, building.Position.Q, building.Position.R))
			} else {
				notes = append(notes, fmt.Sprintf("Construction of %s started at (%d, %d).", building.Kind, building.Position.Q, building.Position.R))
			}
			continue
		}
		if !prev.Complete && building.Complete {
			notes = append(notes, fmt.Sprintf("%s completed at (%d, %d).", building.Kind, building.Position.Q, building.Position.R))
			continue
		}
		if building.BuildProgress != prev.BuildProgress && !building.Complete {
			notes = append(notes, fmt.Sprintf("%s progressed to %d/%d.", building.Kind, building.BuildProgress, building.BuildTicksTotal))
		}
		if building.ProductionQueueLen != prev.ProductionQueueLen || building.ProductionTicksRemaining != prev.ProductionTicksRemaining {
			notes = append(notes, fmt.Sprintf("%s queue is now %d item(s), %d tick(s) remaining on the front item.", building.Kind, building.ProductionQueueLen, building.ProductionTicksRemaining))
		}
	}

	for _, building := range beforeBuildings {
		if _, ok := afterByID[building.ID]; !ok {
			notes = append(notes, fmt.Sprintf("Building %s at (%d, %d) was removed.", building.Kind, building.Position.Q, building.Position.R))
		}
	}

	return notes
}
