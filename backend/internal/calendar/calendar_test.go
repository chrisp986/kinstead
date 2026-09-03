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
