package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
	"github.com/jason9075/agents_of_dynasties/internal/ticker"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

func TestCommandHandler_InvalidGatherReturnsCodeAndReason(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandHandler{w: w, q: q}

	infantry := w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 8, R: 7})
	body := map[string]any{
		"unit_id": infantry.ID(),
		"kind":    "GATHER",
	}

	rec := doCommandRequest(t, h, body, "1")
	assertErrorResponse(t, rec, http.StatusBadRequest, "missing_target_coord")
}

func TestMapHandler_ReflectsGatheredResourceRemaining(t *testing.T) {
	w := world.NewWorld(42)
	h := &mapHandler{w: w}
	villager := w.UnitsByTeam(entity.Team1)[0]
	pos := hex.Coord{Q: 3, R: 8}

	w.WriteFunc(func() {
		villager.SetPosition(pos)
		w.Tiles[pos] = terrain.Tile{Coord: pos, Terrain: terrain.GoldMine}
		w.ResourceRemaining[pos] = 15
	})

	if !w.GatherAtCurrentTile(villager.ID()) {
		t.Fatalf("expected gather to succeed")
	}

	req := httptest.NewRequest(http.MethodGet, "/map", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp mapResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal map response: %v", err)
	}

	for _, tile := range resp.Tiles {
		if tile.Coord.Q == pos.Q && tile.Coord.R == pos.R {
			if tile.Terrain != terrain.GoldMine.String() {
				t.Fatalf("terrain = %q, want %q", tile.Terrain, terrain.GoldMine.String())
			}
			if tile.Remaining != 3 {
				t.Fatalf("remaining = %d, want 3", tile.Remaining)
			}
			return
		}
	}

	t.Fatalf("expected tile %v in map response", pos)
}

func TestCommandHandler_MoveOutOfBoundsReturnsCodeAndReason(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandHandler{w: w, q: q}

	villager := w.UnitsByTeam(entity.Team1)[0]
	body := map[string]any{
		"unit_id": villager.ID(),
		"kind":    "MOVE_FAST",
		"target_coord": map[string]any{
			"q": 99,
			"r": 99,
		},
	}

	rec := doCommandRequest(t, h, body, "1")
	assertErrorResponse(t, rec, http.StatusBadRequest, "target_out_of_bounds")
}

