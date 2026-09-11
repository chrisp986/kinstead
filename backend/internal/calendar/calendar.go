package calendar

import (
	"errors"
	"fmt"
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

// CalendarDefinition contains the rules that turn the absolute game-day
// clock into presentation and seasonal boundaries. Legacy worlds retain the
// 364-day definition; daily-labor worlds use the 365-day definition.
type CalendarDefinition struct {
	DaysPerYear   int64
	DaysPerWeek   int64
	DaysPerSeason int64
	SpringEnd     int64
	SummerEnd     int64
	AutumnEnd     int64
	HalfYearStart int64
}

var LegacyDefinition = CalendarDefinition{
	DaysPerYear: 364, DaysPerWeek: 7, DaysPerSeason: 91,
	SpringEnd: 91, SummerEnd: 182, AutumnEnd: 273, HalfYearStart: 182,
}

var DailyLaborDefinition = CalendarDefinition{
	DaysPerYear: 365, DaysPerWeek: 7, DaysPerSeason: 0,
	SpringEnd: 91, SummerEnd: 183, AutumnEnd: 274, HalfYearStart: 183,
}

// DefinitionForModel returns the historical/aging calendar for a persisted
// model. Monthly seasons use a real scheduling calendar for work but retain
// the 365-day historical age convention. Unknown identifiers are errors so a
// typo can never silently execute legacy rules.
func DefinitionForModel(model string) (CalendarDefinition, error) {
	switch model {
	case "legacy":
		return LegacyDefinition, nil
	case "daily_labor_v1", "monthly_seasons_v1":
		return DailyLaborDefinition, nil
	default:
		return CalendarDefinition{}, fmt.Errorf("%w: unknown simulation model %q", ErrInvalidClock, model)
	}
}

func (d CalendarDefinition) DayOfYear(day GameDay) int64 {
	return floorDivModValue(int64(day), d.DaysPerYear)
}

func (d CalendarDefinition) YearIndex(day GameDay) int64 {
	return floorDiv(int64(day), d.DaysPerYear)
}

func (d CalendarDefinition) ProductionSeasonAt(day GameDay) ProductionSeason {
	doy := d.DayOfYear(day)
	switch {
	case doy < d.SpringEnd:
		return Spring
	case doy < d.SummerEnd:
		return Summer
	case doy < d.AutumnEnd:
		return Autumn
	default:
		return Winter
	}
}

func (d CalendarDefinition) HalfYearAt(day GameDay) HalfYear {
	if d.DayOfYear(day) < d.HalfYearStart {
		return SummerHalf
	}
	return WinterHalf
}

func (d CalendarDefinition) SeasonalPhaseAt(day GameDay) SeasonalPhase {
	doy := d.DayOfYear(day)
	// Phases intentionally remain cultural markers rather than equal-sized
	// seasons. The daily calendar shifts the autumn/winter boundary by one day.
	switch {
	case doy >= d.SpringEnd && doy < d.SpringEnd+30:
		return EarlySummer
	case doy >= d.SpringEnd+30 && doy < d.SpringEnd+61:
		return HighSummer
	case doy >= d.SpringEnd+61 && doy < d.SummerEnd:
		return LateSummer
	case doy >= d.AutumnEnd && doy < d.AutumnEnd+30:
		return EarlyWinter
	case doy >= d.AutumnEnd+30 && doy < d.AutumnEnd+60:
		return MidWinter
	case doy >= d.AutumnEnd+60:
		return LateWinter
	default:
		return SeasonalPhase("")
	}
}

func (d CalendarDefinition) Breakdown(day GameDay) Date {
	doy := d.DayOfYear(day)
	halfDay := d.HalfYearStart
	weekOfHalf := doy / d.DaysPerWeek
	if doy >= halfDay {
		weekOfHalf = (doy - halfDay) / d.DaysPerWeek
	}
	phase := d.SeasonalPhaseAt(day)
	_, dayOfWeek := floorDivMod(int64(day), d.DaysPerWeek)
	return Date{
		GameDay: day, YearIndex: d.YearIndex(day), DayOfYear: doy,
		WeekOfYear: doy/d.DaysPerWeek + 1, WeekOfHalf: weekOfHalf + 1,
		DayOfWeek:        dayOfWeek + 1,
		ProductionSeason: d.ProductionSeasonAt(day), HalfYear: d.HalfYearAt(day),
		SeasonalPhase: phase, Phase: phase,
	}
}

func (d CalendarDefinition) Age(birth, on GameDay) (int, error) {
	if on < birth {
		return 0, ErrInvalidClock
	}
	years := d.YearIndex(on) - d.YearIndex(birth)
	if d.DayOfYear(on) < d.DayOfYear(birth) {
		years--
	}
	return int(years), nil
}

func (d CalendarDefinition) StartOfNextHalfYear(day GameDay) GameDay {
	start := GameDay(d.YearIndex(day) * d.DaysPerYear)
	if d.DayOfYear(day) < d.HalfYearStart {
		return start + GameDay(d.HalfYearStart)
	}
	return start + GameDay(d.DaysPerYear)
}

func (d CalendarDefinition) StartOfNextProductionSeason(day GameDay) GameDay {
	start := GameDay(d.YearIndex(day) * d.DaysPerYear)
	doy := d.DayOfYear(day)
	for _, boundary := range []int64{0, d.SpringEnd, d.SummerEnd, d.AutumnEnd, d.DaysPerYear} {
		if boundary > doy {
			return start + GameDay(boundary)
		}
	}
	return start + GameDay(d.DaysPerYear)
}

// ClockState is the persisted rational game-time clock. For daily-labor
// worlds one remainder unit is one hour (1/24 game days).
type ClockState struct {
	Day                GameDay
	Remainder          int64
	GameDaysPerTickNum int64
	GameDaysPerTickDen int64
}

type Moment struct {
	Day  GameDay `json:"day"`
	Hour int     `json:"hour"`
}

func HourOfDay(clock ClockState) (int, error) {
	if clock.GameDaysPerTickNum <= 0 || clock.GameDaysPerTickDen <= 0 ||
		clock.Remainder < 0 || clock.Remainder >= clock.GameDaysPerTickDen {
		return 0, ErrInvalidClock
	}
	// The daily model is exactly one hour per remainder unit. The generic
	// conversion keeps this helper useful for projections of other rational
	// clocks without making the hour depend on tick modulo arithmetic.
	hour := clock.Remainder * 24 / clock.GameDaysPerTickDen
	if hour < 0 || hour > 23 {
		return 0, ErrInvalidClock
	}
	return int(hour), nil
}

func MomentAtClock(clock ClockState) (Moment, error) {
	hour, err := HourOfDay(clock)
	if err != nil {
		return Moment{}, err
	}
	return Moment{Day: clock.Day, Hour: hour}, nil
}

func IsWorkingHour(hour int) bool { return hour >= 8 && hour < 17 }

func NextWorkStart(clock ClockState) (Moment, error) {
	now, err := MomentAtClock(clock)
	if err != nil {
		return Moment{}, err
	}
	if now.Hour < 8 {
		return Moment{Day: now.Day, Hour: 8}, nil
	}
	return Moment{Day: now.Day + 1, Hour: 8}, nil
}

func CrossesSettlement(start, end Moment) bool {
	return start.Day == end.Day && start.Hour < 17 && end.Hour >= 17
}

func AdvanceMoment(moment Moment, hours int) Moment {
	if hours < 0 {
		return moment
	}
	total := moment.Hour + hours
	return Moment{Day: moment.Day + GameDay(total/24), Hour: total % 24}
}

// ShiftMoment moves across whole-hour boundaries in either direction. It is
// used when an already-running inclusive tick range is projected into the
// end-exclusive [start,end) moment convention.
func ShiftMoment(moment Moment, hours int64) (Moment, error) {
	if moment.Hour < 0 || moment.Hour > 23 {
		return Moment{}, ErrInvalidClock
	}
	totalValue := new(big.Int).Mul(big.NewInt(int64(moment.Day)), big.NewInt(24))
	totalValue.Add(totalValue, big.NewInt(int64(moment.Hour)))
	totalValue.Add(totalValue, big.NewInt(hours))
	if !totalValue.IsInt64() {
		return Moment{}, ErrArithmeticOverflow
	}
	total := totalValue.Int64()
	day, hour := floorDivMod(total, 24)
	return Moment{Day: GameDay(day), Hour: int(hour)}, nil
}

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
