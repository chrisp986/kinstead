package report

import "testing"

func TestSupplyAttentionThresholds(t *testing.T) {
	for _, tc := range []struct {
		days float64
		code string
	}{
		{31, ""}, {30, "supply_strained"}, {15, "supply_strained"}, {14.9, "supply_critical"}, {7, "supply_critical"}, {6.9, "supply_emergency"},
	} {
		items := BuildAttention(Input{SupplyDays: tc.days})
		got := ""
		if len(items) > 0 {
			got = items[0].Code
		}
		if got != tc.code {
			t.Errorf("supply %.1f code %q, want %q", tc.days, got, tc.code)
		}
	}
}

func TestDecisionRankingIsStableAndCapped(t *testing.T) {
	in := Input{CurrentTick: 10, SupplyDays: 6.5,
		Characters:          []Character{{ID: "c", Name: "Bjorn", Fatigue: 90}},
		PoliticalDemands:    []PoliticalDemand{{ID: "p", ActorName: "Eirik", ExpiresTick: 11}},
		ContractObligations: []ContractObligation{{ID: "o", ResourceType: "wood", QuantityMilli: 10000, DueArrivalTick: 11}},
	}
	want := []string{"secure_provisions", "respond_political_demand", "dispatch_contract_obligation"}
	for run := 0; run < 3; run++ {
		got := BuildDecisions(in)
		if len(got) != 3 {
			t.Fatalf("decision count=%d", len(got))
		}
		for i := range want {
			if got[i].Code != want[i] {
				t.Fatalf("decision %d=%s, want %s", i, got[i].Code, want[i])
			}
		}
	}
}

func TestFatigueThresholds(t *testing.T) {
	for _, tc := range []struct {
		fatigue int
		code    string
	}{{69, ""}, {70, "character_fatigue_high"}, {84, "character_fatigue_high"}, {85, "character_fatigue_critical"}, {100, "character_fatigue_critical"}} {
		items := BuildAttention(Input{SupplyDays: 31, Characters: []Character{{ID: "c", Name: "Bjorn", Fatigue: tc.fatigue}}})
		got := ""
		if len(items) > 0 {
			got = items[0].Code
		}
		if got != tc.code {
			t.Errorf("fatigue %d code %q, want %q", tc.fatigue, got, tc.code)
		}
	}
}
