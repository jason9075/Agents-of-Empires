package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/ticker"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

func TestSandboxPresetsHandler_ReturnsPresetSummary(t *testing.T) {
	h := &sandboxPresetsHandler{}

	req := httptest.NewRequest(http.MethodGet, "/sandbox/presets", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp sandboxPresetsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal presets response: %v", err)
	}
	if len(resp.Presets) == 0 {
		t.Fatalf("expected at least one preset")
	}

	preset := findPresetSummary(resp.Presets, "gather_four_corners")
	if preset == nil {
		t.Fatalf("expected gather_four_corners preset, got %+v", resp.Presets)
	}
	if preset.MaxTick != 18 {
		t.Fatalf("max_tick = %d, want 18", preset.MaxTick)
	}
	if len(preset.Actors) != 4 {
		t.Fatalf("unexpected actors: %+v", preset.Actors)
	}
	if len(preset.DefaultTimeline) != 4 {
		t.Fatalf("default timeline len = %d, want 4", len(preset.DefaultTimeline))
	}
}

func TestSandboxSimulateHandler_DefaultPresetReplaysLiveServerLogic(t *testing.T) {
	h := &sandboxSimulateHandler{}
	reqBody := sandboxSimulationRequest{PresetID: "villager_move_then_build"}

	rec := doSandboxRequest(t, http.MethodPost, "/sandbox/simulate", reqBody, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp sandboxSimulationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal sandbox simulate response: %v", err)
	}

	if len(resp.ValidationIssues) != 0 {
		t.Fatalf("expected no validation issues, got %+v", resp.ValidationIssues)
	}
	if resp.Map.Width != 8 || resp.Map.Height != 6 {
		t.Fatalf("unexpected sandbox map: %+v", resp.Map)
	}
	if len(resp.Snapshots) != resp.Preset.MaxTick+1 {
		t.Fatalf("snapshots len = %d, want %d", len(resp.Snapshots), resp.Preset.MaxTick+1)
	}

	assertUnitPos(t, resp.Snapshots[0], 1001, 2, 2)
	assertUnitPos(t, resp.Snapshots[1], 1001, 4, 2)
	assertUnitPos(t, resp.Snapshots[2], 1001, 5, 2)
	assertUnitPos(t, resp.Snapshots[3], 1001, 5, 2)

	tick2Barracks := findBuilding(resp.Snapshots[2].Team1.Buildings, "barracks", 6, 2)
	if tick2Barracks == nil {
		t.Fatalf("expected barracks on tick 2, buildings=%+v", resp.Snapshots[2].Team1.Buildings)
	}
	if tick2Barracks.Complete {
		t.Fatalf("expected barracks to still be under construction on tick 2")
	}
	if tick2Barracks.BuildProgress != 1 || tick2Barracks.BuildTicksTotal != 2 {
		t.Fatalf("unexpected barracks construction state on tick 2: %+v", *tick2Barracks)
	}

	tick3Barracks := findBuilding(resp.Snapshots[3].Team1.Buildings, "barracks", 6, 2)
	if tick3Barracks == nil || !tick3Barracks.Complete {
		t.Fatalf("expected barracks to complete on tick 3, buildings=%+v", resp.Snapshots[3].Team1.Buildings)
	}
	if got := resp.Snapshots[2].Team1.Resources; got.Wood != 80 || got.Stone != 40 || got.Food != 200 || got.Gold != 100 {
		t.Fatalf("unexpected resources after build payment: %+v", got)
	}
}

