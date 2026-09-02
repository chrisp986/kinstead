//go:build postgres

package httpapi

import "testing"

func TestValidActivityMatchesOnlineSimulation(t *testing.T) {
	for _, activity := range []string{"agriculture", "fishing", "woodcutting", "rest"} {
		if !validActivity(activity) {
			t.Fatalf("%q should be available online", activity)
		}
	}
	for _, activity := range []string{"building", "crafting", "training", "market", "travel", "ruler_service"} {
		if validActivity(activity) {
			t.Fatalf("%q has no implemented online effect and must be rejected", activity)
		}
	}
}
