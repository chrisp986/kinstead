package politics

import "testing"

func TestResolution(t *testing.T) {
	cases := []struct {
		d        DemandType
		o        Option
		delta    int
		resource string
		amount   int64
		worker   bool
	}{
		{DemandLaborService, OptionServe, 10, "", 0, true},
		{DemandLaborService, OptionRefuse, -5, "", 0, false},
		{DemandLevy, OptionPayWood, 10, "wood", 18000, false},
		{DemandLevy, OptionPaySilver, 10, "silver", 6000, false},
		{DemandLevy, OptionRefuse, -5, "", 0, false},
	}
	for _, tc := range cases {
		r, err := ResolveChoice(tc.d, tc.o)
		if err != nil || r.StandingDelta != tc.delta || r.ResourceCode != tc.resource || r.ResourceMilli != tc.amount || r.RequiresWorker != tc.worker {
			t.Errorf("%v %v => %#v %v", tc.d, tc.o, r, err)
		}
	}
}

func TestStandingAndClamp(t *testing.T) {
	for _, tc := range []struct {
		in       Score
		out      Score
		standing Standing
	}{{-101, -100, StandingDisapproving}, {-30, -30, StandingNeutral}, {29, 29, StandingNeutral}, {30, 30, StandingFavorable}, {69, 69, StandingFavorable}, {70, 70, StandingConnected}, {101, 100, StandingConnected}} {
		if got := ClampScore(tc.in); got != tc.out || DeriveStanding(tc.in) != tc.standing {
			t.Errorf("%v => %v %v", tc.in, got, DeriveStanding(tc.in))
		}
	}
}

func TestResponseDeadline(t *testing.T) {
	if err := ResponseAllowed(10, 11); err != nil {
		t.Fatal(err)
	}
	if err := ResponseAllowed(11, 11); err != ErrExpired {
		t.Fatalf("expected expiry, got %v", err)
	}
}
