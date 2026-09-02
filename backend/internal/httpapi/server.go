//go:build postgres

package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"game/backend/internal/application"
	"game/backend/internal/postgres"
	"game/backend/internal/simulation"
)

type Server struct {
	store   *postgres.Store
	reports *application.ReportService
	log     *slog.Logger
}

func New(store *postgres.Store, log *slog.Logger) http.Handler {
	s := &Server{store: store, reports: application.NewReportService(store), log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/households/{id}/report", s.farmReport)
	mux.HandleFunc("GET /api/households/{id}/assignments", s.assignments)
	mux.HandleFunc("POST /api/households/{id}/assignments", s.createAssignment)
	return cors(mux)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) farmReport(w http.ResponseWriter, r *http.Request) {
	report, err := s.reports.FarmReport(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) assignments(w http.ResponseWriter, r *http.Request) {
	snap, err := s.store.GetHouseholdReport(r.Context(), r.PathValue("id"), simulation.DefaultBalanceConfig())
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tick": snap.CurrentTick, "assignments": snap.Assignments})
}

type createAssignmentRequest struct {
	CharacterID   string `json:"character_id"`
	Activity      string `json:"activity"`
	Intensity     string `json:"intensity"`
	DurationTicks int64  `json:"duration_ticks"`
	StartsTick    *int64 `json:"starts_tick,omitempty"`
}

func (s *Server) createAssignment(w http.ResponseWriter, r *http.Request) {
	var req createAssignmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	if !validActivity(req.Activity) || !validIntensity(req.Intensity) || !validDuration(req.DurationTicks) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_assignment", "message": "activity/intensity/duration is invalid"})
		return
	}
	snap, err := s.store.GetHouseholdReport(r.Context(), r.PathValue("id"), simulation.DefaultBalanceConfig())
	if err != nil {
		s.writeError(w, err)
		return
	}
	starts := snap.CurrentTick + 1
	if req.StartsTick != nil {
		starts = *req.StartsTick
	}
	// Inclusive tick range: duration 1 means starts_tick == ends_tick.
	ends := starts + req.DurationTicks - 1
	out, err := s.store.CreateAssignment(r.Context(), r.PathValue("id"), req.CharacterID, req.Activity, req.Intensity, starts, ends)
	if err != nil {
		if strings.Contains(err.Error(), "overlaps") || strings.Contains(err.Error(), "starts_tick") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "assignment_conflict", "message": err.Error()})
			return
		}
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func validActivity(v string) bool {
	switch v {
	case "agriculture", "fishing", "woodcutting", "building", "crafting", "training", "market", "travel", "ruler_service", "rest":
		return true
	}
	return false
}
func validIntensity(v string) bool { return v == "light" || v == "normal" || v == "high" }
func validDuration(v int64) bool   { return v == 1 || v == 3 || v == 6 || v == 12 }

func (s *Server) writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	s.log.Error("api request failed", "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
