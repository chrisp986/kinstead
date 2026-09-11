package calendar

import (
	"fmt"
	"time"
)

// SeasonForMonth maps the world scheduling-calendar month to a game season.
// The four-season cycle repeats three times per real calendar year; it does
// not affect the separate historical age calendar.
func SeasonForMonth(month time.Month) (ProductionSeason, error) {
	if month < time.January || month > time.December {
		return "", ErrInvalidClock
	}
	seasons := [...]ProductionSeason{Spring, Summer, Autumn, Winter}
	return seasons[(int(month)-1)%4], nil
}

type SeasonalPosition struct {
	Season       ProductionSeason `json:"season"`
	Day          int              `json:"season_day"` // one-based within the current scheduling month
	LengthDays   int              `json:"season_length_days"`
	Cycle        int              `json:"seasonal_cycle"` // one-based cycle within the real calendar year
	NextBoundary time.Time        `json:"-"`
}

// FixedZone returns the world-owned scheduling zone. A fixed offset keeps all
// game days exactly 24 hours even when a player's local timezone observes DST.
func FixedZone(offsetMinutes int) (*time.Location, error) {
	if offsetMinutes < -14*60 || offsetMinutes > 14*60 {
		return nil, ErrInvalidClock
	}
	return time.FixedZone(fmt.Sprintf("world%+03d:%02d", offsetMinutes/60, abs(offsetMinutes%60)), offsetMinutes*60), nil
}

// SchedulingDate maps an authoritative game moment onto the persisted modern
// scheduling calendar. Historical labels and character ages remain separate.
func SchedulingDate(anchor time.Time, offsetMinutes int, moment Moment) (time.Time, error) {
	if anchor.IsZero() || moment.Day < 0 || moment.Hour < 0 || moment.Hour > 23 {
		return time.Time{}, ErrInvalidClock
	}
	zone, err := FixedZone(offsetMinutes)
	if err != nil {
		return time.Time{}, err
	}
	localAnchor := anchor.In(zone)
	if localAnchor.Hour() != 0 || localAnchor.Minute() != 0 || localAnchor.Second() != 0 || localAnchor.Nanosecond() != 0 {
		return time.Time{}, fmt.Errorf("%w: calendar anchor must be midnight in world time", ErrInvalidClock)
	}
	return localAnchor.AddDate(0, 0, int(moment.Day)).Add(time.Duration(moment.Hour) * time.Hour), nil
}

func SeasonalPositionForDate(date time.Time) (SeasonalPosition, error) {
	season, err := SeasonForMonth(date.Month())
	if err != nil || date.IsZero() {
		return SeasonalPosition{}, ErrInvalidClock
	}
	year, month, day := date.Date()
	startNext := time.Date(year, month+1, 1, 0, 0, 0, 0, date.Location())
	length := startNext.AddDate(0, 0, -1).Day()
	return SeasonalPosition{Season: season, Day: day, LengthDays: length, Cycle: (int(month)-1)/4 + 1, NextBoundary: startNext}, nil
}

func NextSeasonBoundary(date time.Time) (time.Time, error) {
	position, err := SeasonalPositionForDate(date)
	if err != nil {
		return time.Time{}, err
	}
	return position.NextBoundary, nil
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
