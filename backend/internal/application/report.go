package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"game/backend/internal/balance"
	"game/backend/internal/calendar"
	chronicle_domain "game/backend/internal/domain/chronicle"
	reportdomain "game/backend/internal/domain/report"
	workdomain "game/backend/internal/domain/work"
	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

type Alert struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type FarmReport struct {
	HouseholdID      string        `json:"household_id"`
	HouseholdName    string        `json:"household_name"`
	WorldID          string        `json:"world_id"`
	SettingStartYear int32         `json:"setting_start_year"`
	Tick             int64         `json:"tick"`
	GameDay          int64         `json:"game_day"`
	Calendar         calendar.Date `json:"calendar"`
	// HistoricalDate is retained for legacy Go fixtures only; it is not part of
	// the player-facing report contract.
	HistoricalDate           string                      `json:"-"`
	Season                   simulation.Season           `json:"season"`
	SeasonDay                int                         `json:"season_day,omitempty"`
	SeasonLengthDays         int                         `json:"season_length_days,omitempty"`
	CurrentMoment            *calendar.Moment            `json:"current_moment,omitempty"`
	WorldUTCOffsetMinutes    *int                        `json:"world_utc_offset_minutes,omitempty"`
	Workday                  *calendar.Workday           `json:"workday,omitempty"`
	NextWorkingPeriod        *calendar.Moment            `json:"next_working_period,omitempty"`
	NextProductionSettlement *calendar.Moment            `json:"next_production_settlement,omitempty"`
	SupplyGameDays           int64                       `json:"supply_game_days"`
	SupplyStatus             string                      `json:"supply_status"`
	Resources                map[string]float64          `json:"resources"`
	Characters               []port.CharacterRecord      `json:"characters"`
	Assignments              []port.AssignmentRecord     `json:"assignments"`
	Alerts                   []Alert                     `json:"alerts"`
	ChangeWindow             ChangeWindow                `json:"change_window"`
	RecentChanges            []port.ChronicleEntryRecord `json:"recent_changes"`
	SinceYouWereAway         []port.ChronicleEntryRecord `json:"since_you_were_away"`
	ChronicleCursor          int64                       `json:"chronicle_cursor"`
	Attention                []reportdomain.Item         `json:"attention"`
	Decisions                []reportdomain.Item         `json:"decisions"`
	SimulationModel          port.SimulationModel        `json:"simulation_model"`
	PendingOutput            map[string]int64            `json:"pending_output_milli,omitempty"`
	ForecastNet              map[string]int64            `json:"forecast_net_milli,omitempty"`
	MinimumFoodMilli         int64                       `json:"minimum_expected_food_milli,omitempty"`
}

type ChangeWindow struct {
	FromTick int64 `json:"from_tick"`
	ToTick   int64 `json:"to_tick"`
}

type ReportService struct {
	Store   port.ReportReader
	Balance simulation.BalanceConfig
}

const (
	recentChangeWindowSeconds  = int64(24 * time.Hour / time.Second)
	maxRecentChangeWindowTicks = int64(12)
)

func recentChangeWindowTicks(tickDurationSeconds int32) int64 {
	if tickDurationSeconds <= 0 {
		return 1
	}

	seconds := int64(tickDurationSeconds)
	ticks := (recentChangeWindowSeconds + seconds - 1) / seconds
	if ticks < 1 {
		return 1
	}
	if ticks > maxRecentChangeWindowTicks {
		return maxRecentChangeWindowTicks
	}
	return ticks
}

func NewReportService(store port.ReportReader) *ReportService {
	return &ReportService{Store: store, Balance: balance.V03()}
}

