package application

import (
	"context"
	"fmt"
	"time"

	"game/backend/internal/balance"
	"game/backend/internal/calendar"
	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

type Alert struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type FarmReport struct {
	HouseholdID    string                  `json:"household_id"`
	HouseholdName  string                  `json:"household_name"`
	WorldID        string                  `json:"world_id"`
	Tick           int64                   `json:"tick"`
	HistoricalDate string                  `json:"historical_date"`
	Season         simulation.Season       `json:"season"`
	SupplyDays     float64                 `json:"supply_days"`
	Resources      map[string]float64      `json:"resources"`
	Characters     []port.CharacterRecord  `json:"characters"`
	Assignments    []port.AssignmentRecord `json:"assignments"`
	Alerts         []Alert                 `json:"alerts"`
}

type ReportService struct {
	Store   port.ReportReader
	Balance simulation.BalanceConfig
}

func NewReportService(store port.ReportReader) *ReportService {
	return &ReportService{Store: store, Balance: balance.V03()}
}

func (s *ReportService) FarmReport(ctx context.Context, householdID string) (FarmReport, error) {
	snap, err := s.Store.GetHouseholdReport(ctx, householdID)
	if err != nil {
		return FarmReport{}, err
	}
	supply := snap.State.SupplyDays(s.Balance)
	alerts := make([]Alert, 0, 3)
	switch {
	case supply < float64(s.Balance.EmergencySupplyDays):
		alerts = append(alerts, Alert{"critical", "supply_emergency", fmt.Sprintf("Vorräte reichen nur %.1f Tage.", supply)})
	case supply < float64(s.Balance.CriticalSupplyDays):
		alerts = append(alerts, Alert{"critical", "supply_critical", fmt.Sprintf("Vorräte reichen nur %.1f Tage.", supply)})
	case supply <= 30:
		alerts = append(alerts, Alert{"warning", "supply_strained", fmt.Sprintf("Vorräte reichen %.1f Tage.", supply)})
	}
	for _, c := range snap.Characters {
		if len(alerts) >= 3 {
			break
		}
		if c.Fatigue >= 70 {
			alerts = append(alerts, Alert{"warning", "fatigue", fmt.Sprintf("%s ist stark erschöpft (%d).", c.Name, c.Fatigue)})
		}
	}
	historical, err := (calendar.Clock{
		StartDate:   snap.HistoricalStart,
		DaysPerTick: calendar.Rational{Numerator: int64(snap.HistoricalDaysPerTickNum), Denominator: int64(snap.HistoricalDaysPerTickDen)},
	}).DateAtTick(snap.CurrentTick)
	if err != nil {
		return FarmReport{}, err
	}
	for i := range snap.Characters {
		birthDate, err := time.Parse(time.DateOnly, snap.Characters[i].BirthDate)
		if err != nil {
			return FarmReport{}, fmt.Errorf("character %s birth date: %w", snap.Characters[i].ID, err)
		}
		snap.Characters[i].Age, err = calendar.AgeOn(birthDate, historical)
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
	return FarmReport{
		HouseholdID: snap.HouseholdID, HouseholdName: snap.HouseholdName, WorldID: snap.WorldID,
		Tick: snap.CurrentTick, HistoricalDate: historical.Format(time.DateOnly), Season: simulation.SeasonForDate(historical),
		SupplyDays: supply,
		Resources:  map[string]float64{"provisions": float64(snap.State.ProvisionsMilli) / 1000, "wood": float64(snap.State.WoodMilli) / 1000, "trade_goods": float64(snap.State.TradeGoodsMilli) / 1000, "silver": float64(snap.State.SilverMilli) / 1000},
		Characters: characters, Assignments: assignments, Alerts: alerts,
	}, nil
}
