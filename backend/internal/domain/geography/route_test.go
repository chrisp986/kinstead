package geography

import (
	"errors"
	"testing"
)

func TestRouteForDistanceUsesFrozenTravelAndTransportValues(t *testing.T) {
	tests := []struct {
		distance DistanceClass
		travel   Tick
		cost     MoneyMilli
	}{
		{DistanceNeighbor, 1, 0},
		{DistanceLocal, 2, 1_000},
		{DistanceNearRegional, 3, 2_000},
		{DistanceRegional, 5, 3_000},
		{DistanceFarRegional, 8, 5_000},
	}
	for _, tt := range tests {
		route, err := RouteForDistance("world", "origin", "destination", tt.distance)
		if err != nil || route.TravelTicks != tt.travel || route.TransportCostMilli != tt.cost {
			t.Fatalf("route %s = %+v, %v", tt.distance, route, err)
		}
	}
	if _, err := RouteForDistance("world", "origin", "destination", DistanceLong); !errors.Is(err, ErrRouteUnavailable) {
		t.Fatalf("long-distance error = %v", err)
	}
}