func (s *ReportService) FarmReport(ctx context.Context, householdID string) (FarmReport, error) {
	snap, err := s.Store.GetHouseholdReport(ctx, householdID)
	if err != nil {
		return FarmReport{}, err
	}
	supply := snap.State.SupplyGameDays(s.Balance, snap.GameDaysPerTickNum, snap.GameDaysPerTickDen)
	pendingOutput := map[string]int64{}
	forecastNet := map[string]int64{}
	minimumFood := int64(0)
	reliableFoodReplenishment := false
	if snap.SimulationModel.UsesHourlyLabor() && snap.DailyLabor != nil {
		dailyCfg := balance.DailyLaborV1()
		if snap.SimulationModel == port.ModelMonthlySeasons {
			dailyCfg = balance.MonthlySeasonsV1()
		}
		consumption := simulation.DailyConsumptionPerDay(*snap.DailyLabor, dailyCfg)
		if consumption > 0 {
			supply = snap.DailyLabor.ProvisionsMilli / consumption
		}
		pendingOutput = map[string]int64{"provisions": snap.DailyLabor.PendingProvisionsMilli, "wood": snap.DailyLabor.PendingWoodMilli}
		if forecast, forecastErr := ForecastHousehold(snap, nil, 7); forecastErr == nil {
			forecastNet = map[string]int64{"provisions": forecast.BaselineEndingStocks.ProvisionsMilli - snap.DailyLabor.ProvisionsMilli, "wood": forecast.BaselineEndingStocks.WoodMilli - snap.DailyLabor.WoodMilli}
			minimumFood = forecast.MinimumProvisionsMilli
			hasConfirmedFoodArrival := false
			for _, shipment := range snap.IncomingShipments {
				if shipment.Status == "in_transit" && shipment.ResourceType == "provisions" {
					hasConfirmedFoodArrival = true
					break
				}
			}
			reliableFoodReplenishment = forecast.FirstShortageAt == nil && minimumFood > 0 && (hasConfirmedFoodArrival || forecast.BaselineEndingStocks.ProvisionsMilli >= snap.DailyLabor.ProvisionsMilli)
		}
	}
	// Alerts is retained as a compatibility field for older clients. New
	// clients consume structured Attention items below; no localized prose is
	// generated by the backend.
	alerts := make([]Alert, 0)
	gameDay := calendar.GameDay(snap.CurrentGameDay)
	if snap.GameDaysPerTickNum <= 0 || snap.GameDaysPerTickDen <= 0 || snap.CalendarRemainder < 0 || snap.CalendarRemainder >= snap.GameDaysPerTickDen {
		return FarmReport{}, fmt.Errorf("invalid game-day clock")
	}
	for i := range snap.Characters {
		var err error
		definition, definitionErr := calendar.DefinitionForModel(string(snap.SimulationModel))
		if definitionErr != nil {
			return FarmReport{}, definitionErr
		}
		snap.Characters[i].Age, err = definition.Age(calendar.GameDay(snap.Characters[i].BirthGameDay), gameDay)
		if err != nil {
			return FarmReport{}, fmt.Errorf("character %s age: %w", snap.Characters[i].ID, err)
		}
	}
	assignments := snap.Assignments
	if assignments == nil {
		assignments = make([]port.AssignmentRecord, 0)
	}
	characters := snap.Characters
	if characters == nil {
		characters = make([]port.CharacterRecord, 0)
	}
	attentionInput := reportdomain.Input{CurrentTick: snap.CurrentTick, CurrentGameDay: snap.CurrentGameDay, SupplyGameDays: supply, ReliableFoodReplenishment: reliableFoodReplenishment}
	for _, c := range characters {
		attentionInput.Characters = append(attentionInput.Characters, reportdomain.Character{ID: c.ID, Name: c.Name, Fatigue: c.Fatigue})
	}
	var recent []port.ChronicleEntryRecord
	var sinceAway []port.ChronicleEntryRecord
	var political []port.PoliticalReportDemand
	var obligations []port.ContractReportObligation
	var observedCursor int64
	if reader, ok := s.Store.(port.FarmReportReader); ok {
		windowTicks := recentChangeWindowTicks(snap.TickDurationSeconds)
		from := snap.CurrentTick - windowTicks + 1
		if from < 0 {
			from = 0
		}
		recent, err = reader.ListRecentChronicleForReport(ctx, householdID, from, 50)
		if err != nil {
			return FarmReport{}, err
		}
		recent = selectSignificantChanges(recent)
		if precise, preciseOK := s.Store.(port.PreciseFarmReportReader); preciseOK {
			observedCursor, err = precise.CurrentChronicleCursor(ctx, householdID)
			if err != nil {
				return FarmReport{}, err
			}
			sinceAway, err = precise.ListChronicleSinceCursor(ctx, householdID, snap.LastSeenChronicleSequence, observedCursor, 100)
			if err != nil {
				return FarmReport{}, err
			}
		} else {
			sinceAway, err = reader.ListChronicleSinceGameDayForReport(ctx, householdID, snap.LastSeenGameDay, 100)
			if err != nil {
				return FarmReport{}, err
			}
		}
		sinceAway = selectSinceAway(sinceAway)
		for _, entry := range sinceAway {
			if entry.EntryType == chronicle_domain.FoodShortage {
				if number, ok := entry.Data["food_shortage_milli"].(json.Number); ok {
					value, err := number.Int64()
					if err == nil && value > attentionInput.FoodShortageMilli {
						attentionInput.FoodShortageMilli = value
					}
				}
			}
		}
		political, err = reader.ListPendingPoliticalDemandsForReport(ctx, householdID)
		if err != nil {
			return FarmReport{}, err
		}
		obligations, err = reader.ListContractObligationsForReport(ctx, householdID)
		if err != nil {
			return FarmReport{}, err
		}
	}
	for _, d := range political {
		attentionInput.PoliticalDemands = append(attentionInput.PoliticalDemands, reportdomain.PoliticalDemand{ID: d.ID, ActorName: d.ActorName, ExpiresTick: d.ExpiresTick, ExpiresGameDay: d.ExpiresGameDay})
	}
	for _, o := range obligations {
		attentionInput.ContractObligations = append(attentionInput.ContractObligations, reportdomain.ContractObligation{ID: o.ID, ResourceType: o.ResourceType, QuantityMilli: o.QuantityMilli, DueArrivalTick: o.DueArrivalTick, ExpectedArrivalTick: o.ExpectedArrivalTick, DueGameDay: o.DueGameDay, ExpectedArrivalGameDay: o.ExpectedArrivalGameDay})
	}
	attention := reportdomain.BuildAttention(attentionInput)
	decisions := reportdomain.BuildDecisions(attentionInput)
	if recent == nil {
		recent = make([]port.ChronicleEntryRecord, 0)
	}
	if sinceAway == nil {
		sinceAway = make([]port.ChronicleEntryRecord, 0)
	}
	if attention == nil {
		attention = make([]reportdomain.Item, 0)
	}
	if decisions == nil {
		decisions = make([]reportdomain.Item, 0)
	}
	windowTicks := recentChangeWindowTicks(snap.TickDurationSeconds)
	fromTick := snap.CurrentTick - windowTicks + 1
	if fromTick < 0 {
		fromTick = 0
	}
	definition, err := calendar.DefinitionForModel(string(snap.SimulationModel))
	if err != nil {
		return FarmReport{}, err
	}
	breakdown := definition.Breakdown(gameDay)
	season := simulation.Season(breakdown.ProductionSeason)
	var currentMoment, nextWork, nextSettlement *calendar.Moment
	var workday *calendar.Workday
	seasonDay, seasonLength := 0, 0
	if snap.SimulationModel.UsesHourlyLabor() && snap.DailyLabor != nil {
		moment, momentErr := calendar.MomentAtClock(calendar.ClockState{Day: gameDay, Remainder: snap.CalendarRemainder, GameDaysPerTickNum: snap.GameDaysPerTickNum, GameDaysPerTickDen: snap.GameDaysPerTickDen})
		if momentErr != nil {
			return FarmReport{}, momentErr
		}
		temporal, temporalErr := temporalContext(snap.SimulationModel, snap.CalendarAnchorAt, snap.WorldUTCOffsetMinutes, moment)
		if temporalErr != nil {
			return FarmReport{}, temporalErr
		}
		season = simulation.Season(temporal.Context.Season)
		breakdown.ProductionSeason = temporal.Context.Season
		currentMoment = &moment
		wd := temporal.Context.Workday
		workday = &wd
		if snap.SimulationModel == port.ModelMonthlySeasons {
			seasonDay, seasonLength = temporal.Position.Day, temporal.Position.LengthDays
			next, nextErr := calendar.NextWorkStartForWorld(*snap.CalendarAnchorAt, *snap.WorldUTCOffsetMinutes, moment, calendar.DefaultDaylightConfig())
			if nextErr != nil {
				return FarmReport{}, nextErr
			}
			nextWork = &next
		} else {
			next, nextErr := calendar.NextWorkStart(calendar.ClockState{Day: gameDay, Remainder: snap.CalendarRemainder, GameDaysPerTickNum: snap.GameDaysPerTickNum, GameDaysPerTickDen: snap.GameDaysPerTickDen})
			if nextErr != nil {
				return FarmReport{}, nextErr
			}
			nextWork = &next
		}
		settlement := calendar.Moment{Day: moment.Day, Hour: wd.EndHour}
		if moment.Hour >= wd.EndHour {
			tomorrow := calendar.Moment{Day: moment.Day + 1, Hour: 0}
			if snap.SimulationModel == port.ModelMonthlySeasons {
				tomorrowTemporal, tomorrowErr := temporalContext(snap.SimulationModel, snap.CalendarAnchorAt, snap.WorldUTCOffsetMinutes, tomorrow)
				if tomorrowErr != nil {
					return FarmReport{}, tomorrowErr
				}
				settlement = calendar.Moment{Day: tomorrow.Day, Hour: tomorrowTemporal.Context.Workday.EndHour}
			} else {
				settlement = calendar.Moment{Day: tomorrow.Day, Hour: 17}
			}
		}
		nextSettlement = &settlement
		policy := simulation.WorkPolicyDecision{}
		cfg := balance.MonthlySeasonsV1()
		if snap.SimulationModel == port.ModelDailyLabor {
			cfg = balance.DailyLaborV1()
			policy = simulation.ResolveReservePolicy(*snap.DailyLabor, cfg)
		}
		for i := range snap.DailyLabor.Characters {
			character := &snap.DailyLabor.Characters[i]
			policyActivity, policyReason := policy.ActivityFor(character)
			resolved, resolveErr := workdomain.ResolveEffectiveWork(workdomain.WorkResolutionInput{Moment: moment, Workday: wd, Status: character.Status, LaborPermille: character.LaborPermille, Occupation: character.Occupation, TemporaryDuties: character.TemporaryDuties, PolicyActivity: policyActivity, PolicyReason: policyReason})
			if resolveErr != nil {
				return FarmReport{}, resolveErr
			}
			for j := range snap.Characters {
				if snap.Characters[j].ID == character.ID {
					snap.Characters[j].CurrentActivity = string(resolved.Activity)
					snap.Characters[j].CurrentActivityReason = resolved.Reason
				}
			}
		}
		characters = snap.Characters
	}
	supplyStatus := simulation.SupplyStatus(supply, s.Balance)
	if reliableFoodReplenishment {
		supplyStatus = "safe"
	}
	return FarmReport{
		HouseholdID: snap.HouseholdID, HouseholdName: snap.HouseholdName, WorldID: snap.WorldID,
		SettingStartYear: snap.SettingStartYear, Tick: snap.CurrentTick, GameDay: int64(gameDay), Calendar: breakdown,
		Season: season, SeasonDay: seasonDay, SeasonLengthDays: seasonLength, CurrentMoment: currentMoment,
		WorldUTCOffsetMinutes: snap.WorldUTCOffsetMinutes, Workday: workday, NextWorkingPeriod: nextWork, NextProductionSettlement: nextSettlement,
		SupplyGameDays: supply, SupplyStatus: supplyStatus,
		Resources:  map[string]float64{"provisions": float64(snap.State.ProvisionsMilli) / 1000, "wood": float64(snap.State.WoodMilli) / 1000, "trade_goods": float64(snap.State.TradeGoodsMilli) / 1000, "silver": float64(snap.State.SilverMilli) / 1000},
		Characters: characters, Assignments: assignments, Alerts: alerts,
		ChangeWindow: ChangeWindow{FromTick: fromTick, ToTick: snap.CurrentTick}, RecentChanges: recent,
		SinceYouWereAway: sinceAway,
		Attention:        attention, Decisions: decisions,
		SimulationModel: snap.SimulationModel, PendingOutput: pendingOutput, ForecastNet: forecastNet, MinimumFoodMilli: minimumFood,
		ChronicleCursor: observedCursor,
	}, nil
}

