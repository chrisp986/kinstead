package calendar

import (
	"testing"
	"time"
)

func TestSeasonForAllMonths(t *testing.T) {
	want := []ProductionSeason{Spring, Summer, Autumn, Winter, Spring, Summer, Autumn, Winter, Spring, Summer, Autumn, Winter}
	for month := time.January; month <= time.December; month++ {
		got, err := SeasonForMonth(month)
		if err != nil || got != want[int(month)-1] {
			t.Fatalf("month %s: got %q err=%v want %q", month, got, err, want[int(month)-1])
		}
	}
}

func TestSeasonalPositionLeapFebruaryAndYearBoundary(t *testing.T) {
	zone := time.FixedZone("world", 60*60)
	position, err := SeasonalPositionForDate(time.Date(2028, time.February, 29, 12, 0, 0, 0, zone))
	if err != nil || position.Season != Summer || position.LengthDays != 29 || position.Day != 29 {
		t.Fatalf("leap position=%+v err=%v", position, err)
	}
	december, _ := SeasonalPositionForDate(time.Date(2027, time.December, 31, 0, 0, 0, 0, zone))
	january, _ := SeasonalPositionForDate(december.NextBoundary)
	if december.Season != Winter || january.Season != Spring || january.Cycle != 1 {
		t.Fatalf("December=%+v January=%+v", december, january)
	}
}

func TestSchedulingDateUsesFixedWorldOffset(t *testing.T) {
	anchor := time.Date(2027, time.January, 1, 23, 0, 0, 0, time.UTC) // midnight at UTC+1
	got, err := SchedulingDate(anchor, 60, Moment{Day: 31, Hour: 6})
	if err != nil {
		t.Fatal(err)
	}
	if y, m, d := got.Date(); y != 2027 || m != time.February || d != 2 || got.Hour() != 6 {
		t.Fatalf("scheduling date=%s", got)
	}
}
