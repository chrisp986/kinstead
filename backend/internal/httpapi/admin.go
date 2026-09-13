//go:build postgres

package httpapi

import (
	"net/http"
	"strconv"

	"game/backend/internal/application"
	"game/backend/internal/port"
)

func adminLimit(r *http.Request) int {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return 50
	}
	limit, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return limit
}

func (s *Server) adminWorlds(w http.ResponseWriter, r *http.Request) {
	items, cursor, err := s.admin.Worlds(r.Context(), r.URL.Query().Get("cursor"), adminLimit(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"worlds": items, "next_cursor": cursor})
}

func (s *Server) adminWorld(w http.ResponseWriter, r *http.Request) {
	item, err := s.admin.World(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) adminHouseholds(w http.ResponseWriter, r *http.Request) {
	items, cursor, err := s.admin.Households(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("cursor"), adminLimit(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"households": items, "next_cursor": cursor})
}

func (s *Server) adminHousehold(w http.ResponseWriter, r *http.Request) {
	item, err := s.admin.Household(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) adminHouseholdTicks(w http.ResponseWriter, r *http.Request) {
	var before *int64
	if value := r.URL.Query().Get("before_tick"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed < 0 {
			s.writeError(w, application.ErrInvalidAdminRequest)
			return
		}
		before = &parsed
	}
	items, err := s.admin.Ticks(r.Context(), r.PathValue("id"), before, adminLimit(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"diagnostics": items, "retained_days": 30})
}

func (s *Server) adminHouseholdTick(w http.ResponseWriter, r *http.Request) {
	tick, err := strconv.ParseInt(r.PathValue("tick"), 10, 64)
	if err != nil {
		s.writeError(w, application.ErrInvalidAdminRequest)
		return
	}
	item, err := s.admin.Tick(r.Context(), r.PathValue("id"), tick)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) adminErrors(w http.ResponseWriter, r *http.Request) {
	filter := port.AdminErrorFilter{WorldID: r.URL.Query().Get("world_id"), HouseholdID: r.URL.Query().Get("household_id"), Source: r.URL.Query().Get("source"), RequestID: r.URL.Query().Get("request_id")}
	if value := r.URL.Query().Get("tick"); value != "" {
		tick, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			s.writeError(w, application.ErrInvalidAdminRequest)
			return
		}
		filter.Tick = &tick
	}
	items, cursor, err := s.admin.Errors(r.Context(), filter, r.URL.Query().Get("cursor"), adminLimit(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"errors": items, "next_cursor": cursor})
}

func (s *Server) adminAccounts(w http.ResponseWriter, r *http.Request) {
	items, cursor, err := s.admin.Accounts(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("cursor"), adminLimit(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": items, "next_cursor": cursor})
}

func (s *Server) adminAccount(w http.ResponseWriter, r *http.Request) {
	item, err := s.admin.Account(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) adminSessions(w http.ResponseWriter, r *http.Request) {
	items, cursor, err := s.admin.Sessions(r.Context(), r.PathValue("id"), r.URL.Query().Get("cursor"), adminLimit(r))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": items, "next_cursor": cursor})
}
