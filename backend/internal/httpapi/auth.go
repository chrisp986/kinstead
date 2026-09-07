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

type ownershipReader interface {
	AuthenticateSession(context.Context, string) (string, error)
	PlayerOwnsHousehold(context.Context, string, string) (bool, error)
	PlayerHasWorld(context.Context, string, string) (bool, error)
}

// Authorization runs before any gameplay handler. The authenticated identity
// comes exclusively from an unexpired, unrevoked database session. Body IDs
// select the requested household; they never establish the player's identity.
func authenticated(store ownershipReader, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
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
		if r.URL.Path == "/api/session" && r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}
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
		if r.URL.Path == "/api/market/offers" && r.Method == http.MethodGet {
			allowed, err = store.PlayerHasWorld(r.Context(), playerID, r.URL.Query().Get("world_id"))
		} else if householdID != "" {
			allowed, err = store.PlayerOwnsHousehold(r.Context(), playerID, householdID)
		}
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "authorization_unavailable"})
			return
		}
		if !allowed {
			writeJSON(w, 403, map[string]string{"error": "household_forbidden"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	playerID, _ := r.Context().Value(playerContextKey{}).(string)
	households, err := s.store.ListOwnedHouseholds(r.Context(), playerID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"households": households})
}