func TestCommandHandler_AttackOutOfRangeIsAcceptedForPersistentChase(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandHandler{w: w, q: q}

	attacker := w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 2, R: 2})
	target := w.SpawnUnit(entity.Team2, entity.KindArcher, hex.Coord{Q: 10, R: 10})
	body := map[string]any{
		"unit_id":   attacker.ID(),
		"kind":      "ATTACK",
		"target_id": target.ID(),
	}

	rec := doCommandRequest(t, h, body, "1")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	var resp struct {
		CommandID uint64 `json:"command_id"`
		Tick      uint64 `json:"tick"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal accepted response: %v", err)
	}
	if resp.CommandID == 0 {
		t.Fatalf("expected non-zero command_id")
	}
	if resp.Tick != w.GetTick() {
		t.Fatalf("tick = %d, want %d", resp.Tick, w.GetTick())
	}
}

func TestCommandHandler_InvalidProducerReturnsCodeAndReason(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandHandler{w: w, q: q}

	var townCenterID entity.EntityID
	for _, b := range w.BuildingsByTeam(entity.Team1) {
		if b.Kind() == entity.KindTownCenter {
			townCenterID = b.ID()
		}
	}

	body := map[string]any{
		"building_id": townCenterID,
		"kind":        "PRODUCE",
		"unit_kind":   "infantry",
	}

	rec := doCommandRequest(t, h, body, "1")
	assertErrorResponse(t, rec, http.StatusBadRequest, "invalid_producer")
}

func TestCommandHandler_AccountsForPendingResourceReservations(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandHandler{w: w, q: q}

	builders := w.UnitsByTeam(entity.Team1)
	builder1 := builders[0]
	builder2 := builders[1]
	if builder1.ID() == builder2.ID() {
		t.Fatalf("expected distinct builders, got duplicate id %d", builder1.ID())
	}
	target1 := hex.Coord{Q: 6, R: 5}
	target2 := hex.Coord{Q: 6, R: 6}

	w.WriteFunc(func() {
		builder1.SetPosition(hex.Coord{Q: 5, R: 5})
		builder2.SetPosition(hex.Coord{Q: 5, R: 6})
		w.Tiles[target1] = terrain.Tile{Coord: target1, Terrain: terrain.Plain}
		w.Tiles[target2] = terrain.Tile{Coord: target2, Terrain: terrain.Plain}
	})

	first := map[string]any{
		"unit_id":       builder1.ID(),
		"kind":          "BUILD",
		"building_kind": "stable",
		"target_coord": map[string]any{
			"q": target1.Q,
			"r": target1.R,
		},
	}
	if rec := doCommandRequest(t, h, first, "1"); rec.Code != http.StatusAccepted {
		t.Fatalf("first build status = %d, want %d, body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	snap := q.Snapshot()
	if len(snap) != 1 || snap[0].UnitID != builder1.ID() {
		t.Fatalf("after first build expected one pending command for builder1=%d, got %+v", builder1.ID(), snap)
	}

	second := map[string]any{
		"unit_id":       builder2.ID(),
		"kind":          "BUILD",
		"building_kind": "stable",
		"target_coord": map[string]any{
			"q": target2.Q,
			"r": target2.R,
		},
	}
	rec := doCommandRequest(t, h, second, "1")
	if rec.Code == http.StatusAccepted {
		t.Fatalf("second build unexpectedly accepted, queue=%+v body=%s", q.Snapshot(), rec.Body.String())
	}
	assertErrorResponse(t, rec, http.StatusBadRequest, "insufficient_resources")
}

func TestCommandHandler_RejectsProduceWhenPopulationCapWouldBeExceeded(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandHandler{w: w, q: q}

	var tc *entity.Building
	for _, b := range w.BuildingsByTeam(entity.Team1) {
		if b.Kind() == entity.KindTownCenter {
			tc = b
			break
		}
	}
	if tc == nil {
		t.Fatalf("missing town center")
	}
	for i := len(w.UnitsByTeam(entity.Team1)); i < entity.PopulationCap; i++ {
		w.SpawnUnit(entity.Team1, entity.KindInfantry, hex.Coord{Q: 10 + (i % 5), R: 5 + (i / 5)})
	}

	body := map[string]any{
		"building_id": tc.ID(),
		"kind":        "PRODUCE",
		"unit_kind":   "villager",
	}

	rec := doCommandRequest(t, h, body, "1")
	assertErrorResponse(t, rec, http.StatusBadRequest, "population_cap_reached")
}

func TestCommandsHandler_ReturnsPendingCommandsForTeamOnly(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandsHandler{w: w, q: q}

	team1Unit := w.UnitsByTeam(entity.Team1)[0]
	team2Unit := w.UnitsByTeam(entity.Team2)[0]
	target := hex.Coord{Q: 7, R: 7}

	q.Submit(ticker.Command{
		Team:          entity.Team1,
		UnitID:        team1Unit.ID(),
		Kind:          ticker.CmdMoveFast,
		TargetCoord:   &target,
		SubmittedTick: w.GetTick(),
	})
	q.Submit(ticker.Command{
		Team:          entity.Team2,
		UnitID:        team2Unit.ID(),
		Kind:          ticker.CmdGather,
		SubmittedTick: w.GetTick(),
	})

	req := httptest.NewRequest(http.MethodGet, "/commands", nil)
	req.Header.Set("X-Team-ID", "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Tick     uint64 `json:"tick"`
		Commands []struct {
			CommandID     uint64           `json:"command_id"`
			SubmittedTick uint64           `json:"submitted_tick"`
			Team          entity.Team      `json:"team"`
			UnitID        *entity.EntityID `json:"unit_id,omitempty"`
			Kind          string           `json:"kind"`
			TargetCoord   *struct {
				Q int `json:"q"`
				R int `json:"r"`
			} `json:"target_coord,omitempty"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal commands response: %v", err)
	}
	if resp.Tick != w.GetTick() {
		t.Fatalf("tick = %d, want %d", resp.Tick, w.GetTick())
	}
	if len(resp.Commands) != 1 {
		t.Fatalf("commands len = %d, want 1, body=%s", len(resp.Commands), rec.Body.String())
	}
	if resp.Commands[0].CommandID == 0 {
		t.Fatalf("expected command_id in commands response")
	}
	if resp.Commands[0].SubmittedTick != w.GetTick() {
		t.Fatalf("submitted_tick = %d, want %d", resp.Commands[0].SubmittedTick, w.GetTick())
	}
	if resp.Commands[0].Team != entity.Team1 || resp.Commands[0].UnitID == nil || *resp.Commands[0].UnitID != team1Unit.ID() || resp.Commands[0].Kind != string(ticker.CmdMoveFast) {
		t.Fatalf("unexpected command: %+v", resp.Commands[0])
	}
	if resp.Commands[0].TargetCoord == nil || resp.Commands[0].TargetCoord.Q != target.Q || resp.Commands[0].TargetCoord.R != target.R {
		t.Fatalf("unexpected target coord: %+v", resp.Commands[0].TargetCoord)
	}
}

