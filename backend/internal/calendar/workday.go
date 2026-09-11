package calendar

import (
	"fmt"
	"time"
)

type DaylightAnchor struct {
	SunriseHour int
	SunsetHour  int
}

type DaylightConfig struct {
	Anchors                map[ProductionSeason]DaylightAnchor
	PreparationHours       int
	MaximumNormalWorkHours int
}

type Workday struct {
	SunriseHour int `json:"sunrise_hour"`
	SunsetHour  int `json:"sunset_hour"`
	StartHour   int `json:"start_hour"`
	EndHour     int `json:"end_hour"`
}

func DefaultDaylightConfig() DaylightConfig {
	return DaylightConfig{
		Anchors: map[ProductionSeason]DaylightAnchor{
			Spring: {SunriseHour: 8, SunsetHour: 18},
			Summer: {SunriseHour: 5, SunsetHour: 21},
			Autumn: {SunriseHour: 6, SunsetHour: 18},
			Winter: {SunriseHour: 9, SunsetHour: 15},
		},
		PreparationHours: 1, MaximumNormalWorkHours: 8,
	}
}

// WorkdayForDate linearly interpolates whole-hour gameplay daylight from the
// current scheduling month toward the next month's anchor. The working
// interval is [StartHour, EndHour); an interval beginning at EndHour recovers.
func WorkdayForDate(date time.Time, cfg DaylightConfig) (Workday, error) {
	position, err := SeasonalPositionForDate(date)
	if err != nil || cfg.PreparationHours < 0 || cfg.MaximumNormalWorkHours <= 0 {
		return Workday{}, ErrInvalidClock
	}
	current, ok := cfg.Anchors[position.Season]
	if !ok {
		return Workday{}, fmt.Errorf("%w: missing daylight anchor for %s", ErrInvalidClock, position.Season)
	}
	nextDate := position.NextBoundary
	nextSeason, err := SeasonForMonth(nextDate.Month())
	if err != nil {
		return Workday{}, err
	}
	next, ok := cfg.Anchors[nextSeason]
	if !ok {
		return Workday{}, fmt.Errorf("%w: missing daylight anchor for %s", ErrInvalidClock, nextSeason)
	}
	dayIndex := position.Day - 1
	sunrise := interpolateHour(current.SunriseHour, next.SunriseHour, dayIndex, position.LengthDays)
	sunset := interpolateHour(current.SunsetHour, next.SunsetHour, dayIndex, position.LengthDays)
	if sunrise < 0 || sunrise > 23 || sunset < 0 || sunset > 24 || sunrise > sunset {
		return Workday{}, ErrInvalidClock
	}
	start := sunrise + cfg.PreparationHours
	if start > sunset {
		start = sunset
	}
	end := start + cfg.MaximumNormalWorkHours
	if end > sunset {
		end = sunset
	}
	if end < start {
		end = start
	}
	return Workday{SunriseHour: sunrise, SunsetHour: sunset, StartHour: start, EndHour: end}, nil
}

func (w Workday) IsWorkingHour(hour int) bool {
	return hour >= w.StartHour && hour < w.EndHour
}

// NextWorkStartForWorld uses the command's committed world moment. At an
// already-committed start boundary (now.Hour == StartHour), the next boundary
// is tomorrow; this prevents a command racing that tick from changing output
// that has already been decided.
func NextWorkStartForWorld(anchor time.Time, offsetMinutes int, now Moment, cfg DaylightConfig) (Moment, error) {
	today, err := SchedulingDate(anchor, offsetMinutes, now)
	if err != nil {
		return Moment{}, err
	}
	todayWorkday, err := WorkdayForDate(today, cfg)
	if err != nil {
		return Moment{}, err
	}
	if now.Hour < todayWorkday.StartHour {
		return Moment{Day: now.Day, Hour: todayWorkday.StartHour}, nil
	}
	tomorrowMoment := Moment{Day: now.Day + 1, Hour: 0}
	tomorrow, err := SchedulingDate(anchor, offsetMinutes, tomorrowMoment)
	if err != nil {
		return Moment{}, err
	}
	tomorrowWorkday, err := WorkdayForDate(tomorrow, cfg)
	if err != nil {
		return Moment{}, err
	}
	return Moment{Day: tomorrowMoment.Day, Hour: tomorrowWorkday.StartHour}, nil
}

func interpolateHour(from, to, numerator, denominator int) int {
	if denominator <= 0 {
		return from
	}
	delta := (to - from) * numerator
	if delta >= 0 {
		return from + (delta+denominator/2)/denominator
	}
	return from - ((-delta)+denominator/2)/denominator
}
