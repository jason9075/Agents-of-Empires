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
	ID                 entity.EntityID  `json:"id"`
	Kind               string           `json:"kind"`
	Team               entity.Team      `json:"team"`
	Position           coordView        `json:"position"`
	HP                 int              `json:"hp"`
	MaxHP              int              `json:"max_hp"`
	CarryResource      string           `json:"carry_resource,omitempty"`
	CarryAmount        int              `json:"carry_amount"`
	Status             string           `json:"status"`
	StatusPhase        string           `json:"status_phase,omitempty"`
	StatusTargetCoord  *coordView       `json:"status_target_coord,omitempty"`
	StatusTargetID     *entity.EntityID `json:"status_target_id,omitempty"`
	StatusBuildingKind string           `json:"status_building_kind,omitempty"`
	AttackTargetID     *entity.EntityID `json:"attack_target_id,omitempty"`
	Friendly           bool             `json:"friendly"`
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

type commandView struct {
	CommandID     uint64             `json:"command_id"`
	SubmittedTick uint64             `json:"submitted_tick"`
	Team          entity.Team        `json:"team"`
	UnitID        *entity.EntityID   `json:"unit_id,omitempty"`
	BuildingID    *entity.EntityID   `json:"building_id,omitempty"`
	Kind          ticker.CommandKind `json:"kind"`
	TargetCoord   *coordView         `json:"target_coord,omitempty"`
	TargetID      *entity.EntityID   `json:"target_id,omitempty"`
	BuildingKind  *string            `json:"building_kind,omitempty"`
	UnitKind      *string            `json:"unit_kind,omitempty"`
}

type failedCommandView struct {
	CommandID     uint64           `json:"command_id"`
	UnitID        *entity.EntityID `json:"unit_id,omitempty"`
	BuildingID    *entity.EntityID `json:"building_id,omitempty"`
	Kind          string           `json:"kind"`
	TargetCoord   *coordView       `json:"target_coord,omitempty"`
	TargetID      *entity.EntityID `json:"target_id,omitempty"`
	BuildingKind  *string          `json:"building_kind,omitempty"`
	UnitKind      *string          `json:"unit_kind,omitempty"`
	SubmittedTick uint64           `json:"submitted_tick"`
	ResolvedTick  uint64           `json:"resolved_tick"`
	Code          string           `json:"code"`
	Reason        string           `json:"reason"`
}

type populationView struct {
	Used     int `json:"used"`
	Reserved int `json:"reserved"`
	Cap      int `json:"cap"`
}

type stateResponse struct {
	Tick                   uint64              `json:"tick"`
	Resources              world.Resources     `json:"resources"`
	Population             populationView      `json:"population"`
	LastTickFailedCommands []failedCommandView `json:"last_tick_failed_commands"`
	Units                  []unitView          `json:"units"`
	Buildings              []buildingView      `json:"buildings"`
}

