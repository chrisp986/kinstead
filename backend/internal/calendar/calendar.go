package calendar

import (
	"errors"
	"math/big"
)

const (
	DaysPerYear   int64 = 364
	DaysPerWeek   int64 = 7
	WeeksPerYear  int64 = 52
	DaysPerSeason int64 = 91
)

var ErrInvalidClock = errors.New("invalid historical clock")
var ErrArithmeticOverflow = errors.New("calendar arithmetic overflow")

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
	return AdvanceTicks(day, remainder, numerator, denominator, 1)
}

// AdvanceTicks applies ticks steps of a rational game-day clock. The
// remainder is carried between calls; wall-clock tick duration is deliberately
// not an input because it only controls when a worker invokes this function.
func AdvanceTicks(day GameDay, remainder, numerator, denominator, ticks int64) (GameDay, int64, error) {
	if remainder < 0 || numerator <= 0 || denominator <= 0 || remainder >= denominator || ticks < 0 {
		return 0, 0, ErrInvalidClock
	}
	total := new(big.Int).SetInt64(remainder)
	total.Add(total, new(big.Int).Mul(new(big.Int).SetInt64(numerator), new(big.Int).SetInt64(ticks)))
	delta, nextRemainder := new(big.Int), new(big.Int)
	delta.QuoRem(total, new(big.Int).SetInt64(denominator), nextRemainder)
	result := new(big.Int).Add(new(big.Int).SetInt64(int64(day)), delta)
	if !result.IsInt64() || !nextRemainder.IsInt64() {
		return 0, 0, ErrInvalidClock
	}
	return GameDay(result.Int64()), nextRemainder.Int64(), nil
}

// GameDayAfterTicks returns only the absolute day after ticks execution
// steps. It is useful for projecting an execution deadline without duplicating
// the rational-clock formula in application or persistence code.
func GameDayAfterTicks(day GameDay, remainder, numerator, denominator, ticks int64) (GameDay, error) {
	result, _, err := AdvanceTicks(day, remainder, numerator, denominator, ticks)
	return result, err
}

// GameDayAtTick projects a signed execution-tick offset from a known clock
// state. The returned day is the floor of the rational position and is useful
// for read projections of both queued and recently completed work.
func GameDayAtTick(day GameDay, remainder, numerator, denominator, tickOffset int64) (GameDay, error) {
	if remainder < 0 || numerator <= 0 || denominator <= 0 || remainder >= denominator {
		return 0, ErrInvalidClock
	}
	position := new(big.Int).Mul(new(big.Int).SetInt64(int64(day)), new(big.Int).SetInt64(denominator))
	position.Add(position, new(big.Int).SetInt64(remainder))
	position.Add(position, new(big.Int).Mul(new(big.Int).SetInt64(numerator), new(big.Int).SetInt64(tickOffset)))
	projected, residual := new(big.Int), new(big.Int)
	projected.QuoRem(position, new(big.Int).SetInt64(denominator), residual)
	if residual.Sign() < 0 {
		projected.Sub(projected, big.NewInt(1))
	}
	if !projected.IsInt64() {
		return 0, ErrInvalidClock
	}
	return GameDay(projected.Int64()), nil
}

// CeilDaysForTicks converts a positive execution-tick travel duration into
// whole game days using the world's rational calendar pacing. It deliberately
// uses integer arithmetic and rejects values that cannot be represented.
func CeilDaysForTicks(ticks, numerator, denominator int64) (int64, error) {
	if ticks < 0 || numerator <= 0 || denominator <= 0 {
		return 0, ErrInvalidClock
	}
	if ticks == 0 {
		return 0, nil
	}
	value := new(big.Int).Mul(new(big.Int).SetInt64(ticks), new(big.Int).SetInt64(numerator))
	value.Add(value, new(big.Int).Sub(new(big.Int).SetInt64(denominator), big.NewInt(1)))
	value.Quo(value, new(big.Int).SetInt64(denominator))
	if !value.IsInt64() {
		return 0, ErrArithmeticOverflow
	}
	return value.Int64(), nil
}

// LatestDispatchGameDay projects the exact dispatch deadline from the world's
// current clock state. CeilDaysForTicks is suitable for duration display, but
// cannot determine an exact deadline because the current remainder matters.
func LatestDispatchGameDay(
	currentDay GameDay,
	remainder int64,
	numerator int64,
	denominator int64,
	dueGameDay GameDay,
	travelTicks int64,
) (GameDay, error) {
	if numerator <= 0 || denominator <= 0 || remainder < 0 || remainder >= denominator || travelTicks < 0 {
		return 0, ErrInvalidClock
	}

	denominatorBig := big.NewInt(denominator)
	currentPosition := new(big.Int).Mul(big.NewInt(int64(currentDay)), denominatorBig)
	currentPosition.Add(currentPosition, big.NewInt(remainder))

	arrivalBoundary := new(big.Int).Add(big.NewInt(int64(dueGameDay)), big.NewInt(1))
	arrivalBoundary.Mul(arrivalBoundary, denominatorBig)
	arrivalBoundary.Sub(arrivalBoundary, big.NewInt(1))
	arrivalBoundary.Sub(arrivalBoundary, currentPosition)

	latestArrivalOffset := floorDivBigInt(arrivalBoundary, big.NewInt(numerator))
	latestDispatchOffset := new(big.Int).Sub(latestArrivalOffset, big.NewInt(travelTicks))
	if !latestDispatchOffset.IsInt64() {
		return 0, ErrArithmeticOverflow
	}

	return GameDayAtTick(currentDay, remainder, numerator, denominator, latestDispatchOffset.Int64())
}

func floorDivBigInt(numerator, denominator *big.Int) *big.Int {
	quotient := new(big.Int)
	remainder := new(big.Int)
	quotient.QuoRem(numerator, denominator, remainder)
	if remainder.Sign() < 0 {
		quotient.Sub(quotient, big.NewInt(1))
	}
	return quotient
}

// SubtractInt64 subtracts two signed integers without allowing wraparound.
func SubtractInt64(a, b int64) (int64, error) {
	result := new(big.Int).Sub(new(big.Int).SetInt64(a), new(big.Int).SetInt64(b))
	if !result.IsInt64() {
		return 0, ErrArithmeticOverflow
	}
	return result.Int64(), nil
}

// TicksUntilGameDay returns the smallest non-negative number of execution
// ticks whose end position reaches target. A target at or before day needs no
// execution steps. The calculation is integer-only and checks int64 bounds.
func TicksUntilGameDay(day GameDay, remainder, numerator, denominator int64, target GameDay) (int64, error) {
	if remainder < 0 || numerator <= 0 || denominator <= 0 || remainder >= denominator {
		return 0, ErrInvalidClock
	}
	if target <= day {
		return 0, nil
	}

	neededDays := new(big.Int).Sub(new(big.Int).SetInt64(int64(target)), new(big.Int).SetInt64(int64(day)))
	needed := new(big.Int).Mul(neededDays, new(big.Int).SetInt64(denominator))
	needed.Sub(needed, new(big.Int).SetInt64(remainder))
	if needed.Sign() <= 0 {
		return 0, nil
	}
	needed.Add(needed, new(big.Int).SetInt64(numerator-1))
	ticks := needed.Quo(needed, new(big.Int).SetInt64(numerator))
	if !ticks.IsInt64() {
		return 0, ErrInvalidClock
	}
	return ticks.Int64(), nil
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