func TestSandboxSimulateHandler_ReturnsValidationIssuesWithoutTouchingLiveWorld(t *testing.T) {
	liveWorld := world.NewWorld(42)
	q := ticker.NewQueue()
	server := NewServer(liveWorld, q, "./web")

	beforeTick := liveWorld.GetTick()
	beforeRes := liveWorld.GetResources(entity.Team1)
	beforeUnits := liveWorld.UnitsByTeam(entity.Team1)
	if len(beforeUnits) == 0 {
		t.Fatalf("expected live world to have starting units")
	}
	beforePositions := make(map[entity.EntityID]coordView, len(beforeUnits))
	for _, unit := range beforeUnits {
		pos := unit.Position()
		beforePositions[unit.ID()] = coordView{Q: pos.Q, R: pos.R}
	}

	reqBody := sandboxSimulationRequest{
		PresetID: "villager_move_then_build",
		Timeline: []sandboxTimelineRow{
			{
				RowID:   "bad_row",
				Tick:    1,
				ActorID: "missing_actor",
				Kind:    ticker.CmdMoveGuard,
				TargetCoord: &coordView{
					Q: 5,
					R: 2,
				},
			},
		},
	}

	rec := doSandboxRequest(t, http.MethodPost, "/sandbox/simulate", reqBody, server)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp sandboxSimulationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal sandbox simulate response: %v", err)
	}
	if len(resp.ValidationIssues) != 1 {
		t.Fatalf("validation issues len = %d, want 1", len(resp.ValidationIssues))
	}
	if resp.ValidationIssues[0].RowID != "bad_row" || resp.ValidationIssues[0].Code != "unknown_actor" {
		t.Fatalf("unexpected validation issue: %+v", resp.ValidationIssues[0])
	}

	if got := liveWorld.GetTick(); got != beforeTick {
		t.Fatalf("live world tick changed: got %d want %d", got, beforeTick)
	}
	if got := liveWorld.GetResources(entity.Team1); got != beforeRes {
		t.Fatalf("live world resources changed: got %+v want %+v", got, beforeRes)
	}
	afterUnits := liveWorld.UnitsByTeam(entity.Team1)
	if len(afterUnits) != len(beforeUnits) {
		t.Fatalf("live world unit count changed: got %d want %d", len(afterUnits), len(beforeUnits))
	}
	for _, unit := range afterUnits {
		wantPos, ok := beforePositions[unit.ID()]
		if !ok {
			t.Fatalf("live world has unexpected unit %d after sandbox request", unit.ID())
		}
		got := unit.Position()
		if got.Q != wantPos.Q || got.R != wantPos.R {
			t.Fatalf("live world unit %d position changed: got (%d,%d) want (%d,%d)", unit.ID(), got.Q, got.R, wantPos.Q, wantPos.R)
		}
	}
}

func TestSandboxSimulateHandler_AllowsExtendedSimulationBeyondPresetMaxTick(t *testing.T) {
	h := &sandboxSimulateHandler{}
	reqBody := sandboxSimulationRequest{
		PresetID: "villager_move_then_build",
		MaxTick:  10,
	}

	rec := doSandboxRequest(t, http.MethodPost, "/sandbox/simulate", reqBody, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp sandboxSimulationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal sandbox simulate response: %v", err)
	}

	if resp.Preset.MaxTick != 10 {
		t.Fatalf("response max_tick = %d, want 10", resp.Preset.MaxTick)
	}
	if len(resp.Snapshots) != 11 {
		t.Fatalf("snapshots len = %d, want 11", len(resp.Snapshots))
	}

	assertUnitPos(t, resp.Snapshots[10], 1001, 5, 2)

	tick10Barracks := findBuilding(resp.Snapshots[10].Team1.Buildings, "barracks", 6, 2)
	if tick10Barracks == nil || !tick10Barracks.Complete {
		t.Fatalf("expected completed barracks on tick 10, buildings=%+v", resp.Snapshots[10].Team1.Buildings)
	}
}

func TestSandboxSimulateHandler_ExposesUnitStatusInSnapshots(t *testing.T) {
	h := &sandboxSimulateHandler{}
	reqBody := sandboxSimulationRequest{PresetID: "villager_move_then_build"}

	rec := doSandboxRequest(t, http.MethodPost, "/sandbox/simulate", reqBody, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp sandboxSimulationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal sandbox simulate response: %v", err)
	}

	unitAtTick2 := findUnit(resp.Snapshots[2].Team1.Units, 1001)
	if unitAtTick2 == nil {
		t.Fatalf("expected unit on tick 2, units=%+v", resp.Snapshots[2].Team1.Units)
	}
	if unitAtTick2.Status != string(entity.StatusBuilding) {
		t.Fatalf("status = %q, want %q", unitAtTick2.Status, entity.StatusBuilding)
	}

	unitAtTick3 := findUnit(resp.Snapshots[3].Team1.Units, 1001)
	if unitAtTick3 == nil {
		t.Fatalf("expected unit on tick 3, units=%+v", resp.Snapshots[3].Team1.Units)
	}
	if unitAtTick3.Status != string(entity.StatusIdle) {
		t.Fatalf("status = %q, want %q", unitAtTick3.Status, entity.StatusIdle)
	}
}

