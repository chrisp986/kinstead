package calendar

import (
	"errors"
	"math"
	"time"
)

const (
	DaysPerYear   int64 = 364
	DaysPerWeek   int64 = 7
	WeeksPerYear  int64 = 52
	DaysPerSeason int64 = 91
)

var ErrInvalidClock = errors.New("invalid historical clock")

type GameDay int64

type Date struct {
	GameDay          GameDay          `json:"game_day"`
	YearIndex        int64            `json:"year_index"`
	DayOfYear        int64            `json:"day_of_year"`
	WeekOfYear       int64            `json:"week_of_year"`
	WeekOfHalf       int64            `json:"week_of_half"`
	DayOfWeek        int64            `json:"day_of_week"`
	ProductionSeason ProductionSeason `json:"production_season"`
	HalfYear         HalfYear         `json:"half_year"`
	SeasonalPhase    SeasonalPhase    `json:"seasonal_phase"`
	Phase            SeasonalPhase    `json:"phase"`
}

type ProductionSeason string

const (
	Spring ProductionSeason = "spring"
	Summer ProductionSeason = "summer"
	Autumn ProductionSeason = "autumn"
	Winter ProductionSeason = "winter"
)

type HalfYear string

const (
	SummerHalf HalfYear = "summer"
	WinterHalf HalfYear = "winter"
)

type SeasonalPhase string

const (
	EarlySummer SeasonalPhase = "early_summer"
	HighSummer  SeasonalPhase = "high_summer"
	LateSummer  SeasonalPhase = "late_summer"
	EarlyWinter SeasonalPhase = "early_winter"
	MidWinter   SeasonalPhase = "midwinter"
	LateWinter  SeasonalPhase = "late_winter"
)

func Breakdown(day GameDay) Date {
	year, dayOfYear := floorDivMod(int64(day), DaysPerYear)
	weekOfHalf := dayOfYear / DaysPerWeek
	if dayOfYear >= 182 {
		weekOfHalf = (dayOfYear - 182) / DaysPerWeek
	}
	phase := SeasonalPhaseAt(day)
	return Date{
		GameDay: day, YearIndex: year, DayOfYear: dayOfYear,
		WeekOfYear: dayOfYear/DaysPerWeek + 1, WeekOfHalf: weekOfHalf + 1,
		DayOfWeek:        dayOfYear%DaysPerWeek + 1,
		ProductionSeason: ProductionSeasonAt(day), HalfYear: HalfYearAt(day),
		SeasonalPhase: phase, Phase: phase,
	}
}

// BreakdownOf is retained as a descriptive alias for existing callers.
func BreakdownOf(day GameDay) Date { return Breakdown(day) }

func WeekOfHalf(day GameDay) int64 { return Breakdown(day).WeekOfHalf }

func YearIndex(day GameDay) int64 { return floorDiv(int64(day), DaysPerYear) }

// DayOfYear is zero-based, in the range [0, 363].
func DayOfYear(day GameDay) int64 { return floorDivModValue(int64(day), DaysPerYear) }

// WeekOfYear is one-based, in the range [1, 52].
func WeekOfYear(day GameDay) int64 { return DayOfYear(day)/DaysPerWeek + 1 }

func ProductionSeasonAt(day GameDay) ProductionSeason {
	switch DayOfYear(day) / DaysPerSeason {
	case 0:
		return Spring
	case 1:
		return Summer
	case 2:
		return Autumn
	default:
		return Winter
	}
}

func HalfYearAt(day GameDay) HalfYear {
	if DayOfYear(day) < 182 {
		return SummerHalf
	}
	return WinterHalf
}

func SeasonalPhaseAt(day GameDay) SeasonalPhase {
	dayOfYear := DayOfYear(day)
	switch {
	case dayOfYear >= 91 && dayOfYear < 121:
		return EarlySummer
	case dayOfYear >= 121 && dayOfYear < 152:
		return HighSummer
	case dayOfYear >= 152 && dayOfYear < 182:
		return LateSummer
	case dayOfYear >= 273 && dayOfYear < 304:
		return EarlyWinter
	case dayOfYear >= 304 && dayOfYear < 334:
		return MidWinter
	case dayOfYear >= 334:
		return LateWinter
	default:
		return SeasonalPhase("")
	}
}

func DaysUntil(from, target GameDay) int64 { return int64(target - from) }

// Advance applies one rational game-day step and returns the new absolute day
// and remainder. The remainder is part of authoritative world state so that
// repeated ticks are deterministic and no fractional days are lost.
func Advance(day GameDay, remainder, numerator, denominator int64) (GameDay, int64, error) {
	if remainder < 0 || numerator <= 0 || denominator <= 0 || remainder >= denominator ||
		int64(day) > math.MaxInt64-numerator || numerator > math.MaxInt64-remainder {
		return 0, 0, ErrInvalidClock
	}
	total := remainder + numerator
	return day + GameDay(total/denominator), total % denominator, nil
}

// Age returns completed 364-day calendar years. Birth days before GameDay
// zero are valid, which allows the initial world snapshot to contain adults.
func Age(birth, on GameDay) (int, error) {
	if on < birth {
		return 0, ErrInvalidClock
	}
	years := YearIndex(on) - YearIndex(birth)
	if DayOfYear(on) < DayOfYear(birth) {
		years--
	}
	return int(years), nil
}

func StartOfNextHalfYear(day GameDay) GameDay {
	start := GameDay(YearIndex(day) * DaysPerYear)
	if DayOfYear(day) < 182 {
		return start + 182
	}
	return start + GameDay(DaysPerYear)
}

func StartOfNextProductionSeason(day GameDay) GameDay {
	start := GameDay(YearIndex(day) * DaysPerYear)
	dayOfYear := DayOfYear(day)
	seasonStart := (dayOfYear / DaysPerSeason) * DaysPerSeason
	if dayOfYear < seasonStart+DaysPerSeason {
		return start + GameDay(seasonStart+DaysPerSeason)
	}
	return start + GameDay(DaysPerYear)
}

func floorDiv(value, divisor int64) int64 {
	quotient := value / divisor
	if value%divisor < 0 {
		quotient--
	}
	return quotient
}

func floorDivMod(value, divisor int64) (int64, int64) {
	quotient := floorDiv(value, divisor)
	return quotient, value - quotient*divisor
}

func floorDivModValue(value, divisor int64) int64 {
	_, remainder := floorDivMod(value, divisor)
	return remainder
}

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
