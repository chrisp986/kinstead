package geography

import (
	"errors"
	"math"
)

type WorldID string
type LocationID string
type Tick int64
type MoneyMilli int64
type DistanceClass string

const (
	DistanceNeighbor     DistanceClass = "neighbor"
	DistanceLocal        DistanceClass = "local"
	DistanceNearRegional DistanceClass = "near_regional"
	DistanceRegional     DistanceClass = "regional"
	DistanceFarRegional  DistanceClass = "far_regional"
	DistanceLong         DistanceClass = "long_distance"
)

var (
	ErrRouteUnavailable   = errors.New("no route exists between locations")
	ErrInvalidTravelTime  = errors.New("travel time must be positive")
	ErrArithmeticOverflow = errors.New("geography arithmetic overflow")
)

type Route struct {
	WorldID               WorldID
	OriginLocationID      LocationID
	DestinationLocationID LocationID
	DistanceClass         DistanceClass
	TravelTicks           Tick
	TransportCostMilli    MoneyMilli
}

// RouteForDistance is the shared authoritative conversion from geography to
// travel duration and fixed-point transport cost. Long-distance cost remains
// intentionally undefined in the frozen v0.3 balance.
func RouteForDistance(worldID WorldID, origin, destination LocationID, distance DistanceClass) (Route, error) {
	route := Route{WorldID: worldID, OriginLocationID: origin, DestinationLocationID: destination, DistanceClass: distance}
	switch distance {
	case DistanceNeighbor:
		route.TravelTicks, route.TransportCostMilli = 1, 0
	case DistanceLocal:
		route.TravelTicks, route.TransportCostMilli = 2, 1_000
	case DistanceNearRegional:
		route.TravelTicks, route.TransportCostMilli = 3, 2_000
	case DistanceRegional:
		route.TravelTicks, route.TransportCostMilli = 5, 3_000
	case DistanceFarRegional:
		route.TravelTicks, route.TransportCostMilli = 8, 5_000
	case DistanceLong:
		return Route{}, ErrRouteUnavailable
	default:
		return Route{}, ErrRouteUnavailable
	}
	if worldID == "" || origin == "" || destination == "" || origin == destination {
		return Route{}, ErrRouteUnavailable
	}
	return route, nil
}

func ArrivalTick(departure, travelTicks Tick) (Tick, error) {
	if travelTicks <= 0 {
		return 0, ErrInvalidTravelTime
	}
	if int64(departure) > math.MaxInt64-int64(travelTicks) {
		return 0, ErrArithmeticOverflow
	}
	return departure + travelTicks, nil
}
