package dailylabor

import "testing"

func TestStartingHouseholdSurvivesMultipleYears(t *testing.T) {
	summary, err := RunStartingHousehold(3)
	if err != nil {
		t.Fatal(err)
	}
	if summary.FoodShortageHours != 0 {
		t.Fatalf("food shortage hours=%d", summary.FoodShortageHours)
	}
	if summary.Days != 1096 { // 2028 is leap; the following two years are not.
		t.Fatalf("calendar days=%d", summary.Days)
	}
	if summary.MinimumWoodMilli <= 0 {
		t.Fatalf("wood reserve was not covered: %d", summary.MinimumWoodMilli)
	}
	if summary.MaximumFatigue > 35 {
		t.Fatalf("fatigue did not stabilize: %d", summary.MaximumFatigue)
	}
	if summary.EndingFoodMilli < 0 || summary.EndingWoodMilli < 0 {
		t.Fatalf("negative ending stocks: %+v", summary)
	}
	if summary.EndingFoodMilli > 500_000 || summary.EndingWoodMilli > 600_000 {
		t.Fatalf("starting plan accumulates implausibly large reserves: %+v", summary)
	}
	t.Logf("three-year baseline: %+v", summary)
}
