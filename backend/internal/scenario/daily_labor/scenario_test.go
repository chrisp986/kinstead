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
	if summary.MinimumWoodMilli <= 0 {
		t.Fatalf("wood reserve was not covered: %d", summary.MinimumWoodMilli)
	}
	if summary.MaximumFatigue > 35 {
		t.Fatalf("fatigue did not stabilize: %d", summary.MaximumFatigue)
	}
	if summary.EndingFoodMilli < 0 || summary.EndingWoodMilli < 0 {
		t.Fatalf("negative ending stocks: %+v", summary)
	}
	t.Logf("three-year baseline: %+v", summary)
}
