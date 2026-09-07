//go:build postgres

package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

type authStub struct{}

func (authStub) AuthenticateSession(_ context.Context, token string) (string, error) {
	if token != strings.Repeat("a", 43) {
		return "", pgx.ErrNoRows
	}
	return "player", nil
}
func (authStub) PlayerOwnsHousehold(_ context.Context, player, household string) (bool, error) {
	return player == "player" && household == "owned", nil
}
func (authStub) PlayerHasWorld(_ context.Context, player, world string) (bool, error) {
	return player == "player" && world == "world", nil
}

func TestOwnershipEnforcedForEveryHouseholdReadAndCommand(t *testing.T) {
	cases := []struct{ method, path, body string }{
		{"GET", "/api/households/HOUSE/report", ""},
		{"GET", "/api/households/HOUSE/calendar", ""},
		{"GET", "/api/households/HOUSE/assignments", ""},
		{"GET", "/api/households/HOUSE/shipments", ""},
		{"GET", "/api/households/HOUSE/chronicle", ""},
		{"GET", "/api/households/HOUSE/relationships", ""},
		{"GET", "/api/households/HOUSE/contracts", ""},
		{"GET", "/api/households/HOUSE/politics", ""},
		{"POST", "/api/households/HOUSE/assignments", "{}"},
		{"POST", "/api/households/HOUSE/work-preview", "{}"},
		{"POST", "/api/households/HOUSE/report/acknowledge", "{}"},
		{"POST", "/api/market/offers/offer/quote", `{"buyer_household_id":"HOUSE"}`},
		{"POST", "/api/market/offers/offer/purchase", `{"buyer_household_id":"HOUSE"}`},
		{"POST", "/api/contracts", `{"proposer_household_id":"HOUSE","counterparty_household_id":"foreign"}`},
		{"POST", "/api/contracts/preview", `{"proposer_household_id":"HOUSE"}`},
		{"POST", "/api/contracts/contract/respond", `{"counterparty_household_id":"HOUSE"}`},
		{"POST", "/api/contract-obligations/obligation/dispatch", `{"debtor_household_id":"HOUSE"}`},
		{"POST", "/api/political-demands/demand/respond", `{"household_id":"HOUSE"}`},
		{"POST", "/api/shipments/shipment/cancel", `{"sender_household_id":"HOUSE"}`},
	}
	for _, tc := range cases {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			for _, owned := range []bool{false, true} {
				house := "foreign"
				want := 403
				if owned {
					house = "owned"
					want = 204
				}
				called := false
				handler := authenticated(authStub{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) }))
				r := httptest.NewRequest(tc.method, strings.ReplaceAll(tc.path, "HOUSE", house), strings.NewReader(strings.ReplaceAll(tc.body, "HOUSE", house)))
				r.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 43))
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				if w.Code != want || called != owned {
					t.Fatalf("owned=%v status=%d handler called=%v", owned, w.Code, called)
				}
			}
		})
	}
}

func TestAuthenticationFailsClosed(t *testing.T) {
	for _, token := range []string{"", "Bearer owned", "Bearer " + strings.Repeat("b", 43)} {
		r := httptest.NewRequest("GET", "/api/households/owned/report", nil)
		r.Header.Set("Authorization", token)
		r.Header.Set("X-Player-ID", "player")
		w := httptest.NewRecorder()
		authenticated(authStub{}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unauthenticated request reached gameplay") })).ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("status=%d", w.Code)
		}
	}
}