func TestSandboxSimulateHandler_GatherFourCornersStartsPersistentGathering(t *testing.T) {
	h := &sandboxSimulateHandler{}
	reqBody := sandboxSimulationRequest{PresetID: "gather_four_corners"}

	rec := doSandboxRequest(t, http.MethodPost, "/sandbox/simulate", reqBody, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp sandboxSimulationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal sandbox simulate response: %v", err)
	}

	if resp.Map.Width != 9 || resp.Map.Height != 9 {
		t.Fatalf("unexpected sandbox map: %+v", resp.Map)
	}

	tick1 := resp.Snapshots[1]
	starts := map[entity.EntityID]coordView{
		1101: {Q: 3, R: 4},
		1102: {Q: 4, R: 3},
		1103: {Q: 4, R: 5},
		1104: {Q: 5, R: 4},
	}
	for _, id := range []entity.EntityID{1101, 1102, 1103, 1104} {
		unit := findUnit(tick1.Team1.Units, id)
		if unit == nil {
			t.Fatalf("missing unit %d on tick 1", id)
		}
		if unit.Status != string(entity.StatusGathering) {
			t.Fatalf("unit %d status = %q, want %q", id, unit.Status, entity.StatusGathering)
		}
		start := starts[id]
		if unit.Position == start {
			t.Fatalf("unit %d did not leave its start tile: %+v", id, unit.Position)
		}
	}
}

func TestSandboxSimulateHandler_GatherFourCornersReducesResourceRemaining(t *testing.T) {
	h := &sandboxSimulateHandler{}
	reqBody := sandboxSimulationRequest{PresetID: "gather_four_corners"}

	rec := doSandboxRequest(t, http.MethodPost, "/sandbox/simulate", reqBody, h)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp sandboxSimulationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal sandbox simulate response: %v", err)
	}

	startTile := findTile(resp.Snapshots[0].Tiles, 0, 0)
	if startTile == nil || startTile.Remaining <= 0 {
		t.Fatalf("expected deer tile remaining on tick 0, tiles=%+v", resp.Snapshots[0].Tiles)
	}

	reduced := false
	for _, snapshot := range resp.Snapshots[1:] {
		tile := findTile(snapshot.Tiles, 0, 0)
		if tile != nil && tile.Remaining < startTile.Remaining {
			reduced = true
			break
		}
	}
	if !reduced {
		t.Fatalf("expected deer tile remaining to decrease across snapshots")
	}
}

func doSandboxRequest(t *testing.T, method, path string, body any, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal sandbox body: %v", err)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func assertUnitPos(t *testing.T, snapshot sandboxSnapshot, id entity.EntityID, wantQ, wantR int) {
	t.Helper()

	for _, unit := range snapshot.Team1.Units {
		if unit.ID != id {
			continue
		}
		if unit.Position.Q != wantQ || unit.Position.R != wantR {
			t.Fatalf("unit %d position = (%d,%d), want (%d,%d)", id, unit.Position.Q, unit.Position.R, wantQ, wantR)
		}
		return
	}

	t.Fatalf("unit %d not found in snapshot %+v", id, snapshot.Team1.Units)
}

func findBuilding(buildings []buildingView, kind string, q, r int) *buildingView {
	for _, building := range buildings {
		if building.Kind == kind && building.Position.Q == q && building.Position.R == r {
			copyBuilding := building
			return &copyBuilding
		}
	}
	return nil
}

func findUnit(units []unitView, id entity.EntityID) *unitView {
	for _, unit := range units {
		if unit.ID == id {
			copyUnit := unit
			return &copyUnit
		}
	}
	return nil
}

func findPresetSummary(presets []sandboxPresetSummary, id string) *sandboxPresetSummary {
	for _, preset := range presets {
		if preset.ID == id {
			copyPreset := preset
			return &copyPreset
		}
	}
	return nil
}

func findTile(tiles []tileView, q, r int) *tileView {
	for _, tile := range tiles {
		if tile.Coord.Q == q && tile.Coord.R == r {
			copyTile := tile
			return &copyTile
		}
	}
	return nil
}
