package calendar

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestDateAtTickCarriesRationalRemainder(t *testing.T) {
	clock := Clock{StartDate: time.Date(980, time.January, 1, 12, 0, 0, 0, time.FixedZone("test", 3600)), DaysPerTick: Rational{Numerator: 365, Denominator: 48}}
	cases := map[int64]string{0: "0980-01-01", 1: "0980-01-08", 12: "0980-04-01", 48: "0980-12-31"}
	for tick, want := range cases {
		got, err := clock.DateAtTick(tick)
		if err != nil {
			t.Fatal(err)
		}
		if formatted := got.Format(time.DateOnly); formatted != want {
			t.Fatalf("tick %d date = %s, want %s", tick, formatted, want)
		}
	}
}

func TestAgeOnUsesHistoricalBirthday(t *testing.T) {
	birth := time.Date(948, time.September, 10, 0, 0, 0, 0, time.UTC)
	before := time.Date(980, time.September, 9, 0, 0, 0, 0, time.UTC)
	on := time.Date(980, time.September, 10, 0, 0, 0, 0, time.UTC)
	if got, _ := AgeOn(birth, before); got != 31 {
		t.Fatalf("age before birthday = %d", got)
	}
	if got, _ := AgeOn(birth, on); got != 32 {
		t.Fatalf("age on birthday = %d", got)
	}
	if _, err := AgeOn(on, birth); !errors.Is(err, ErrInvalidClock) {
		t.Fatalf("date before birth error = %v", err)
	}
}

func TestGameDayBreakdownUses364DayYears(t *testing.T) {
	tests := []struct {
		day      GameDay
		year     int64
		dayYear  int64
		week     int64
		season   ProductionSeason
		halfYear HalfYear
	}{
		{0, 0, 0, 1, Spring, SummerHalf},
		{90, 0, 90, 13, Spring, SummerHalf},
		{91, 0, 91, 14, Summer, SummerHalf},
		{181, 0, 181, 26, Summer, SummerHalf},
		{182, 0, 182, 27, Autumn, WinterHalf},
		{273, 0, 273, 40, Winter, WinterHalf},
		{363, 0, 363, 52, Winter, WinterHalf},
		{364, 1, 0, 1, Spring, SummerHalf},
		{-1, -1, 363, 52, Winter, WinterHalf},
		{-364, -1, 0, 1, Spring, SummerHalf},
	}
	for _, tt := range tests {
		got := BreakdownOf(tt.day)
		if got.YearIndex != tt.year || got.DayOfYear != tt.dayYear || got.WeekOfYear != tt.week || got.ProductionSeason != tt.season || got.HalfYear != tt.halfYear {
			t.Errorf("day %d breakdown = %+v", tt.day, got)
		}
	}
}

func TestGameDayPhasesAndBoundaries(t *testing.T) {
	for _, tt := range []struct {
		day GameDay
		got SeasonalPhase
	}{
		{91, EarlySummer}, {120, EarlySummer}, {121, HighSummer},
		{151, HighSummer}, {152, LateSummer}, {181, LateSummer},
		{273, EarlyWinter}, {303, EarlyWinter}, {304, MidWinter},
		{333, MidWinter}, {334, LateWinter},
	} {
		if got := SeasonalPhaseAt(tt.day); got != tt.got {
			t.Errorf("day %d phase = %q, want %q", tt.day, got, tt.got)
		}
	}
	if got := StartOfNextHalfYear(181); got != 182 {
		t.Errorf("next half-year = %d, want 182", got)
	}
	if got := StartOfNextHalfYear(363); got != 364 {
		t.Errorf("next half-year = %d, want 364", got)
	}
	if got := StartOfNextProductionSeason(90); got != 91 {
		t.Errorf("next season = %d, want 91", got)
	}
	if got := StartOfNextProductionSeason(363); got != 364 {
		t.Errorf("next season = %d, want 364", got)
	}
}

func TestDefaultAnchorsUseStableGameCalendarDays(t *testing.T) {
	want := map[string]int64{
		"summer_start":  91,
		"midsummer":     121,
		"harvest_start": 152,
		"thing":         287,
		"winter_start":  273,
		"midwinter":     304,
		"jol":           320,
	}
	for _, rule := range DefaultAnchors() {
		if got := int64(AnchorGameDay(rule, 0)); got != want[rule.Code] {
			t.Errorf("anchor %s = %d, want %d", rule.Code, got, want[rule.Code])
		}
	}
}

