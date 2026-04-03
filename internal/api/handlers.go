package api

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/jason9075/agents_of_dynasties/internal/entity"
	"github.com/jason9075/agents_of_dynasties/internal/hex"
	"github.com/jason9075/agents_of_dynasties/internal/terrain"
	"github.com/jason9075/agents_of_dynasties/internal/ticker"
	"github.com/jason9075/agents_of_dynasties/internal/world"
)

// --- Response shapes ---

type mapResponse struct {
	Width  int            `json:"width"`
	Height int            `json:"height"`
	Tiles  []terrain.Tile `json:"tiles"`
}

type unitView struct {
	ID       entity.EntityID `json:"id"`
	Kind     string          `json:"kind"`
	Team     entity.Team     `json:"team"`
	Position coordView       `json:"position"`
	HP       int             `json:"hp"`
	MaxHP    int             `json:"max_hp"`
	Friendly bool            `json:"friendly"`
}

type buildingView struct {
	ID       entity.EntityID `json:"id"`
	Kind     string          `json:"kind"`
	Team     entity.Team     `json:"team"`
	Position coordView       `json:"position"`
	HP       int             `json:"hp"`
	MaxHP    int             `json:"max_hp"`
	Friendly bool            `json:"friendly"`
}

type coordView struct {
	Q int `json:"q"`
	R int `json:"r"`
}

type stateResponse struct {
	Tick      uint64         `json:"tick"`
	Resources world.Resources `json:"resources"`
	Units     []unitView     `json:"units"`
	Buildings []buildingView `json:"buildings"`
}

// --- Map handler (cached after first call) ---

type mapHandler struct {
	w    *world.World
	once sync.Once
	data []byte
}

func (h *mapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	h.once.Do(func() {
		resp := mapResponse{
			Width:  hex.GridSize,
			Height: hex.GridSize,
			Tiles:  h.w.AllTiles(),
		}
		h.data, _ = json.Marshal(resp)
	})
	w.Header().Set("Content-Type", "application/json")
	w.Write(h.data)
}

// --- State handler ---

type stateHandler struct {
	w *world.World
}

func (h *stateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	team, err := teamFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ownUnits, ownBuildings, enemyUnits, enemyBuildings := h.w.VisibleTo(team)

	var units []unitView
	for _, u := range ownUnits {
		pos := u.Position()
		units = append(units, unitView{
			ID:       u.ID(),
			Kind:     u.Kind().String(),
			Team:     u.Team(),
			Position: coordView{Q: pos.Q, R: pos.R},
			HP:       u.HP(),
			MaxHP:    u.MaxHP(),
			Friendly: true,
		})
	}
	for _, u := range enemyUnits {
		pos := u.Position()
		units = append(units, unitView{
			ID:       u.ID(),
			Kind:     u.Kind().String(),
			Team:     u.Team(),
			Position: coordView{Q: pos.Q, R: pos.R},
			HP:       u.HP(),
			MaxHP:    u.MaxHP(),
			Friendly: false,
		})
	}

	var buildings []buildingView
	for _, b := range ownBuildings {
		pos := b.Position()
		buildings = append(buildings, buildingView{
			ID:       b.ID(),
			Kind:     b.Kind().String(),
			Team:     b.Team(),
			Position: coordView{Q: pos.Q, R: pos.R},
			HP:       b.HP(),
			MaxHP:    b.MaxHP(),
			Friendly: true,
		})
	}
	for _, b := range enemyBuildings {
		pos := b.Position()
		buildings = append(buildings, buildingView{
			ID:       b.ID(),
			Kind:     b.Kind().String(),
			Team:     b.Team(),
			Position: coordView{Q: pos.Q, R: pos.R},
			HP:       b.HP(),
			MaxHP:    b.MaxHP(),
			Friendly: false,
		})
	}

	resp := stateResponse{
		Tick:      h.w.GetTick(),
		Resources: h.w.GetResources(team),
		Units:     units,
		Buildings: buildings,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --- Full state handler (god-mode, no LOS masking) ---

type fullStateTeam struct {
	Resources world.Resources `json:"resources"`
	Units     []unitView      `json:"units"`
	Buildings []buildingView  `json:"buildings"`
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
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	teamData := func(team entity.Team) fullStateTeam {
		var units []unitView
		for _, u := range h.w.UnitsByTeam(team) {
			pos := u.Position()
			units = append(units, unitView{
				ID:       u.ID(),
				Kind:     u.Kind().String(),
				Team:     u.Team(),
				Position: coordView{Q: pos.Q, R: pos.R},
				HP:       u.HP(),
				MaxHP:    u.MaxHP(),
				Friendly: true,
			})
		}
		var buildings []buildingView
		for _, b := range h.w.BuildingsByTeam(team) {
			pos := b.Position()
			buildings = append(buildings, buildingView{
				ID:       b.ID(),
				Kind:     b.Kind().String(),
				Team:     b.Team(),
				Position: coordView{Q: pos.Q, R: pos.R},
				HP:       b.HP(),
				MaxHP:    b.MaxHP(),
				Friendly: true,
			})
		}
		return fullStateTeam{
			Resources: h.w.GetResources(team),
			Units:     units,
			Buildings: buildings,
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
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	team, err := teamFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var cmd ticker.Command
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	cmd.Team = team

	// Verify the unit belongs to the requesting team.
	unit := h.w.GetUnit(cmd.UnitID)
	if unit == nil {
		writeError(w, http.StatusNotFound, "unit not found")
		return
	}
	if unit.Team() != team {
		writeError(w, http.StatusForbidden, "unit does not belong to your team")
		return
	}

	h.q.Submit(cmd)
	w.WriteHeader(http.StatusAccepted)
}