type commandsResponse struct {
	Tick     uint64        `json:"tick"`
	Commands []commandView `json:"commands"`
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
		units = append(units, toUnitView(u, true))
	}
	for _, u := range enemyUnits {
		units = append(units, toUnitView(u, false))
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
		Tick:                   h.w.GetTick(),
		Resources:              h.w.GetResources(team),
		Population:             toPopulationView(h.w.GetPopulationSummary(team)),
		LastTickFailedCommands: toFailedCommandViews(h.w.GetLastTickCommandFailures(team)),
		Units:                  units,
		Buildings:              buildings,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- Full state handler (god-mode, no LOS masking) ---

type fullStateTeam struct {
	Resources              world.Resources     `json:"resources"`
	Population             populationView      `json:"population"`
	LastTickFailedCommands []failedCommandView `json:"last_tick_failed_commands"`
	Units                  []unitView          `json:"units"`
	Buildings              []buildingView      `json:"buildings"`
}

type commandAcceptedResponse struct {
	CommandID uint64 `json:"command_id"`
	Tick      uint64 `json:"tick"`
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
			units = append(units, toUnitView(u, true))
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
			Resources:              h.w.GetResources(team),
			Population:             toPopulationView(h.w.GetPopulationSummary(team)),
			LastTickFailedCommands: toFailedCommandViews(h.w.GetLastTickCommandFailures(team)),
			Units:                  units,
			Buildings:              buildings,
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

// --- Pending commands handler ---

type commandsHandler struct {
	w *world.World
	q *ticker.Queue
}

func (h *commandsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	team, err := teamFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_header", err.Error())
		return
	}

	queued := h.q.Snapshot()
	commands := make([]commandView, 0, len(queued))
	for _, cmd := range queued {
		if cmd.Team != team {
			continue
		}
		commands = append(commands, toCommandView(cmd))
	}

	resp := commandsResponse{
		Tick:     h.w.GetTick(),
		Commands: commands,
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

	cmd.SubmittedTick = h.w.GetTick()
	accepted := h.q.Submit(cmd)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(commandAcceptedResponse{
		CommandID: accepted.CommandID,
		Tick:      accepted.SubmittedTick,
	})
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
	case ticker.CmdStop:
		return 0, "", ""
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
	if cmd.TargetCoord == nil {
		return http.StatusBadRequest, "missing_target_coord", "target_coord is required for GATHER"
	}
	unit := h.w.GetUnit(cmd.UnitID)
	if unit.Kind() != entity.KindVillager {
		return http.StatusBadRequest, "unit_cannot_gather", "only villagers can gather resources"
	}
	if !hex.InBounds(*cmd.TargetCoord) {
		return http.StatusBadRequest, "target_out_of_bounds", "target_coord is outside the map"
	}
	tile, ok := h.w.Tile(*cmd.TargetCoord)
	if !ok || tile.Terrain.ResourceYield() == terrain.ResourceNone {
		return http.StatusBadRequest, "no_gatherable_resource", "target_coord is not a gatherable resource tile"
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
	switch h.w.BuildTargetStatus(cmd.Team, kind, *cmd.TargetCoord) {
	case world.BuildTargetInvalid:
		return http.StatusBadRequest, "invalid_build_tile", "build target is not valid for this building"
	case world.BuildTargetCreate:
		fallthrough
	case world.BuildTargetBlocked:
		cost := entity.BuildingCost(kind)
		if !h.canAffordWithPending(cmd.Team, cost, cmd) {
			return http.StatusBadRequest, "insufficient_resources", "team cannot afford this building"
		}
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
		return 0, "", ""
	}
	if targetBuilding := h.w.GetBuilding(*cmd.TargetID); targetBuilding != nil {
		if targetBuilding.Team() == attacker.Team() {
			return http.StatusBadRequest, "friendly_fire_forbidden", "cannot attack a friendly building"
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

func toCommandView(cmd ticker.Command) commandView {
	var targetCoord *coordView
	if cmd.TargetCoord != nil {
		targetCoord = &coordView{Q: cmd.TargetCoord.Q, R: cmd.TargetCoord.R}
	}
	var unitID *entity.EntityID
	if cmd.BuildingID == nil {
		id := cmd.UnitID
		unitID = &id
	}
	return commandView{
		CommandID:     cmd.CommandID,
		SubmittedTick: cmd.SubmittedTick,
		Team:          cmd.Team,
		UnitID:        unitID,
		BuildingID:    cmd.BuildingID,
		Kind:          cmd.Kind,
		TargetCoord:   targetCoord,
		TargetID:      cmd.TargetID,
		BuildingKind:  cmd.BuildingKind,
		UnitKind:      cmd.UnitKind,
	}
}

func toUnitView(u *entity.Unit, friendly bool) unitView {
	pos := u.Position()
	var attackTargetID *entity.EntityID
	if id, ok := u.AttackTargetID(); ok {
		attackTargetID = &id
	}
	var statusTargetCoord *coordView
	if coord, ok := u.StatusTargetCoord(); ok {
		statusTargetCoord = &coordView{Q: coord.Q, R: coord.R}
	}
	var statusTargetID *entity.EntityID
	if id, ok := u.StatusTargetID(); ok {
		statusTargetID = &id
	}
	statusBuildingKind := ""
	if kind, ok := u.StatusBuildingKind(); ok {
		statusBuildingKind = kind.String()
	}
	return unitView{
		ID:                 u.ID(),
		Kind:               u.Kind().String(),
		Team:               u.Team(),
		Position:           coordView{Q: pos.Q, R: pos.R},
		HP:                 u.HP(),
		MaxHP:              u.MaxHP(),
		CarryResource:      string(u.CarryType()),
		CarryAmount:        u.CarryAmount(),
		Status:             string(u.Status()),
		StatusPhase:        string(u.StatusPhase()),
		StatusTargetCoord:  statusTargetCoord,
		StatusTargetID:     statusTargetID,
		StatusBuildingKind: statusBuildingKind,
		AttackTargetID:     attackTargetID,
		Friendly:           friendly,
	}
}

func toFailedCommandViews(failures []world.CommandFailure) []failedCommandView {
	out := make([]failedCommandView, 0, len(failures))
	for _, failure := range failures {
		var targetCoord *coordView
		if failure.TargetCoord != nil {
			targetCoord = &coordView{Q: failure.TargetCoord.Q, R: failure.TargetCoord.R}
		}
		out = append(out, failedCommandView{
			CommandID:     failure.CommandID,
			UnitID:        failure.UnitID,
			BuildingID:    failure.BuildingID,
			Kind:          failure.Kind,
			TargetCoord:   targetCoord,
			TargetID:      failure.TargetID,
			BuildingKind:  failure.BuildingKind,
			UnitKind:      failure.UnitKind,
			SubmittedTick: failure.SubmittedTick,
			ResolvedTick:  failure.ResolvedTick,
			Code:          failure.Code,
			Reason:        failure.Reason,
		})
	}
	return out
}
