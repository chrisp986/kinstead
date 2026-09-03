package calendar

import (
	"errors"
	"math"
	"time"
)

var ErrInvalidClock = errors.New("invalid historical clock")

// Clock maps sequential simulation ticks onto the historical calendar.
// Tick 0 is the start-date snapshot. Tick N ends floor(N*num/den) days
// after StartDate; the remainder is intentionally carried by the formula.
type Clock struct {
	StartDate   time.Time
	DaysPerTick Rational
}

type Rational struct {
	Numerator   int64
	Denominator int64
}

func (c Clock) DateAtTick(tick int64) (time.Time, error) {
	if c.StartDate.IsZero() || c.DaysPerTick.Numerator <= 0 || c.DaysPerTick.Denominator <= 0 || tick < 0 {
		return time.Time{}, ErrInvalidClock
	}
	if tick > math.MaxInt64/c.DaysPerTick.Numerator {
		return time.Time{}, ErrInvalidClock
	}
	days := tick * c.DaysPerTick.Numerator / c.DaysPerTick.Denominator
	return dateOnly(c.StartDate).AddDate(0, 0, int(days)), nil
}

// AgeOn returns completed historical years, respecting whether the birthday
// has occurred in the year. Dates before birth are rejected.
func AgeOn(birthDate, onDate time.Time) (int, error) {
	birthDate = dateOnly(birthDate)
	onDate = dateOnly(onDate)
	if birthDate.IsZero() || onDate.Before(birthDate) {
		return 0, ErrInvalidClock
	}
	age := onDate.Year() - birthDate.Year()
	anniversary := birthDate.AddDate(age, 0, 0)
	if anniversary.After(onDate) {
		age--
	}
	return age, nil
}

func dateOnly(value time.Time) time.Time {
	y, m, d := value.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