func (s *ReportService) Acknowledge(ctx context.Context, householdID string, gameDay int64) error {
	writer, ok := s.Store.(port.ReportAcknowledgementWriter)
	if !ok {
		return fmt.Errorf("report acknowledgement is unavailable")
	}
	return writer.AcknowledgeHouseholdReport(ctx, householdID, gameDay)
}

func (s *ReportService) AcknowledgeCursor(ctx context.Context, householdID string, cursor int64) error {
	writer, ok := s.Store.(port.PreciseReportAcknowledgementWriter)
	if !ok {
		return fmt.Errorf("precise report acknowledgement is unavailable")
	}
	return writer.AcknowledgeHouseholdReportCursor(ctx, householdID, cursor)
}

func selectSinceAway(entries []port.ChronicleEntryRecord) []port.ChronicleEntryRecord {
	importance := map[string]int{
		chronicle_domain.FoodShortage: 100, chronicle_domain.EmergencyFoodWorkScheduled: 95,
		chronicle_domain.EmergencyFoodPolicyNoAction: 94, chronicle_domain.ContractObligationBroken: 90,
		chronicle_domain.ContractObligationLate: 85, chronicle_domain.PoliticalDemandAutoResolved: 80,
		chronicle_domain.ShipmentArrived: 70, chronicle_domain.EmergencyWorkOverridden: 60,
	}
	selected := entries[:0]
	for _, entry := range entries {
		if importance[entry.EntryType] > 0 {
			selected = append(selected, entry)
		}
	}
	sort.SliceStable(selected, func(i, j int) bool {
		pi, pj := importance[selected[i].EntryType], importance[selected[j].EntryType]
		if pi != pj {
			return pi > pj
		}
		if selected[i].OccurredGameDay != selected[j].OccurredGameDay {
			return selected[i].OccurredGameDay > selected[j].OccurredGameDay
		}
		return selected[i].ID > selected[j].ID
	})
	if len(selected) > 5 {
		selected = selected[:5]
	}
	return selected
}

