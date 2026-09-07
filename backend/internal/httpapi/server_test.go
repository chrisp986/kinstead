//go:build postgres

package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"game/backend/internal/application"
	shipmentdomain "game/backend/internal/domain/shipment"
)

func TestValidActivityMatchesOnlineSimulation(t *testing.T) {
	for _, activity := range []string{"agriculture", "fishing", "woodcutting", "rest"} {
		if !validActivity(activity) {
			t.Fatalf("%q should be available online", activity)
		}
	}
	for _, activity := range []string{"building", "crafting", "training", "market", "travel", "ruler_service"} {
		if validActivity(activity) {
			t.Fatalf("%q has no implemented online effect and must be rejected", activity)
		}
	}
}

func TestNewPreviewEndpointsRejectInvalidInputBeforePersistence(t *testing.T) {
	s := &Server{}
	tests := []struct {
		name       string
		handler    func(http.ResponseWriter, *http.Request)
		path, body string
	}{
		{"market quote", s.quoteMarketOffer, "/api/market/offers/id/quote", "{"},
		{"work preview", s.previewWork, "/api/households/id/work-preview", `{"character_id":"x","activity":"combat","intensity":"normal","duration_ticks":1}`},
		{"report acknowledgement", s.acknowledgeFarmReport, "/api/households/id/report/acknowledge", `{"game_day":-1}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
			response := httptest.NewRecorder()
			tc.handler(response, request)
			if response.Code != 400 {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestContractShipmentProjectionIncludesHouseholdNames(t *testing.T) {
	contract := application.ContractProjection{
		PartyAHouseholdID: "sender", PartyAHouseholdName: "Hof Sender",
		PartyBHouseholdID: "receiver", PartyBHouseholdName: "Hof Receiver",
	}
	shipment := contractShipmentRecord(shipmentdomain.Shipment{
		SenderHouseholdID: "sender", ReceiverHouseholdID: "receiver",
	}, contractHouseholdName(contract, "sender"), contractHouseholdName(contract, "receiver"))
	if shipment.SenderHouseholdName != "Hof Sender" || shipment.ReceiverHouseholdName != "Hof Receiver" {
		t.Fatalf("shipment names = %+v", shipment)
	}
}

func TestCORSAllowsSessionAuthorizationHeader(t *testing.T) {
	handler := cors(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("preflight reached protected handler")
	}))
	request := httptest.NewRequest(http.MethodOptions, "/api/session", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || !strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("preflight status/headers = %d/%q", response.Code, response.Header().Get("Access-Control-Allow-Headers"))
	}
}