func TestGameDayAgeAllowsBirthBeforeWorldStart(t *testing.T) {
	if got, err := Age(-364*32-10, 0); err != nil || got != 32 {
		t.Fatalf("age at world start = %d, %v; want 32", got, err)
	}
	if got, err := Age(-10, 9); err != nil || got != 0 {
		t.Fatalf("age before birthday = %d, %v; want 0", got, err)
	}
	if got, err := Age(-10, 10); err != nil || got != 0 {
		t.Fatalf("age on birthday = %d, %v; want 0", got, err)
	}
	if _, err := Age(2, 1); !errors.Is(err, ErrInvalidClock) {
		t.Fatalf("age before birth error = %v", err)
	}
}

func TestAdvancePreservesRemainderAndCompletesYear(t *testing.T) {
	day, remainder := GameDay(0), int64(0)
	for i := 0; i < 48; i++ {
		var err error
		day, remainder, err = Advance(day, remainder, 91, 12)
		if err != nil {
			t.Fatal(err)
		}
	}
	if day != 364 || remainder != 0 {
		t.Fatalf("48 ticks = day %d remainder %d, want 364/0", day, remainder)
	}
}

func TestDefaultClockBoundaries(t *testing.T) {
	day, remainder, err := AdvanceTicks(0, 0, 91, 12, 1)
	if err != nil || day != 7 || remainder != 7 {
		t.Fatalf("tick 1 = day %d remainder %d, err %v; want 7/7", day, remainder, err)
	}
	day, remainder, err = AdvanceTicks(0, 0, 91, 12, 11)
	if err != nil || day != 83 || remainder != 5 {
		t.Fatalf("tick 11 = day %d remainder %d, err %v; want 83/5", day, remainder, err)
	}
	day, remainder, err = AdvanceTicks(0, 0, 91, 12, 12)
	if err != nil || day != 91 || remainder != 0 {
		t.Fatalf("tick 12 = day %d remainder %d, err %v; want 91/0", day, remainder, err)
	}
	if got := ProductionSeasonAt(90); got != Spring {
		t.Fatalf("day 90 season = %s, want spring", got)
	}
	if got := ProductionSeasonAt(91); got != Summer {
		t.Fatalf("day 91 season = %s, want summer", got)
	}
	day, remainder, err = AdvanceTicks(0, 0, 91, 12, 48)
	if err != nil || day != 364 || remainder != 0 || ProductionSeasonAt(day) != Spring {
		t.Fatalf("tick 48 = day %d remainder %d season %s, err %v; want 364/0/spring", day, remainder, ProductionSeasonAt(day), err)
	}
}

func TestProductionSeasonUsesTheStartOfEachTickInterval(t *testing.T) {
	day, remainder := GameDay(0), int64(0)
	for tick := int64(1); tick <= 48; tick++ {
		want := Spring
		switch {
		case tick >= 13 && tick <= 24:
			want = Summer
		case tick >= 25 && tick <= 36:
			want = Autumn
		case tick >= 37:
			want = Winter
		}
		if got := ProductionSeasonAt(day); got != want {
			t.Fatalf("tick %d starts on day %d in %s, want %s", tick, day, got, want)
		}
		var err error
		day, remainder, err = Advance(day, remainder, 91, 12)
		if err != nil {
			t.Fatal(err)
		}
		if tick == 12 && day != 91 {
			t.Fatalf("after tick 12 day = %d, want 91", day)
		}
		if tick == 48 && (day != 364 || remainder != 0) {
			t.Fatalf("after tick 48 = day %d remainder %d, want 364/0", day, remainder)
		}
	}
}

func TestTickGameDayConversions(t *testing.T) {
	for _, tt := range []struct {
		name   string
		target GameDay
		want   int64
	}{
		{name: "zero", target: 0, want: 0},
		{name: "first day", target: 1, want: 1},
		{name: "spring boundary", target: 91, want: 12},
		{name: "next year", target: 364, want: 48},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TicksUntilGameDay(0, 0, 91, 12, tt.target)
			if err != nil || got != tt.want {
				t.Fatalf("ticks until %d = %d, err %v; want %d", tt.target, got, err, tt.want)
			}
		})
	}
	for _, ticks := range []int64{0, 1, 2, 5, 12, 48, 1_000_000} {
		got, err := GameDayAfterTicks(0, 0, 91, 12, ticks)
		if err != nil {
			t.Fatalf("ticks %d: %v", ticks, err)
		}
		want := GameDay(ticks * 91 / 12)
		if got != want {
			t.Fatalf("ticks %d = day %d, want %d", ticks, got, want)
		}
	}
}

