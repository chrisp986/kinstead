package v03

import (
	"math"
	"testing"
)

func TestGoScenarioRegressionSnapshot(t *testing.T) {
	want := map[StrategyName]ScenarioSummary{
		StrategyAutark:     {SupplyDays: 34.04, Silver: 30, Wood: 42.33, StrainedDays: 7, CriticalDays: 0, TradeVolume: 0, JarlStanding: 0, PoliticalServiceDays: 4, Buildings: []string{"storage", "workshop"}},
		StrategySupplySafe: {SupplyDays: 41.16, Silver: 30, Wood: 25.90, StrainedDays: 0, CriticalDays: 0, TradeVolume: 0, JarlStanding: 0, PoliticalServiceDays: 4, Buildings: []string{"storage", "workshop"}},
		StrategyTrader:     {SupplyDays: 28.20, Silver: 78, Wood: 46, StrainedDays: 46, CriticalDays: 0, TradeVolume: 374, JarlStanding: 0, PoliticalSilverPaid: 6, Buildings: []string{"storage", "workshop"}},
		StrategyBuilder:    {SupplyDays: 19.52, Silver: 16, Wood: 81.83, StrainedDays: 46, CriticalDays: 0, TradeVolume: 42, JarlStanding: -15, Buildings: []string{"storage", "workshop", "housing"}},
		StrategyLoyal:      {SupplyDays: 23.49, Silver: 30, Wood: 59.80, StrainedDays: 46, CriticalDays: 0, TradeVolume: 0, JarlStanding: 30, PoliticalServiceDays: 8, PoliticalWoodPaid: 18, Buildings: []string{"storage", "workshop"}},
	}
	rows, err := RunAllV03Scenarios()
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range rows {
		expected := want[got.Strategy]
		if round2(got.SupplyDays) != expected.SupplyDays || round2(got.Silver) != expected.Silver || round2(got.Wood) != expected.Wood ||
			got.StrainedDays != expected.StrainedDays || got.CriticalDays != expected.CriticalDays ||
			round2(got.TradeVolume) != expected.TradeVolume || got.JarlStanding != expected.JarlStanding ||
			got.PoliticalServiceDays != expected.PoliticalServiceDays || got.PoliticalWoodPaid != expected.PoliticalWoodPaid ||
			got.PoliticalSilverPaid != expected.PoliticalSilverPaid || !sameStrings(got.Buildings, expected.Buildings) {
			t.Fatalf("%s regression drift:\n got  %+v\n want %+v", got.Strategy, got, expected)
		}
	}
}

func round2(value float64) float64 { return math.Round(value*100) / 100 }

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestV03StrategyProfilesRemainDistinct(t *testing.T) {
	rows, err := RunAllV03Scenarios()
	if err != nil {
		t.Fatal(err)
	}
	by := map[StrategyName]ScenarioSummary{}
	for _, r := range rows {
		by[r.Strategy] = r
	}

	if by[StrategySupplySafe].SupplyDays <= by[StrategyAutark].SupplyDays {
		t.Fatalf("supply-safe should finish with more supply than autark")
	}
	if by[StrategyTrader].Silver <= by[StrategyAutark].Silver || by[StrategyTrader].TradeVolume <= 0 {
		t.Fatalf("trader should lead liquidity and trade")
	}
	if len(by[StrategyBuilder].Buildings) != 3 || by[StrategyBuilder].JarlStanding != -15 {
		t.Fatalf("builder profile drifted: %+v", by[StrategyBuilder])
	}
	if by[StrategyLoyal].JarlStanding != 30 || by[StrategyLoyal].PoliticalServiceDays != 8 || by[StrategyLoyal].PoliticalWoodPaid != 18 {
		t.Fatalf("loyal political cost/reward drifted: %+v", by[StrategyLoyal])
	}
	for _, r := range rows {
		if r.CriticalDays != 0 {
			t.Fatalf("%s entered critical supply in deterministic baseline", r.Strategy)
		}
	}
}