func TestCommandsHandler_ReflectsLastCommandWins(t *testing.T) {
	w := world.NewWorld(42)
	q := ticker.NewQueue()
	h := &commandsHandler{w: w, q: q}

	unit := w.UnitsByTeam(entity.Team1)[0]
	first := hex.Coord{Q: 6, R: 6}
	second := hex.Coord{Q: 8, R: 8}

	q.Submit(ticker.Command{
		Team:          entity.Team1,
		UnitID:        unit.ID(),
		Kind:          ticker.CmdMoveFast,
		TargetCoord:   &first,
		SubmittedTick: w.GetTick(),
	})
	q.Submit(ticker.Command{
		Team:          entity.Team1,
		UnitID:        unit.ID(),
		Kind:          ticker.CmdMoveGuard,
		TargetCoord:   &second,
		SubmittedTick: w.GetTick(),
	})

	req := httptest.NewRequest(http.MethodGet, "/commands", nil)
	req.Header.Set("X-Team-ID", "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Commands []struct {
			Kind        string `json:"kind"`
			TargetCoord *struct {
				Q int `json:"q"`
				R int `json:"r"`
			} `json:"target_coord,omitempty"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal commands response: %v", err)
	}
	if len(resp.Commands) != 1 {
		t.Fatalf("commands len = %d, want 1, body=%s", len(resp.Commands), rec.Body.String())
	}
	if resp.Commands[0].Kind != string(ticker.CmdMoveGuard) {
		t.Fatalf("kind = %q, want %q", resp.Commands[0].Kind, ticker.CmdMoveGuard)
	}
	if resp.Commands[0].TargetCoord == nil || resp.Commands[0].TargetCoord.Q != second.Q || resp.Commands[0].TargetCoord.R != second.R {
		t.Fatalf("unexpected target coord: %+v", resp.Commands[0].TargetCoord)
	}
}

func TestStateHandler_ExposesPersistentUnitStatus(t *testing.T) {
	w := world.NewWorld(42)
	h := &stateHandler{w: w}

	unit := w.UnitsByTeam(entity.Team1)[0]
	target := hex.Coord{Q: 7, R: 4}
	w.WriteFunc(func() {
		unit.SetMoveStatus(entity.StatusMovingFast, target)
		unit.SetStatusPhase(entity.PhaseMovingToTarget)
	})

	req := httptest.NewRequest(http.MethodGet, "/state", nil)
	req.Header.Set("X-Team-ID", "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Units []struct {
			ID                entity.EntityID `json:"id"`
			Status            string          `json:"status"`
			StatusPhase       string          `json:"status_phase,omitempty"`
			StatusTargetCoord *struct {
				Q int `json:"q"`
				R int `json:"r"`
			} `json:"status_target_coord,omitempty"`
		} `json:"units"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal state response: %v", err)
	}
	for _, got := range resp.Units {
		if got.ID != unit.ID() {
			continue
		}
		if got.Status != string(entity.StatusMovingFast) || got.StatusPhase != string(entity.PhaseMovingToTarget) {
			t.Fatalf("unexpected status payload: %+v", got)
		}
		if got.StatusTargetCoord == nil || got.StatusTargetCoord.Q != target.Q || got.StatusTargetCoord.R != target.R {
			t.Fatalf("unexpected target coord: %+v", got.StatusTargetCoord)
		}
		return
	}
	t.Fatalf("expected unit %d in state response", unit.ID())
}