func selectSignificantChanges(entries []port.ChronicleEntryRecord) []port.ChronicleEntryRecord {
	importance := map[string]int{
		chronicle_domain.ShipmentArrived: 3, chronicle_domain.ContractObligationBroken: 3, chronicle_domain.ContractObligationLate: 3,
		chronicle_domain.PoliticalDemandAutoResolved: 3, chronicle_domain.EmergencyFoodWorkScheduled: 3,
		chronicle_domain.ContractObligationFulfilled: 2, chronicle_domain.PoliticalDemandReceived: 2, chronicle_domain.PoliticalDemandResolved: 2,
		chronicle_domain.MarketPurchase: 2, chronicle_domain.MarketSale: 2, chronicle_domain.ContractAccepted: 2, chronicle_domain.ContractRejected: 2,
		chronicle_domain.AssignmentScheduled: 1, chronicle_domain.AssignmentCompleted: 1, chronicle_domain.ContractShipmentDispatched: 1,
	}
	sort.SliceStable(entries, func(i, j int) bool {
		pi, pj := importance[entries[i].EntryType], importance[entries[j].EntryType]
		if pi != pj {
			return pi > pj
		}
		if entries[i].OccurredGameDay != entries[j].OccurredGameDay {
			return entries[i].OccurredGameDay > entries[j].OccurredGameDay
		}
		if entries[i].OccurredTick != entries[j].OccurredTick {
			return entries[i].OccurredTick > entries[j].OccurredTick
		}
		if entries[i].EntryType != entries[j].EntryType {
			return entries[i].EntryType < entries[j].EntryType
		}
		return entries[i].ID < entries[j].ID
	})
	if len(entries) > 3 {
		entries = entries[:3]
	}
	return entries
}
