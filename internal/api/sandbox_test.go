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

	preset := resp.Presets[0]
	if preset.ID != "villager_move_then_build" {
		t.Fatalf("preset id = %q, want %q", preset.ID, "villager_move_then_build")
	}
	if preset.MaxTick != 6 {
		t.Fatalf("max_tick = %d, want 6", preset.MaxTick)
	}
	if len(preset.Actors) != 1 || preset.Actors[0].ID != "villager_1" {
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
	assertUnitPos(t, resp.Snapshots[1], 1001, 3, 2)
	assertUnitPos(t, resp.Snapshots[2], 1001, 4, 2)
	assertUnitPos(t, resp.Snapshots[3], 1001, 5, 2)

	tick4Barracks := findBuilding(resp.Snapshots[4].Team1.Buildings, "barracks", 6, 2)
	if tick4Barracks == nil {
		t.Fatalf("expected barracks on tick 4, buildings=%+v", resp.Snapshots[4].Team1.Buildings)
	}
	if tick4Barracks.Complete {
		t.Fatalf("expected barracks to still be under construction on tick 4")
	}
	if tick4Barracks.BuildProgress != 1 || tick4Barracks.BuildTicksTotal != 2 {
		t.Fatalf("unexpected barracks construction state on tick 4: %+v", *tick4Barracks)
	}

	tick5Barracks := findBuilding(resp.Snapshots[5].Team1.Buildings, "barracks", 6, 2)
	if tick5Barracks == nil || !tick5Barracks.Complete {
		t.Fatalf("expected barracks to complete on tick 5, buildings=%+v", resp.Snapshots[5].Team1.Buildings)
	}
	if got := resp.Snapshots[4].Team1.Resources; got.Wood != 80 || got.Stone != 40 || got.Food != 200 || got.Gold != 100 {
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