func TestStateHandler_ExposesLastTickFailedCommands(t *testing.T) {
	w := world.NewWorld(42)
	h := &stateHandler{w: w}

	unit := w.UnitsByTeam(entity.Team1)[0]
	target := hex.Coord{Q: 7, R: 4}
	commandID := uint64(42)
	w.SetLastTickCommandFailures(entity.Team1, []world.CommandFailure{{
		CommandID:     commandID,
		Team:          entity.Team1,
		UnitID:        ptrEntityID(unit.ID()),
		Kind:          string(ticker.CmdMoveFast),
		TargetCoord:   &target,
		SubmittedTick: 3,
		ResolvedTick:  4,
		Code:          "target_building_occupied",
		Reason:        "target hex is occupied by a building at resolution",
	}})

	req := httptest.NewRequest(http.MethodGet, "/state", nil)
	req.Header.Set("X-Team-ID", "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		LastTickFailedCommands []struct {
			CommandID     uint64           `json:"command_id"`
			UnitID        *entity.EntityID `json:"unit_id,omitempty"`
			SubmittedTick uint64           `json:"submitted_tick"`
			ResolvedTick  uint64           `json:"resolved_tick"`
			Code          string           `json:"code"`
			Reason        string           `json:"reason"`
			TargetCoord   *struct {
				Q int `json:"q"`
				R int `json:"r"`
			} `json:"target_coord,omitempty"`
		} `json:"last_tick_failed_commands"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal state response: %v", err)
	}
	if len(resp.LastTickFailedCommands) != 1 {
		t.Fatalf("last_tick_failed_commands len = %d, want 1, body=%s", len(resp.LastTickFailedCommands), rec.Body.String())
	}
	got := resp.LastTickFailedCommands[0]
	if got.CommandID != commandID || got.UnitID == nil || *got.UnitID != unit.ID() {
		t.Fatalf("unexpected failed command payload: %+v", got)
	}
	if got.SubmittedTick != 3 || got.ResolvedTick != 4 || got.Code != "target_building_occupied" || got.Reason == "" {
		t.Fatalf("unexpected failure metadata: %+v", got)
	}
	if got.TargetCoord == nil || got.TargetCoord.Q != target.Q || got.TargetCoord.R != target.R {
		t.Fatalf("unexpected target coord: %+v", got.TargetCoord)
	}
}

func TestFullStateHandler_ExposesLastTickFailedCommandsPerTeam(t *testing.T) {
	w := world.NewWorld(42)
	h := &fullStateHandler{w: w}

	team1Unit := w.UnitsByTeam(entity.Team1)[0]
	team2Unit := w.UnitsByTeam(entity.Team2)[0]
	w.SetLastTickCommandFailures(entity.Team1, []world.CommandFailure{{
		CommandID:     11,
		Team:          entity.Team1,
		UnitID:        ptrEntityID(team1Unit.ID()),
		Kind:          string(ticker.CmdMoveFast),
		SubmittedTick: 1,
		ResolvedTick:  2,
		Code:          "target_building_occupied",
		Reason:        "target hex is occupied by a building at resolution",
	}})
	w.SetLastTickCommandFailures(entity.Team2, []world.CommandFailure{{
		CommandID:     22,
		Team:          entity.Team2,
		UnitID:        ptrEntityID(team2Unit.ID()),
		Kind:          string(ticker.CmdAttack),
		SubmittedTick: 1,
		ResolvedTick:  2,
		Code:          "target_not_found",
		Reason:        "attack target no longer exists at resolution",
	}})

	req := httptest.NewRequest(http.MethodGet, "/state/full", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Team1 struct {
			LastTickFailedCommands []struct {
				CommandID uint64 `json:"command_id"`
				Code      string `json:"code"`
			} `json:"last_tick_failed_commands"`
		} `json:"team1"`
		Team2 struct {
			LastTickFailedCommands []struct {
				CommandID uint64 `json:"command_id"`
				Code      string `json:"code"`
			} `json:"last_tick_failed_commands"`
		} `json:"team2"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal full state response: %v", err)
	}
	if len(resp.Team1.LastTickFailedCommands) != 1 || resp.Team1.LastTickFailedCommands[0].CommandID != 11 || resp.Team1.LastTickFailedCommands[0].Code != "target_building_occupied" {
		t.Fatalf("unexpected team1 failures: %+v", resp.Team1.LastTickFailedCommands)
	}
	if len(resp.Team2.LastTickFailedCommands) != 1 || resp.Team2.LastTickFailedCommands[0].CommandID != 22 || resp.Team2.LastTickFailedCommands[0].Code != "target_not_found" {
		t.Fatalf("unexpected team2 failures: %+v", resp.Team2.LastTickFailedCommands)
	}
}

