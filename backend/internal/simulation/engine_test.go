package simulation

import "testing"

func TestBjornvikSeed(t *testing.T) {
	s := NewBjornvikState()
	if len(s.Characters) != 5 {
		t.Fatalf("characters=%d", len(s.Characters))
	}
	if s.ProvisionsMilli != 150_000 || s.WoodMilli != 20_000 || s.SilverMilli != 30_000 {
		t.Fatalf("unexpected seed resources: %+v", s)
	}
}

func TestSeasonBoundaries(t *testing.T) {
	cases := map[int64]Season{1: Spring, 12: Spring, 13: Summer, 24: Summer, 25: Autumn, 36: Autumn, 37: Winter, 48: Winter}
	for tick, want := range cases {
		if got := SeasonForTick(tick); got != want {
			t.Fatalf("tick %d: got %s want %s", tick, got, want)
		}
	}
}

func TestSpecializedFishingProduction(t *testing.T) {
	s := NewBjornvikState()
	cfg := DefaultBalanceConfig()
	result, err := ProcessTick(s, 1, []Assignment{{Character: "Astrid", Activity: Fishing, Intensity: Normal}}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Spring fishing: 3.0 * farm 1.15 * Astrid skill 1.15 = 3.967 (integer milli arithmetic).
	if result.ProducedProvisionsMilli != 3967 {
		t.Fatalf("produced=%d", result.ProducedProvisionsMilli)
	}
}

func TestFatigueThresholdPenalty(t *testing.T) {
	s := NewBjornvikState()
	for i := range s.Characters {
		if s.Characters[i].Name == "Einar" {
			s.Characters[i].Fatigue = 70
		}
	}
	result, err := ProcessTick(s, 1, []Assignment{{Character: "Einar", Activity: Fishing, Intensity: Normal}}, DefaultBalanceConfig())
	if err != nil {
		t.Fatal(err)
	}
	// 3.0 * farm 1.15 * fatigue 0.9 = 3.105
	if result.ProducedProvisionsMilli != 3105 {
		t.Fatalf("produced=%d", result.ProducedProvisionsMilli)
	}
}

func TestRestRecoversFatigue(t *testing.T) {
	s := NewBjornvikState()
	s.Characters[0].Fatigue = 20
	result, err := ProcessTick(s, 1, nil, DefaultBalanceConfig())
	if err != nil {
		t.Fatal(err)
	}
	if result.State.Characters[0].Fatigue != 8 {
		t.Fatalf("fatigue=%d", result.State.Characters[0].Fatigue)
	}
}

func TestRejectsDuplicateAssignment(t *testing.T) {
	_, err := ProcessTick(NewBjornvikState(), 1, []Assignment{
		{Character: "Bjorn", Activity: Agriculture, Intensity: Normal},
		{Character: "Bjorn", Activity: Fishing, Intensity: Normal},
	}, DefaultBalanceConfig())
	if err == nil {
		t.Fatal("expected duplicate assignment error")
	}
}
