//go:build postgres

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

type playerContextKey struct{}

type sessionReader interface {
	AuthenticateSession(context.Context, string) (string, error)
}

type ownershipReader interface {
	PlayerOwnsHousehold(context.Context, string, string) (bool, error)
	PlayerHasWorld(context.Context, string, string) (bool, error)
}

type adminReader interface {
	PlayerIsAdmin(context.Context, string) (bool, error)
}

// sessionAuthenticated establishes identity exclusively from an unexpired,
// unrevoked database session. It has no gameplay or administrator semantics.
func sessionAuthenticated(store sessionReader, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		authorization := strings.Fields(r.Header.Get("Authorization"))
		if len(authorization) != 2 || !strings.EqualFold(authorization[0], "Bearer") || len(authorization[1]) != 43 {
			writeJSON(w, 401, map[string]string{"error": "authentication_required"})
			return
		}
		playerID, err := store.AuthenticateSession(r.Context(), authorization[1])
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, 401, map[string]string{"error": "invalid_session"})
			return
		}
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "authentication_unavailable"})
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), playerContextKey{}, playerID))
		next.ServeHTTP(w, r)
	})
}

// gameplayAuthorized preserves the existing ownership rules. An administrator
// identity never bypasses these checks.
func gameplayAuthorized(store ownershipReader, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		playerID, _ := r.Context().Value(playerContextKey{}).(string)
		var householdID string
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 && parts[0] == "api" && parts[1] == "households" {
			householdID = parts[2]
		} else if r.Method == http.MethodPost {
			body, readErr := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
			if readErr != nil {
				writeJSON(w, 413, map[string]string{"error": "request_too_large"})
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			var intent struct {
				HouseholdID    string `json:"household_id"`
				BuyerID        string `json:"buyer_household_id"`
				ProposerID     string `json:"proposer_household_id"`
				CounterpartyID string `json:"counterparty_household_id"`
				DebtorID       string `json:"debtor_household_id"`
				SenderID       string `json:"sender_household_id"`
			}
			if json.Unmarshal(body, &intent) != nil {
				writeJSON(w, 400, map[string]string{"error": "invalid_json"})
				return
			}
			switch {
			case r.URL.Path == "/api/contracts" || r.URL.Path == "/api/contracts/preview":
				householdID = intent.ProposerID
			case len(parts) == 4 && parts[1] == "contracts" && parts[3] == "respond":
				householdID = intent.CounterpartyID
			case len(parts) == 4 && parts[1] == "contract-obligations" && parts[3] == "dispatch":
				householdID = intent.DebtorID
			case len(parts) == 4 && parts[1] == "political-demands" && parts[3] == "respond":
				householdID = intent.HouseholdID
			case len(parts) == 4 && parts[1] == "shipments" && parts[3] == "cancel":
				householdID = intent.SenderID
			case len(parts) == 5 && parts[1] == "market" && parts[2] == "offers" && (parts[4] == "purchase" || parts[4] == "quote"):
				householdID = intent.BuyerID
			}
		}
		allowed := false
		applicable := false
		var err error
		if r.URL.Path == "/api/market/offers" && r.Method == http.MethodGet {
			applicable = true
			allowed, err = store.PlayerHasWorld(r.Context(), playerID, r.URL.Query().Get("world_id"))
		} else if householdID != "" {
			applicable = true
			allowed, err = store.PlayerOwnsHousehold(r.Context(), playerID, householdID)
		}
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "authorization_unavailable"})
			return
		}
		if !applicable {
			next.ServeHTTP(w, r)
			return
		}
		if !allowed {
			writeJSON(w, 403, map[string]string{"error": "household_forbidden"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func adminAuthorized(store adminReader, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		playerID, _ := r.Context().Value(playerContextKey{}).(string)
		isAdmin, err := store.PlayerIsAdmin(r.Context(), playerID)
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "authorization_unavailable"})
			return
		}
		if !isAdmin {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "administrator_required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authenticated is retained as the combined gameplay middleware used by
// existing focused tests and callers. Production route registration composes
// the concerns explicitly.
func authenticated(store interface {
	sessionReader
	ownershipReader
}, next http.Handler) http.Handler {
	return sessionAuthenticated(store, gameplayAuthorized(store, next))
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	playerID, _ := r.Context().Value(playerContextKey{}).(string)
	households, err := s.store.ListOwnedHouseholds(r.Context(), playerID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	isAdmin, err := s.store.PlayerIsAdmin(r.Context(), playerID)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "authorization_unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"player_id": playerID, "is_admin": isAdmin, "households": households})
}
