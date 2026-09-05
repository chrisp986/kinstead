//go:build postgres

package postgres

import (
	"errors"
	"math"
	"testing"

	"game/backend/internal/calendar"
)

func TestCeilTravelDays(t *testing.T) {
	got, err := calendar.CeilDaysForTicks(5, 91, 12)
	if err != nil {
		t.Fatal(err)
	}
	if got != 38 {
		t.Fatalf("CeilDaysForTicks(5, 91, 12) = %d, want 38", got)
	}

	got, err = calendar.CeilDaysForTicks(1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("CeilDaysForTicks(1, 1, 2) = %d, want 1", got)
	}
}

func TestCeilTravelDaysRejectsOverflow(t *testing.T) {
	if _, err := calendar.CeilDaysForTicks(math.MaxInt64, math.MaxInt64, 1); !errors.Is(err, calendar.ErrArithmeticOverflow) {
		t.Fatalf("overflow error = %v, want %v", err, calendar.ErrArithmeticOverflow)
	}
}

func TestCalendarQueryEndDoesNotOverflow(t *testing.T) {
	if got := calendarQueryEnd(math.MaxInt64); got != math.MaxInt64 {
		t.Fatalf("calendarQueryEnd(MaxInt64) = %d, want MaxInt64", got)
	}
	if got := calendarQueryEnd(math.MaxInt64 - 91); got != math.MaxInt64 {
		t.Fatalf("calendarQueryEnd(MaxInt64-91) = %d, want MaxInt64", got)
	}
}
