package application

import (
	"fmt"

	"game/backend/internal/balance"
	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

type OccupationChange struct {
	CharacterID string
	Activity    workdomain.Activity
}

type StockProjection struct {
	ProvisionsMilli        int64 `json:"provisions_milli"`
	WoodMilli              int64 `json:"wood_milli"`
	PendingProvisionsMilli int64 `json:"pending_provisions_milli"`
	PendingWoodMilli       int64 `json:"pending_wood_milli"`
}

type ForecastWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type HouseholdForecast struct {
	SnapshotTick                   int64             `json:"snapshot_tick"`
	BasedOnRevision                int64             `json:"based_on_revision"`
	HorizonGameDays                int               `json:"horizon_game_days"`
	BaselineEndingStocks           StockProjection   `json:"baseline_ending_stocks"`
	ProposedEndingStocks           StockProjection   `json:"proposed_ending_stocks"`
	MinimumProvisionsMilli         int64             `json:"minimum_provisions_milli"`
	ProposedMinimumProvisionsMilli int64             `json:"proposed_minimum_provisions_milli"`
	FirstShortageAt                *calendar.Moment  `json:"first_shortage_at,omitempty"`
	ProposedFirstShortageAt        *calendar.Moment  `json:"proposed_first_shortage_at,omitempty"`
	Warnings                       []ForecastWarning `json:"warnings"`
	Assumptions                    []string          `json:"assumptions"`
}

// ForecastHousehold runs the same bounded hourly rules as execution. The
// forecast is intentionally seven game days at most for a request-facing
// preview and excludes unaccepted offers or unknown future events.
func ForecastHousehold(snapshot port.HouseholdSnapshot, proposedChange *OccupationChange, horizonGameDays int) (HouseholdForecast, error) {
	if horizonGameDays <= 0 || horizonGameDays > 7 {
		return HouseholdForecast{}, fmt.Errorf("forecast horizon must be between 1 and 7 game days")
	}
	if snapshot.DailyLabor == nil || !snapshot.SimulationModel.UsesHourlyLabor() {
		return HouseholdForecast{}, ErrUnsupportedSimulationModel
	}
	base := cloneDailyState(*snapshot.DailyLabor)
	proposed := cloneDailyState(*snapshot.DailyLabor)
	if proposedChange != nil {
		changed := false
		for i := range proposed.Characters {
			if proposed.Characters[i].ID != proposedChange.CharacterID {
				continue
			}
			if err := workdomain.ValidateOccupation(proposedChange.Activity); err != nil {
				return HouseholdForecast{}, err
			}
			next := proposedChange.Activity
			if next == proposed.Characters[i].Occupation.Activity {
				proposed.Characters[i].Occupation.PendingActivity = nil
				proposed.Characters[i].Occupation.EffectiveDay = nil
				proposed.Characters[i].Occupation.EffectiveHour = nil
			} else {
				proposed.Characters[i].Occupation.PendingActivity = &next
				clock := calendar.ClockState{Day: snapshot.DailyLabor.CurrentGameDay, Remainder: snapshot.CalendarRemainder, GameDaysPerTickNum: snapshot.GameDaysPerTickNum, GameDaysPerTickDen: snapshot.GameDaysPerTickDen}
				current, err := calendar.MomentAtClock(clock)
				if err != nil {
					return HouseholdForecast{}, err
				}
				var effective calendar.Moment
				if snapshot.SimulationModel == port.ModelMonthlySeasons {
					if snapshot.CalendarAnchorAt == nil || snapshot.WorldUTCOffsetMinutes == nil {
						return HouseholdForecast{}, ErrUnsupportedSimulationModel
					}
					effective, err = calendar.NextWorkStartForWorld(*snapshot.CalendarAnchorAt, *snapshot.WorldUTCOffsetMinutes, current, calendar.DefaultDaylightConfig())
				} else {
					effective, err = calendar.NextWorkStart(clock)
				}
				if err != nil {
					return HouseholdForecast{}, err
				}
				day := effective.Day
				hour := effective.Hour
				proposed.Characters[i].Occupation.EffectiveDay = &day
				proposed.Characters[i].Occupation.EffectiveHour = &hour
			}
			changed = true
			break
		}
		if !changed {
			return HouseholdForecast{}, fmt.Errorf("character %q is not in the household", proposedChange.CharacterID)
		}
	}
	baseStocks, baseMin, baseShortage, err := forecastState(base, snapshot, horizonGameDays)
	if err != nil {
		return HouseholdForecast{}, err
	}
	proposedStocks, proposedMin, proposedShortage, err := forecastState(proposed, snapshot, horizonGameDays)
	if err != nil {
		return HouseholdForecast{}, err
	}
	warnings := make([]ForecastWarning, 0)
	if baseShortage != nil {
		warnings = append(warnings, ForecastWarning{Code: "baseline_shortage", Message: "The current plan reaches a food shortage before the preview horizon."})
	}
	if proposedShortage != nil {
		warnings = append(warnings, ForecastWarning{Code: "proposed_shortage", Message: "This occupation change reaches a food shortage before the preview horizon."})
	}
	return HouseholdForecast{SnapshotTick: snapshot.CurrentTick, BasedOnRevision: forecastRevision(*snapshot.DailyLabor), HorizonGameDays: horizonGameDays, BaselineEndingStocks: baseStocks, ProposedEndingStocks: proposedStocks, MinimumProvisionsMilli: baseMin, ProposedMinimumProvisionsMilli: proposedMin, FirstShortageAt: baseShortage, ProposedFirstShortageAt: proposedShortage, Warnings: warnings,
		Assumptions: []string{"confirmed incoming shipments arrive at their scheduled tick", "known temporary commitments continue as scheduled", "no unconfirmed trades or future events are assumed"}}, nil
}

func forecastState(state simulation.DailyLaborState, snapshot port.HouseholdSnapshot, days int) (StockProjection, int64, *calendar.Moment, error) {
	start, err := calendar.MomentAtClock(calendar.ClockState{Day: calendar.GameDay(snapshot.CurrentGameDay), Remainder: snapshot.CalendarRemainder, GameDaysPerTickNum: snapshot.GameDaysPerTickNum, GameDaysPerTickDen: snapshot.GameDaysPerTickDen})
	if err != nil {
		return StockProjection{}, 0, nil, err
	}
	minimum := state.ProvisionsMilli
	var first *calendar.Moment
	for hour := 0; hour < days*24; hour++ {
		begin := calendar.AdvanceMoment(start, hour)
		nextTick := state.Tick + 1
		for _, shipment := range snapshot.IncomingShipments {
			if shipment.Status != "in_transit" || shipment.ExpectedArrivalTick != nextTick {
				continue
			}
			switch shipment.ResourceType {
			case "provisions":
				state.ProvisionsMilli += shipment.QuantityMilli
			case "wood":
				state.WoodMilli += shipment.QuantityMilli
			case "trade_goods":
				state.TradeGoodsMilli += shipment.QuantityMilli
			case "silver":
				state.SilverMilli += shipment.QuantityMilli
			}
		}
		temporal, err := temporalContext(snapshot.SimulationModel, snapshot.CalendarAnchorAt, snapshot.WorldUTCOffsetMinutes, begin)
		if err != nil {
			return StockProjection{}, 0, nil, err
		}
		cfg := balance.DailyLaborV1()
		if snapshot.SimulationModel == port.ModelMonthlySeasons {
			cfg = balance.MonthlySeasonsV1()
		}
		result, err := simulation.ProcessHourWithContext(state, simulation.HourInterval{Start: begin, End: calendar.AdvanceMoment(begin, 1)}, temporal.Context, cfg)
		if err != nil {
			return StockProjection{}, 0, nil, err
		}
		state = result.State
		if state.ProvisionsMilli < minimum {
			minimum = state.ProvisionsMilli
		}
		if result.FoodShortageMilli > 0 && first == nil {
			value := begin
			first = &value
		}
	}
	return StockProjection{ProvisionsMilli: state.ProvisionsMilli, WoodMilli: state.WoodMilli, PendingProvisionsMilli: state.PendingProvisionsMilli, PendingWoodMilli: state.PendingWoodMilli}, minimum, first, nil
}

func cloneDailyState(input simulation.DailyLaborState) simulation.DailyLaborState {
	out := input
	out.ProductionRemainders = map[string]int64{}
	for k, v := range input.ProductionRemainders {
		out.ProductionRemainders[k] = v
	}
	out.FatigueRemainders = map[string]int64{}
	for k, v := range input.FatigueRemainders {
		out.FatigueRemainders[k] = v
	}
	out.Characters = append([]simulation.DailyCharacter(nil), input.Characters...)
	for i := range out.Characters {
		out.Characters[i].TemporaryDuties = append([]workdomain.TemporaryDuty(nil), input.Characters[i].TemporaryDuties...)
		if input.Characters[i].Occupation.PendingActivity != nil {
			value := *input.Characters[i].Occupation.PendingActivity
			out.Characters[i].Occupation.PendingActivity = &value
		}
		if input.Characters[i].Occupation.EffectiveDay != nil {
			value := *input.Characters[i].Occupation.EffectiveDay
			out.Characters[i].Occupation.EffectiveDay = &value
		}
		if input.Characters[i].Occupation.EffectiveHour != nil {
			value := *input.Characters[i].Occupation.EffectiveHour
			out.Characters[i].Occupation.EffectiveHour = &value
		}
	}
	if input.LastSettlementDay != nil {
		value := *input.LastSettlementDay
		out.LastSettlementDay = &value
	}
	return out
}

func forecastRevision(state simulation.DailyLaborState) int64 {
	var revision int64
	for _, c := range state.Characters {
		if c.Occupation.Revision > revision {
			revision = c.Occupation.Revision
		}
	}
	return revision
}
