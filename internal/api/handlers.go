package api

import (
	"encoding/json"
	"net/http"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
	"github.com/jason9075/agents_of_dynasties/internal/ticker"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

// --- Response shapes ---

type mapResponse struct {
	Width  int        `json:"width"`
	Height int        `json:"height"`
	Tiles  []tileView `json:"tiles"`
}

type tileView struct {
	Coord     coordView `json:"coord"`
	Terrain   string    `json:"terrain"`
	Remaining int       `json:"remaining,omitempty"`
}

type unitView struct {
	ID             entity.EntityID  `json:"id"`
	Kind           string           `json:"kind"`
	Team           entity.Team      `json:"team"`
	Position       coordView        `json:"position"`
	HP             int              `json:"hp"`
	MaxHP          int              `json:"max_hp"`
	CarryResource  string           `json:"carry_resource,omitempty"`
	CarryAmount    int              `json:"carry_amount"`
	AttackTargetID *entity.EntityID `json:"attack_target_id,omitempty"`
	Friendly       bool             `json:"friendly"`
}

type buildingView struct {
	ID                       entity.EntityID `json:"id"`
	Kind                     string          `json:"kind"`
	Team                     entity.Team     `json:"team"`
	Position                 coordView       `json:"position"`
	HP                       int             `json:"hp"`
	MaxHP                    int             `json:"max_hp"`
	Complete                 bool            `json:"complete"`
	BuildProgress            int             `json:"build_progress"`
	BuildTicksTotal          int             `json:"build_ticks_total"`
	ProductionQueueLen       int             `json:"production_queue_len"`
	ProductionTicksRemaining int             `json:"production_ticks_remaining"`
	Friendly                 bool            `json:"friendly"`
}

type coordView struct {
	Q int `json:"q"`
	R int `json:"r"`
}

type populationView struct {
	Used     int `json:"used"`
	Reserved int `json:"reserved"`
	Cap      int `json:"cap"`
}

type stateResponse struct {
	Tick       uint64          `json:"tick"`
	Resources  world.Resources `json:"resources"`
	Population populationView  `json:"population"`
	Units      []unitView      `json:"units"`
	Buildings  []buildingView  `json:"buildings"`
}

// --- Map handler ---

type mapHandler struct {
	w *world.World
}

func (h *mapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	rawTiles := h.w.AllTiles()
	tiles := make([]tileView, 0, len(rawTiles))
	for _, tile := range rawTiles {
		tiles = append(tiles, tileView{
			Coord:     coordView{Q: tile.Coord.Q, R: tile.Coord.R},
			Terrain:   tile.Terrain.String(),
			Remaining: h.w.ResourceAt(tile.Coord),
		})
	}
	resp := mapResponse{
		Width:  hex.GridWidth,
		Height: hex.GridHeight,
		Tiles:  tiles,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- State handler ---

type stateHandler struct {
	w *world.World
}

func (h *stateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	team, err := teamFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_header", err.Error())
		return
	}

	ownUnits, ownBuildings, enemyUnits, enemyBuildings := h.w.VisibleTo(team)

	var units []unitView
	for _, u := range ownUnits {
		pos := u.Position()
		var attackTargetID *entity.EntityID
		if id, ok := u.AttackTargetID(); ok {
			attackTargetID = &id
		}
		units = append(units, unitView{
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
	for _, u := range enemyUnits {
		pos := u.Position()
		var attackTargetID *entity.EntityID
		if id, ok := u.AttackTargetID(); ok {
			attackTargetID = &id
		}
		units = append(units, unitView{
			ID:             u.ID(),
			Kind:           u.Kind().String(),
			Team:           u.Team(),
			Position:       coordView{Q: pos.Q, R: pos.R},
			HP:             u.HP(),
			MaxHP:          u.MaxHP(),
			CarryResource:  string(u.CarryType()),
			CarryAmount:    u.CarryAmount(),
			AttackTargetID: attackTargetID,
			Friendly:       false,
		})
	}

	var buildings []buildingView
	for _, b := range ownBuildings {
		pos := b.Position()
		buildings = append(buildings, buildingView{
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
	for _, b := range enemyBuildings {
		pos := b.Position()
		buildings = append(buildings, buildingView{
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
			Friendly:                 false,
		})
	}

	resp := stateResponse{
		Tick:       h.w.GetTick(),
		Resources:  h.w.GetResources(team),
		Population: toPopulationView(h.w.GetPopulationSummary(team)),
		Units:      units,
		Buildings:  buildings,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- Full state handler (god-mode, no LOS masking) ---

type fullStateTeam struct {
	Resources  world.Resources `json:"resources"`
	Population populationView  `json:"population"`
	Units      []unitView      `json:"units"`
	Buildings  []buildingView  `json:"buildings"`
}

type fullStateResponse struct {
	Tick  uint64        `json:"tick"`
	Team1 fullStateTeam `json:"team1"`
	Team2 fullStateTeam `json:"team2"`
}

type fullStateHandler struct {
	w *world.World
}

func (h *fullStateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	teamData := func(team entity.Team) fullStateTeam {
		var units []unitView
		for _, u := range h.w.UnitsByTeam(team) {
			pos := u.Position()
			var attackTargetID *entity.EntityID
			if id, ok := u.AttackTargetID(); ok {
				attackTargetID = &id
			}
			units = append(units, unitView{
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
		var buildings []buildingView
		for _, b := range h.w.BuildingsByTeam(team) {
			pos := b.Position()
			buildings = append(buildings, buildingView{
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
			Resources:  h.w.GetResources(team),
			Population: toPopulationView(h.w.GetPopulationSummary(team)),
			Units:      units,
			Buildings:  buildings,
		}
	}

	resp := fullStateResponse{
		Tick:  h.w.GetTick(),
		Team1: teamData(entity.Team1),
		Team2: teamData(entity.Team2),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- Command handler ---

type commandHandler struct {
	w *world.World
	q *ticker.Queue
}

func (h *commandHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	team, err := teamFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_header", err.Error())
		return
	}

	var cmd ticker.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON: "+err.Error())
		return
	}
	cmd.Team = team

	switch cmd.Kind {
	case ticker.CmdProduce:
		if cmd.BuildingID == nil {
			writeError(w, http.StatusBadRequest, "missing_building_id", "building_id is required for PRODUCE")
			return
		}
		building := h.w.GetBuilding(*cmd.BuildingID)
		if building == nil {
			writeError(w, http.StatusNotFound, "building_not_found", "building not found")
			return
		}
		if building.Team() != team {
			writeError(w, http.StatusForbidden, "building_wrong_team", "building does not belong to your team")
			return
		}
	default:
		unit := h.w.GetUnit(cmd.UnitID)
		if unit == nil {
			writeError(w, http.StatusNotFound, "unit_not_found", "unit not found")
			return
		}
		if unit.Team() != team {
			writeError(w, http.StatusForbidden, "unit_wrong_team", "unit does not belong to your team")
			return
		}
	}

	if status, code, reason := h.validateCommand(cmd); status != 0 {
		writeError(w, status, code, reason)
		return
	}

	h.q.Submit(cmd)
	w.WriteHeader(http.StatusAccepted)
}

func (h *commandHandler) validateCommand(cmd ticker.Command) (int, string, string) {
	switch cmd.Kind {
	case ticker.CmdMoveFast, ticker.CmdMoveGuard:
		return h.validateMove(cmd)
	case ticker.CmdGather:
		return h.validateGather(cmd)
	case ticker.CmdBuild:
		return h.validateBuild(cmd)
	case ticker.CmdAttack:
		return h.validateAttack(cmd)
	case ticker.CmdProduce:
		return h.validateProduce(cmd)
	default:
		return http.StatusBadRequest, "invalid_command_kind", "unsupported command kind"
	}
}

func (h *commandHandler) validateMove(cmd ticker.Command) (int, string, string) {
	if cmd.TargetCoord == nil {
		return http.StatusBadRequest, "missing_target_coord", "target_coord is required for MOVE commands"
	}
	if !hex.InBounds(*cmd.TargetCoord) {
		return http.StatusBadRequest, "target_out_of_bounds", "target_coord is outside the map"
	}
	unit := h.w.GetUnit(cmd.UnitID)
	tile, ok := h.w.Tile(*cmd.TargetCoord)
	if !ok || !entity.UnitCanEnterTerrain(unit.Kind(), tile.Terrain) {
		return http.StatusBadRequest, "target_not_passable", "target_coord is not passable"
	}
	return 0, "", ""
}

func (h *commandHandler) validateGather(cmd ticker.Command) (int, string, string) {
	unit := h.w.GetUnit(cmd.UnitID)
	if unit.Kind() != entity.KindVillager {
		return http.StatusBadRequest, "unit_cannot_gather", "only villagers can gather resources"
	}
	if h.w.CanDepositCarry(cmd.UnitID) {
		return 0, "", ""
	}
	tile, ok := h.w.Tile(unit.Position())
	if !ok || tile.Terrain.ResourceYield() == terrain.ResourceNone {
		return http.StatusBadRequest, "no_gatherable_resource", "unit is not standing on a gatherable resource tile"
	}
	return 0, "", ""
}

func (h *commandHandler) validateBuild(cmd ticker.Command) (int, string, string) {
	if cmd.TargetCoord == nil {
		return http.StatusBadRequest, "missing_target_coord", "target_coord is required for BUILD"
	}
	if cmd.BuildingKind == nil {
		return http.StatusBadRequest, "missing_building_kind", "building_kind is required for BUILD"
	}
	builder := h.w.GetUnit(cmd.UnitID)
	if builder.Kind() != entity.KindVillager {
		return http.StatusBadRequest, "unit_cannot_build", "only villagers can construct buildings"
	}
	kind, ok := entity.ParseBuildingKind(*cmd.BuildingKind)
	if !ok {
		return http.StatusBadRequest, "invalid_building_kind", "unknown building_kind"
	}
	if kind == entity.KindTownCenter {
		return http.StatusBadRequest, "building_not_allowed", "town_center cannot be built by villagers"
	}
	if !hex.InBounds(*cmd.TargetCoord) {
		return http.StatusBadRequest, "target_out_of_bounds", "target_coord is outside the map"
	}
	if hex.Distance(builder.Position(), *cmd.TargetCoord) > 1 {
		return http.StatusBadRequest, "target_out_of_range", "builder must be adjacent to the build target"
	}
	tile, ok := h.w.Tile(*cmd.TargetCoord)
	if !ok || tile.Terrain != terrain.Plain {
		return http.StatusBadRequest, "invalid_build_tile", "build target must be a plain tile"
	}
	if !h.w.CanOccupy(*cmd.TargetCoord) {
		return http.StatusBadRequest, "target_occupied", "build target is occupied"
	}
	cost := entity.BuildingCost(kind)
	if !h.canAffordWithPending(cmd.Team, cost, cmd) {
		return http.StatusBadRequest, "insufficient_resources", "team cannot afford this building"
	}
	return 0, "", ""
}

func (h *commandHandler) validateAttack(cmd ticker.Command) (int, string, string) {
	if cmd.TargetID == nil {
		return http.StatusBadRequest, "missing_target_id", "target_id is required for ATTACK"
	}
	attacker := h.w.GetUnit(cmd.UnitID)
	if targetUnit := h.w.GetUnit(*cmd.TargetID); targetUnit != nil {
		if targetUnit.Team() == attacker.Team() {
			return http.StatusBadRequest, "friendly_fire_forbidden", "cannot attack a friendly unit"
		}
		if hex.Distance(attacker.Position(), targetUnit.Position()) > entity.AttackRange(attacker.Kind()) {
			return http.StatusBadRequest, "target_out_of_range", "target is outside attack range"
		}
		return 0, "", ""
	}
	if targetBuilding := h.w.GetBuilding(*cmd.TargetID); targetBuilding != nil {
		if targetBuilding.Team() == attacker.Team() {
			return http.StatusBadRequest, "friendly_fire_forbidden", "cannot attack a friendly building"
		}
		if hex.Distance(attacker.Position(), targetBuilding.Position()) > entity.AttackRange(attacker.Kind()) {
			return http.StatusBadRequest, "target_out_of_range", "target is outside attack range"
		}
		return 0, "", ""
	}
	return http.StatusNotFound, "target_not_found", "attack target does not exist"
}

func (h *commandHandler) validateProduce(cmd ticker.Command) (int, string, string) {
	if cmd.UnitKind == nil {
		return http.StatusBadRequest, "missing_unit_kind", "unit_kind is required for PRODUCE"
	}
	building := h.w.GetBuilding(*cmd.BuildingID)
	if !building.IsComplete() {
		return http.StatusBadRequest, "building_under_construction", "building cannot produce units until construction is complete"
	}
	kind, ok := entity.ParseUnitKind(*cmd.UnitKind)
	if !ok {
		return http.StatusBadRequest, "invalid_unit_kind", "unknown unit_kind"
	}
	if !entity.BuildingCanTrain(building.Kind(), kind) {
		return http.StatusBadRequest, "invalid_producer", "this building cannot produce the requested unit kind"
	}
	cost := entity.UnitCost(kind)
	if !h.canAffordWithPending(cmd.Team, cost, cmd) {
		return http.StatusBadRequest, "insufficient_resources", "team cannot afford this unit"
	}
	if !h.canReservePopulationWithPending(cmd.Team, kind, cmd) {
		return http.StatusBadRequest, "population_cap_reached", "team population cap would be exceeded"
	}
	return 0, "", ""
}

func toPopulationView(summary world.PopulationSummary) populationView {
	return populationView{
		Used:     summary.Used,
		Reserved: summary.Reserved,
		Cap:      summary.Cap,
	}
}

func (h *commandHandler) canAffordWithPending(team entity.Team, cost entity.Cost, incoming ticker.Command) bool {
	available := h.w.GetResources(team)
	reserved, _ := h.pendingReservations(team, incoming)
	available.Food -= reserved.Food
	available.Gold -= reserved.Gold
	available.Stone -= reserved.Stone
	available.Wood -= reserved.Wood
	return available.Food >= cost.Food &&
		available.Gold >= cost.Gold &&
		available.Stone >= cost.Stone &&
		available.Wood >= cost.Wood
}

func (h *commandHandler) canReservePopulationWithPending(team entity.Team, unitKind entity.UnitKind, incoming ticker.Command) bool {
	summary := h.w.GetPopulationSummary(team)
	_, pendingPop := h.pendingReservations(team, incoming)
	return summary.Used+summary.Reserved+pendingPop+entity.UnitPopulation(unitKind) <= summary.Cap
}

func (h *commandHandler) pendingReservations(team entity.Team, incoming ticker.Command) (world.Resources, int) {
	var reserved world.Resources
	reservedPop := 0

	for _, pending := range h.q.Snapshot() {
		if pending.Team != team || sameActor(pending, incoming) {
			continue
		}

		switch pending.Kind {
		case ticker.CmdBuild:
			if pending.BuildingKind == nil {
				continue
			}
			kind, ok := entity.ParseBuildingKind(*pending.BuildingKind)
			if !ok {
				continue
			}
			cost := entity.BuildingCost(kind)
			reserved.Food += cost.Food
			reserved.Gold += cost.Gold
			reserved.Stone += cost.Stone
			reserved.Wood += cost.Wood
		case ticker.CmdProduce:
			if pending.UnitKind == nil {
				continue
			}
			kind, ok := entity.ParseUnitKind(*pending.UnitKind)
			if !ok {
				continue
			}
			cost := entity.UnitCost(kind)
			reserved.Food += cost.Food
			reserved.Gold += cost.Gold
			reserved.Stone += cost.Stone
			reserved.Wood += cost.Wood
			reservedPop += entity.UnitPopulation(kind)
		}
	}

	return reserved, reservedPop
}

func sameActor(a, b ticker.Command) bool {
	switch {
	case a.BuildingID != nil && b.BuildingID != nil:
		return *a.BuildingID == *b.BuildingID
	case a.BuildingID == nil && b.BuildingID == nil:
		return a.UnitID == b.UnitID
	default:
		return false
	}
}
