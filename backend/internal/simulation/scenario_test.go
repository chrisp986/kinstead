package simulation

import "testing"

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
