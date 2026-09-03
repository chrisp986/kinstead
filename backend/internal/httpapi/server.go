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
	marketdomain "game/backend/internal/domain/market"
	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/postgres"
)

type Server struct {
	store     *postgres.Store
	reports   *application.ReportService
	shipments *application.ShipmentService
	market    *application.MarketService
	chronicle *application.ChronicleService
	log       *slog.Logger
}

func New(store *postgres.Store, log *slog.Logger) http.Handler {
	s := &Server{
		store: store, reports: application.NewReportService(store),
		shipments: application.NewShipmentService(store),
		market:    application.NewMarketService(store), log: log,
		chronicle: application.NewChronicleService(store),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/households/{id}/report", s.farmReport)
	mux.HandleFunc("GET /api/households/{id}/assignments", s.assignments)
	mux.HandleFunc("POST /api/households/{id}/assignments", s.createAssignment)
	mux.HandleFunc("GET /api/households/{id}/shipments", s.householdShipments)
	mux.HandleFunc("POST /api/shipments/{id}/cancel", s.cancelShipment)
	mux.HandleFunc("GET /api/households/{id}/chronicle", s.householdChronicle)
	mux.HandleFunc("GET /api/market/offers", s.marketOffers)
	mux.HandleFunc("POST /api/market/offers/{id}/purchase", s.purchaseMarketOffer)
	return cors(mux)
}

func (s *Server) householdChronicle(w http.ResponseWriter, r *http.Request) {
	entries, err := s.chronicle.ListForHousehold(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func (s *Server) marketOffers(w http.ResponseWriter, r *http.Request) {
	worldID := r.URL.Query().Get("world_id")
	if worldID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "world_id_required"})
		return
	}
	offers, err := s.market.ListActiveOffers(r.Context(), worldID)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"offers": offers})
}

type purchaseMarketOfferRequest struct {
	BuyerHouseholdID string `json:"buyer_household_id"`
	QuantityMilli    int64  `json:"quantity_milli"`
}

func (s *Server) purchaseMarketOffer(w http.ResponseWriter, r *http.Request) {
	var req purchaseMarketOfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	result, err := s.market.PurchaseOffer(r.Context(), application.PurchaseOfferCommand{
		OfferID: r.PathValue("id"), BuyerHouseholdID: req.BuyerHouseholdID, QuantityMilli: req.QuantityMilli,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) householdShipments(w http.ResponseWriter, r *http.Request) {
	shipments, err := s.shipments.ListForHousehold(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shipments": shipments})
}

type cancelShipmentRequest struct {
	SenderHouseholdID string `json:"sender_household_id"`
}

func (s *Server) cancelShipment(w http.ResponseWriter, r *http.Request) {
	var req cancelShipmentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	shipment, err := s.shipments.Cancel(r.Context(), application.CancelShipmentCommand{
		ShipmentID: shipmentdomain.ID(r.PathValue("id")), SenderHouseholdID: shipmentdomain.HouseholdID(req.SenderHouseholdID),
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, shipment)
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
	snap, err := s.store.GetHouseholdReport(r.Context(), r.PathValue("id"))
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
	snap, err := s.store.GetHouseholdReport(r.Context(), r.PathValue("id"))
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
	case "agriculture", "fishing", "woodcutting", "rest":
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
	if errors.Is(err, postgres.ErrInvalidMarketParticipants) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "market_participant_not_found"})
		return
	}
	if errors.Is(err, marketdomain.ErrInvalidQuantity) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_purchase", "message": err.Error()})
		return
	}
	if errors.Is(err, marketdomain.ErrRouteUnavailable) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "route_unavailable", "message": err.Error()})
		return
	}
	if errors.Is(err, shipmentdomain.ErrCancellationForbidden) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "shipment_cancellation_forbidden", "message": err.Error()})
		return
	}
	if errors.Is(err, shipmentdomain.ErrCancellationClosed) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "shipment_cancellation_closed", "message": err.Error()})
		return
	}
	if errors.Is(err, marketdomain.ErrOfferUnavailable) ||
		errors.Is(err, marketdomain.ErrOfferExpired) ||
		errors.Is(err, marketdomain.ErrInsufficientOffer) ||
		errors.Is(err, marketdomain.ErrInsufficientFunds) ||
		errors.Is(err, marketdomain.ErrInsufficientStock) ||
		errors.Is(err, marketdomain.ErrOwnOffer) ||
		errors.Is(err, postgres.ErrMarketStateChanged) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "purchase_conflict", "message": err.Error()})
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