func TestInvalidAndLargeClockInputs(t *testing.T) {
	for _, value := range [][5]int64{{0, -1, 91, 12, 1}, {0, 0, 0, 12, 1}, {0, 0, 91, 0, 1}, {0, 12, 91, 12, 1}, {0, 0, 91, 12, -1}} {
		if _, _, err := AdvanceTicks(GameDay(value[0]), value[1], value[2], value[3], value[4]); err == nil {
			t.Fatalf("AdvanceTicks(%v) accepted invalid clock", value)
		}
	}
	if _, err := GameDayAfterTicks(1, 0, 1, 1, math.MaxInt64); err == nil {
		t.Fatal("overflowing game-day projection was accepted")
	}
}

func TestCeilDaysForTicksUsesIntegerCalendarPacing(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ticks int64
		want  int64
	}{
		{name: "neighbor", ticks: 1, want: 8},
		{name: "local", ticks: 2, want: 16},
		{name: "regional", ticks: 5, want: 38},
		{name: "far regional", ticks: 8, want: 61},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CeilDaysForTicks(tt.ticks, 91, 12)
			if err != nil || got != tt.want {
				t.Fatalf("CeilDaysForTicks(%d, 91, 12) = %d, %v; want %d", tt.ticks, got, err, tt.want)
			}
		})
	}
}

func TestLatestDispatchGameDayUsesExactClockProjection(t *testing.T) {
	deadline, err := LatestDispatchGameDay(0, 0, 91, 12, 15, 2)
	if err != nil {
		t.Fatalf("LatestDispatchGameDay() error = %v", err)
	}

	// Tick 0 is day 0, tick 1 is day 7, and tick 2 is day 15. A shipment
	// dispatched on day 0 therefore arrives exactly on the due day.
	if deadline != 0 {
		t.Fatalf("LatestDispatchGameDay() = %d, want 0", deadline)
	}
}

func TestLatestDispatchGameDayUsesCalendarRemainder(t *testing.T) {
	deadline, err := LatestDispatchGameDay(7, 7, 91, 12, 22, 2)
	if err != nil {
		t.Fatalf("LatestDispatchGameDay() error = %v", err)
	}
	if deadline != 7 {
		t.Fatalf("LatestDispatchGameDay() = %d, want 7", deadline)
	}

	withRemainder, err := LatestDispatchGameDay(7, 7, 91, 12, 21, 2)
	if err != nil {
		t.Fatalf("LatestDispatchGameDay() with remainder error = %v", err)
	}
	withoutRemainder, err := LatestDispatchGameDay(7, 0, 91, 12, 21, 2)
	if err != nil {
		t.Fatalf("LatestDispatchGameDay() without remainder error = %v", err)
	}
	if withRemainder != 0 || withoutRemainder != -1 {
		t.Fatalf("remainder-sensitive deadlines = %d and %d, want 0 and -1", withRemainder, withoutRemainder)
	}
}

func TestLatestDispatchGameDayRejectsInvalidClock(t *testing.T) {
	tests := []struct {
		name        string
		remainder   int64
		numerator   int64
		denominator int64
		travelTicks int64
	}{
		{name: "zero numerator", numerator: 0, denominator: 12},
		{name: "zero denominator", numerator: 91, denominator: 0},
		{name: "negative remainder", remainder: -1, numerator: 91, denominator: 12},
		{name: "remainder at denominator", remainder: 12, numerator: 91, denominator: 12},
		{name: "negative travel ticks", numerator: 91, denominator: 12, travelTicks: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LatestDispatchGameDay(0, test.remainder, test.numerator, test.denominator, 15, test.travelTicks)
			if !errors.Is(err, ErrInvalidClock) {
				t.Fatalf("LatestDispatchGameDay() error = %v, want %v", err, ErrInvalidClock)
			}
		})
	}
}

func TestCalendarDeadlineArithmeticRejectsInvalidAndOverflowingInputs(t *testing.T) {
	if _, err := CeilDaysForTicks(math.MaxInt64, math.MaxInt64, 1); !errors.Is(err, ErrArithmeticOverflow) {
		t.Fatalf("overflowing travel calculation error = %v", err)
	}
	if _, err := CeilDaysForTicks(1, 91, 0); !errors.Is(err, ErrInvalidClock) {
		t.Fatalf("invalid denominator error = %v", err)
	}
	if _, err := LatestDispatchGameDay(math.MinInt64, 0, 1, 1, math.MaxInt64, 0); !errors.Is(err, ErrArithmeticOverflow) {
		t.Fatalf("overflowing deadline projection error = %v", err)
	}
	if _, err := SubtractInt64(math.MinInt64, 1); !errors.Is(err, ErrArithmeticOverflow) {
		t.Fatalf("overflowing tick subtraction error = %v", err)
	}
}
