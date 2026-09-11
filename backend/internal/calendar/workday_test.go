package calendar

import (
	"testing"
	"time"
)

func TestWorkdayInterpolationAndLimits(t *testing.T) {
	cfg := DefaultDaylightConfig()
	first, err := WorkdayForDate(time.Date(2028, time.February, 1, 0, 0, 0, 0, time.UTC), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first != (Workday{SunriseHour: 5, SunsetHour: 21, StartHour: 6, EndHour: 14}) {
		t.Fatalf("summer first day=%+v", first)
	}
	winter, err := WorkdayForDate(time.Date(2028, time.April, 1, 0, 0, 0, 0, time.UTC), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if winter.StartHour != 10 || winter.EndHour != 15 || !winter.IsWorkingHour(14) || winter.IsWorkingHour(15) {
		t.Fatalf("winter workday=%+v", winter)
	}
	lateWinter, err := WorkdayForDate(time.Date(2028, time.December, 31, 0, 0, 0, 0, time.UTC), cfg)
	if err != nil || lateWinter.EndHour < lateWinter.StartHour {
		t.Fatalf("winter-to-spring=%+v err=%v", lateWinter, err)
	}
}
