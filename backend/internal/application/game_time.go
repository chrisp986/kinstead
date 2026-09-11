package application

import (
	"fmt"
	"time"

	"game/backend/internal/calendar"
	"game/backend/internal/port"
	"game/backend/internal/simulation"
)

func gameDayAfterTicks(day, remainder, numerator, denominator, ticks int64) (int64, error) {
	value, err := calendar.GameDayAfterTicks(calendar.GameDay(day), remainder, numerator, denominator, ticks)
	if err != nil {
		return 0, err
	}
	return int64(value), nil
}

type temporalProjection struct {
	Context  simulation.WorkContext
	Date     time.Time
	Position calendar.SeasonalPosition
}

func temporalContext(model port.SimulationModel, anchor *time.Time, offset *int, moment calendar.Moment) (temporalProjection, error) {
	switch model {
	case port.ModelDailyLabor:
		return temporalProjection{Context: simulation.DailyLaborWorkContext(moment.Day)}, nil
	case port.ModelMonthlySeasons:
		if anchor == nil || offset == nil {
			return temporalProjection{}, fmt.Errorf("monthly season model requires a calendar anchor and world offset")
		}
		date, err := calendar.SchedulingDate(*anchor, *offset, moment)
		if err != nil {
			return temporalProjection{}, err
		}
		position, err := calendar.SeasonalPositionForDate(date)
		if err != nil {
			return temporalProjection{}, err
		}
		workday, err := calendar.WorkdayForDate(date, calendar.DefaultDaylightConfig())
		if err != nil {
			return temporalProjection{}, err
		}
		return temporalProjection{Context: simulation.WorkContext{Season: position.Season, Workday: workday}, Date: date, Position: position}, nil
	default:
		return temporalProjection{}, fmt.Errorf("%w: %q", ErrUnsupportedSimulationModel, model)
	}
}
