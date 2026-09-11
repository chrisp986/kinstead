package dailylabor

import (
	"fmt"
	"time"

	"game/backend/internal/balance"
	"game/backend/internal/calendar"
	workdomain "game/backend/internal/domain/work"
	"game/backend/internal/simulation"
)

type Summary struct {
	Days              int
	FoodShortageHours int
	MinimumFoodMilli  int64
	MinimumWoodMilli  int64
	MaximumFatigue    int
	EndingFoodMilli   int64
	EndingWoodMilli   int64
}

// RunStartingHousehold runs complete real-calendar years without commands.
// It is a baseline validation, not a claim that all future player strategies
// are balanced; trade and relationships remain meaningful decisions.
func RunStartingHousehold(years int) (Summary, error) {
	if years <= 0 {
		return Summary{}, fmt.Errorf("years must be positive")
	}
	state := simulation.DailyLaborState{CurrentGameDay: 0, ProvisionsMilli: 100_000, WoodMilli: 60_000, FarmSpecialization: simulation.Fishing}
	state.Characters = []simulation.DailyCharacter{
		{ID: "bjorn", Name: "Bjorn", BirthGameDay: -32 * 365, LaborPermille: 1000, Status: "active", Specialization: simulation.Agriculture, Occupation: workdomain.Occupation{CharacterID: "bjorn", Activity: workdomain.Agriculture, Revision: 1}},
		{ID: "astrid", Name: "Astrid", BirthGameDay: -29 * 365, LaborPermille: 1000, Status: "active", Specialization: simulation.Fishing, Occupation: workdomain.Occupation{CharacterID: "astrid", Activity: workdomain.Fishing, Revision: 1}},
		{ID: "einar", Name: "Einar", BirthGameDay: -17 * 365, LaborPermille: 1000, Status: "active", Occupation: workdomain.Occupation{CharacterID: "einar", Activity: workdomain.Woodcutting, Revision: 1}},
		{ID: "ragnhild", Name: "Ragnhild", BirthGameDay: -12 * 365, LaborPermille: 500, Status: "active", Occupation: workdomain.Occupation{CharacterID: "ragnhild", Activity: workdomain.Fishing, Revision: 1}},
		{ID: "sven", Name: "Sven", BirthGameDay: -8 * 365, LaborPermille: 0, Status: "active", Occupation: workdomain.Occupation{CharacterID: "sven", Activity: workdomain.Agriculture, Revision: 1}},
	}
	cfg := balance.MonthlySeasonsV1()
	anchor := time.Date(2028, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := anchor.AddDate(years, 0, 0)
	totalHours := int(end.Sub(anchor) / time.Hour)
	summary := Summary{Days: totalHours / 24, MinimumFoodMilli: state.ProvisionsMilli, MinimumWoodMilli: state.WoodMilli}
	for hour := 0; hour < totalHours; hour++ {
		start := calendar.Moment{Day: calendar.GameDay(hour / 24), Hour: hour % 24}
		date := anchor.Add(time.Duration(hour) * time.Hour)
		position, err := calendar.SeasonalPositionForDate(date)
		if err != nil {
			return Summary{}, err
		}
		workday, err := calendar.WorkdayForDate(date, calendar.DefaultDaylightConfig())
		if err != nil {
			return Summary{}, err
		}
		result, err := simulation.ProcessHourWithContext(state, simulation.HourInterval{Start: start, End: calendar.AdvanceMoment(start, 1)}, simulation.WorkContext{Season: position.Season, Workday: workday}, cfg)
		if err != nil {
			return Summary{}, err
		}
		state = result.State
		if result.FoodShortageMilli > 0 {
			summary.FoodShortageHours++
		}
		if state.ProvisionsMilli < summary.MinimumFoodMilli {
			summary.MinimumFoodMilli = state.ProvisionsMilli
		}
		if state.WoodMilli < summary.MinimumWoodMilli {
			summary.MinimumWoodMilli = state.WoodMilli
		}
		for _, c := range state.Characters {
			if c.Fatigue > summary.MaximumFatigue {
				summary.MaximumFatigue = c.Fatigue
			}
		}
	}
	summary.EndingFoodMilli, summary.EndingWoodMilli = state.ProvisionsMilli, state.WoodMilli
	return summary, nil
}
