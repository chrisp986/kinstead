package calendar

import (
	"errors"
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