func TestFullStateHandler_ExposesLastTickContestedHexes(t *testing.T) {
	w := world.NewWorld(42)
	h := &fullStateHandler{w: w}

	w.SetLastTickContestedHexes([]world.ContestedHex{{
		Coord:        hex.Coord{Q: 8, R: 7},
		Team1UnitIDs: []entity.EntityID{101, 102},
		Team2UnitIDs: []entity.EntityID{201},
	}})

	req := httptest.NewRequest(http.MethodGet, "/state/full", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		LastTickContestedHexes []struct {
			Coord struct {
				Q int `json:"q"`
				R int `json:"r"`
			} `json:"coord"`
			Team1UnitIDs []entity.EntityID `json:"team1_unit_ids"`
			Team2UnitIDs []entity.EntityID `json:"team2_unit_ids"`
		} `json:"last_tick_contested_hexes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal full state response: %v", err)
	}
	if len(resp.LastTickContestedHexes) != 1 {
		t.Fatalf("expected one contested hex, got %+v", resp.LastTickContestedHexes)
	}
	got := resp.LastTickContestedHexes[0]
	if got.Coord.Q != 8 || got.Coord.R != 7 {
		t.Fatalf("unexpected contested coord: %+v", got.Coord)
	}
	if len(got.Team1UnitIDs) != 2 || len(got.Team2UnitIDs) != 1 {
		t.Fatalf("unexpected contested participants: %+v", got)
	}
}

func TestFullStateHandler_ExposesTeamAppearance(t *testing.T) {
	w := world.NewWorld(42)
	w.SetTeamAppearance(entity.Team1, world.TeamAppearance{Faction: "linux", Variant: "red"})
	w.SetTeamAppearance(entity.Team2, world.TeamAppearance{Faction: "microsoft", Variant: "blue"})
	h := &fullStateHandler{w: w}

	req := httptest.NewRequest(http.MethodGet, "/state/full", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Team1 struct {
			Appearance struct {
				Faction string `json:"faction"`
				Variant string `json:"variant"`
			} `json:"appearance"`
		} `json:"team1"`
		Team2 struct {
			Appearance struct {
				Faction string `json:"faction"`
				Variant string `json:"variant"`
			} `json:"appearance"`
		} `json:"team2"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal full state response: %v", err)
	}
	if resp.Team1.Appearance.Faction != "linux" || resp.Team1.Appearance.Variant != "red" {
		t.Fatalf("unexpected team1 appearance: %+v", resp.Team1.Appearance)
	}
	if resp.Team2.Appearance.Faction != "microsoft" || resp.Team2.Appearance.Variant != "blue" {
		t.Fatalf("unexpected team2 appearance: %+v", resp.Team2.Appearance)
	}
}

type commandErrorEnvelope struct {
	Error struct {
		Code   string `json:"code"`
		Reason string `json:"reason"`
	} `json:"error"`
}

func doCommandRequest(t *testing.T, h http.Handler, body map[string]any, team string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/command", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Team-ID", team)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	var resp commandErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if resp.Error.Code != wantCode {
		t.Fatalf("error.code = %q, want %q", resp.Error.Code, wantCode)
	}
	if resp.Error.Reason == "" {
		t.Fatalf("error.reason should not be empty")
	}
}

func ptrEntityID(id entity.EntityID) *entity.EntityID {
	return &id
}
