//go:build postgres

package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"game/backend/internal/application"
	contractdomain "game/backend/internal/domain/contract"
	"game/backend/internal/domain/geography"
	marketdomain "game/backend/internal/domain/market"
	politicsdomain "game/backend/internal/domain/politics"
	shipmentdomain "game/backend/internal/domain/shipment"
	"game/backend/internal/port"
	"game/backend/internal/postgres"
)

type Server struct {
	store         *postgres.Store
	reports       *application.ReportService
	shipments     *application.ShipmentService
	market        *application.MarketService
	chronicle     *application.ChronicleService
	relationships *application.RelationshipService
	contracts     *application.ContractService
	politics      *application.PoliticsService
	calendar      *application.CalendarService
	log           *slog.Logger
}

func New(store *postgres.Store, log *slog.Logger) http.Handler {
	s := &Server{
		store: store, reports: application.NewReportService(store),
		shipments: application.NewShipmentService(store),
		market:    application.NewMarketService(store), log: log,
		chronicle:     application.NewChronicleService(store),
		relationships: application.NewRelationshipService(store),
		contracts:     application.NewContractService(store),
		politics:      application.NewPoliticsService(store, store),
		calendar:      application.NewCalendarService(store),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /api/households/{id}/report", s.farmReport)
	mux.HandleFunc("GET /api/households/{id}/calendar", s.householdCalendar)
	mux.HandleFunc("GET /api/households/{id}/assignments", s.assignments)
	mux.HandleFunc("POST /api/households/{id}/assignments", s.createAssignment)
	mux.HandleFunc("GET /api/households/{id}/shipments", s.householdShipments)
	mux.HandleFunc("POST /api/shipments/{id}/cancel", s.cancelShipment)
	mux.HandleFunc("GET /api/households/{id}/chronicle", s.householdChronicle)
	mux.HandleFunc("GET /api/households/{id}/relationships", s.householdRelationships)
	mux.HandleFunc("GET /api/households/{id}/contracts", s.householdContracts)
	mux.HandleFunc("GET /api/households/{id}/politics", s.householdPolitics)
	mux.HandleFunc("POST /api/contracts", s.proposeContract)
	mux.HandleFunc("POST /api/contracts/{id}/respond", s.respondContract)
	mux.HandleFunc("POST /api/contract-obligations/{id}/dispatch", s.dispatchContractObligation)
	mux.HandleFunc("POST /api/political-demands/{id}/respond", s.respondPoliticalDemand)
	mux.HandleFunc("GET /api/market/offers", s.marketOffers)
	mux.HandleFunc("POST /api/market/offers/{id}/purchase", s.purchaseMarketOffer)
	return cors(mux)
}

func (s *Server) householdPolitics(w http.ResponseWriter, r *http.Request) {
	p, err := s.politics.GetHouseholdPolitics(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) householdCalendar(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	var from, to int64
	fromSupplied, toSupplied := false, false
	var err error
	if values, ok := query["from_game_day"]; ok {
		if len(values) != 1 || values[0] == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_calendar_range"})
			return
		}
		fromSupplied = true
		raw := values[0]
		from, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_calendar_range"})
			return
		}
	}
	if values, ok := query["to_game_day"]; ok {
		if len(values) != 1 || values[0] == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_calendar_range"})
			return
		}
		toSupplied = true
		raw := values[0]
		to, err = strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_calendar_range"})
			return
		}
	}
	if toSupplied && !fromSupplied {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "from_game_day_required"})
		return
	}
	var fromPtr, toPtr *int64
	if fromSupplied {
		fromPtr = &from
	}
	if toSupplied {
		toPtr = &to
	}
	projection, err := s.calendar.HouseholdRange(r.Context(), r.PathValue("id"), fromPtr, toPtr, query.Get("category"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projection)
}
func (s *Server) respondPoliticalDemand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HouseholdID string `json:"household_id"`
		Option      string `json:"option"`
		CharacterID string `json:"character_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid_json"})
		return
	}
	if err := s.politics.Respond(r.Context(), application.RespondPoliticalDemandCommand{DecisionID: r.PathValue("id"), HouseholdID: req.HouseholdID, Option: req.Option, CharacterID: req.CharacterID}); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

func (s *Server) householdContracts(w http.ResponseWriter, r *http.Request) {
	contracts, err := s.contracts.ListDetailsForHousehold(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"contracts": contracts})
}

type proposeContractRequest struct {
	ProposerHouseholdID     string                            `json:"proposer_household_id"`
	CounterpartyHouseholdID string                            `json:"counterparty_household_id"`
	StartGameDay            *int64                            `json:"start_game_day,omitempty"`
	EndGameDay              *int64                            `json:"end_game_day,omitempty"`
	FirstDueGameDay         *int64                            `json:"first_due_game_day,omitempty"`
	IntervalDays            *int64                            `json:"interval_days,omitempty"`
	EndCondition            *application.ContractEndCondition `json:"end_condition,omitempty"`
	StartsTick              *int64                            `json:"starts_tick,omitempty"`
	EndsTick                *int64                            `json:"ends_tick,omitempty"`
	IntervalTicks           *int64                            `json:"interval_ticks,omitempty"`
	Terms                   []application.ContractTermIntent  `json:"terms"`
}

func (s *Server) proposeContract(w http.ResponseWriter, r *http.Request) {
	var req proposeContractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	command := application.ProposeContractCommand{
		ProposerHouseholdID: req.ProposerHouseholdID, CounterpartyHouseholdID: req.CounterpartyHouseholdID,
		Terms: req.Terms,
	}
	dayFields := req.StartGameDay != nil || req.EndGameDay != nil || req.FirstDueGameDay != nil || req.IntervalDays != nil
	tickFields := req.StartsTick != nil || req.EndsTick != nil || req.IntervalTicks != nil
	if dayFields && tickFields {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "conflicting_contract_schedule"})
		return
	}
	if (req.StartGameDay != nil || req.EndGameDay != nil) && req.FirstDueGameDay != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "conflicting_contract_schedule"})
		return
	}
	if req.StartGameDay != nil && req.IntervalDays != nil && (req.EndGameDay != nil || req.EndCondition != nil) {
		command.StartGameDay, command.IntervalDays = *req.StartGameDay, *req.IntervalDays
		if req.EndGameDay != nil {
			command.EndGameDay = *req.EndGameDay
		}
		if req.EndCondition != nil {
			command.EndCondition = *req.EndCondition
		}
	} else if req.FirstDueGameDay != nil && req.IntervalDays != nil {
		command.StartGameDay, command.IntervalDays = *req.FirstDueGameDay, *req.IntervalDays
		if req.EndCondition != nil {
			command.EndCondition = *req.EndCondition
		}
	} else if req.StartsTick != nil && req.EndsTick != nil && req.IntervalTicks != nil {
		command.StartsTick, command.EndsTick, command.IntervalTicks = *req.StartsTick, *req.EndsTick, *req.IntervalTicks
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_contract_schedule"})
		return
	}
	created, err := s.contracts.Propose(r.Context(), command)
	if err != nil {
		s.writeError(w, err)
		return
	}
	projection, err := s.contracts.Detail(r.Context(), string(created.ID))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, projection)
}

type respondContractRequest struct {
	CounterpartyHouseholdID string `json:"counterparty_household_id"`
	Decision                string `json:"decision"`
}

func (s *Server) respondContract(w http.ResponseWriter, r *http.Request) {
	var req respondContractRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	if req.Decision != "accept" && req.Decision != "reject" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_contract_decision"})
		return
	}
	updated, err := s.contracts.Respond(r.Context(), application.RespondContractCommand{
		ContractID: r.PathValue("id"), CounterpartyHouseholdID: req.CounterpartyHouseholdID,
		Accept: req.Decision == "accept",
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	projection, err := s.contracts.Detail(r.Context(), string(updated.ID))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, projection)
}

type dispatchContractObligationRequest struct {
	DebtorHouseholdID string `json:"debtor_household_id"`
}

type dispatchContractObligationResponse struct {
	Obligation application.ContractObligationProjection `json:"obligation"`
	Shipment   port.ShipmentRecord                      `json:"shipment"`
}

func (s *Server) dispatchContractObligation(w http.ResponseWriter, r *http.Request) {
	var req dispatchContractObligationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	result, err := s.contracts.DispatchObligation(r.Context(), application.DispatchContractObligationCommand{
		ObligationID: r.PathValue("id"), DebtorHouseholdID: req.DebtorHouseholdID,
	})
	if err != nil {
		s.writeError(w, err)
		return
	}
	projection, err := s.contracts.Detail(r.Context(), string(result.Obligation.ContractID))
	if err != nil {
		s.writeError(w, err)
		return
	}
	var obligation application.ContractObligationProjection
	for _, candidate := range projection.Obligations {
		if candidate.ID == string(result.Obligation.ID) {
			obligation = candidate
			break
		}
	}
	writeJSON(w, http.StatusCreated, dispatchContractObligationResponse{
		Obligation: obligation,
		Shipment:   contractShipmentRecord(result.Shipment),
	})
}

func contractShipmentRecord(value shipmentdomain.Shipment) port.ShipmentRecord {
	record := port.ShipmentRecord{
		ID: string(value.ID), WorldID: string(value.WorldID),
		SenderHouseholdID: string(value.SenderHouseholdID), ReceiverHouseholdID: string(value.ReceiverHouseholdID),
		OriginLocationID: string(value.OriginLocationID), DestinationLocationID: string(value.DestinationLocationID),
		ResourceType: string(value.ResourceType), QuantityMilli: int64(value.QuantityMilli),
		DepartureTick: int64(value.DepartureTick), ExpectedArrivalTick: int64(value.ExpectedArrivalTick),
		DepartureGameDay: int64(value.DepartureGameDay), ExpectedArrivalGameDay: int64(value.ExpectedArrivalGameDay),
		TransportCostMilli: int64(value.TransportCostMilli), Status: string(value.Status),
	}
	if value.ActualArrivalTick != nil {
		actual := int64(*value.ActualArrivalTick)
		record.ActualArrivalTick = &actual
	}
	if value.ActualArrivalGameDay != nil {
		actual := int64(*value.ActualArrivalGameDay)
		record.ActualArrivalGameDay = &actual
	}
	return record
}

func (s *Server) householdRelationships(w http.ResponseWriter, r *http.Request) {
	relationships, err := s.relationships.ListForHousehold(r.Context(), r.PathValue("id"))
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"relationships": relationships})
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
	if errors.Is(err, politicsdomain.ErrInvalidOption) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_political_option", "message": err.Error()})
		return
	}
	if errors.Is(err, port.ErrConcurrentTransaction) ||
		errors.Is(err, application.ErrPoliticalDemandResolved) ||
		errors.Is(err, politicsdomain.ErrExpired) ||
		errors.Is(err, politicsdomain.ErrMissingCharacter) ||
		errors.Is(err, politicsdomain.ErrIneligibleCharacter) ||
		errors.Is(err, politicsdomain.ErrServiceOverlap) ||
		errors.Is(err, politicsdomain.ErrInsufficientResources) ||
		strings.Contains(err.Error(), "overlaps") {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "political_demand_conflict", "message": err.Error()})
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
	if errors.Is(err, contractdomain.ErrInvalidContract) || errors.Is(err, contractdomain.ErrInvalidObligation) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_contract", "message": err.Error()})
		return
	}
	if errors.Is(err, application.ErrInvalidContractSchedule) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_contract_schedule"})
		return
	}
	if errors.Is(err, application.ErrInvalidCalendarCategory) || errors.Is(err, application.ErrInvalidCalendarRange) || errors.Is(err, application.ErrCalendarFromRequired) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_calendar_request"})
		return
	}
	if errors.Is(err, application.ErrContractResponseForbidden) || errors.Is(err, application.ErrContractDispatchForbidden) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "contract_action_forbidden", "message": err.Error()})
		return
	}
	if errors.Is(err, postgres.ErrInvalidContractParticipants) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "contract_participant_not_found", "message": err.Error()})
		return
	}
	if errors.Is(err, application.ErrContractStartsInPast) || errors.Is(err, contractdomain.ErrInvalidTransition) ||
		errors.Is(err, contractdomain.ErrShipmentMismatch) || errors.Is(err, geography.ErrRouteUnavailable) ||
		errors.Is(err, postgres.ErrContractDispatchStateChanged) || errors.Is(err, postgres.ErrInsufficientResources) ||
		errors.Is(err, postgres.ErrShipmentGameDayConflict) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "contract_conflict", "message": err.Error()})
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
