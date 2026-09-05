package application

import "game/backend/internal/calendar"

func gameDayAfterTicks(day, remainder, numerator, denominator, ticks int64) (int64, error) {
	value, err := calendar.GameDayAfterTicks(calendar.GameDay(day), remainder, numerator, denominator, ticks)
	if err != nil {
		return 0, err
	}
	return int64(value), nil
}
