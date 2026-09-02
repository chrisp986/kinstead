package market

import (
	"errors"
	"math"
	"testing"
)

func activeOffer() Offer {
	return Offer{
		ID: "offer-1", WorldID: "world-1", SellerHouseholdID: "seller-1",
		OriginLocationID: "origin-1", ResourceType: "provisions",
		QuantityRemainingMilli: 60_000, PricePerUnitMilli: 1_500,
		CreatedTick: 1, Status: OfferActive,
	}
}

func fundedBuyer() Buyer {
	return Buyer{HouseholdID: "buyer-1", WorldID: "world-1", LocationID: "buyer-location", SilverMilli: 100_000}
}

func TestEvaluatePartialAndFullPurchase(t *testing.T) {
	partial, err := EvaluatePurchase(activeOffer(), fundedBuyer(), 60_000, 20_000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if partial.CostMilli != 30_000 || partial.Offer.QuantityRemainingMilli != 40_000 || partial.Offer.Status != OfferActive {
		t.Fatalf("partial purchase = %+v", partial)
	}

	full, err := EvaluatePurchase(activeOffer(), fundedBuyer(), 60_000, 60_000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if full.CostMilli != 90_000 || full.Offer.QuantityRemainingMilli != 0 || full.Offer.Status != OfferFilled {
		t.Fatalf("full purchase = %+v", full)
	}
}

func TestCostRoundsUpWithoutFloats(t *testing.T) {
	cost, err := CostMilli(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if cost != 1 {
		t.Fatalf("cost = %d, want 1", cost)
	}
	if _, err := CostMilli(QuantityMilli(math.MaxInt64), 2); !errors.Is(err, ErrArithmeticOverflow) {
		t.Fatalf("overflow error = %v", err)
	}
}

func TestPurchaseValidationFailures(t *testing.T) {
	tests := []struct {
		name        string
		offer       Offer
		buyer       Buyer
		stock       QuantityMilli
		quantity    QuantityMilli
		currentTick Tick
		want        error
	}{
		{"zero quantity", activeOffer(), fundedBuyer(), 60_000, 0, 1, ErrInvalidQuantity},
		{"offer quantity", activeOffer(), fundedBuyer(), 60_000, 60_001, 1, ErrInsufficientOffer},
		{"seller stock", activeOffer(), fundedBuyer(), 10_000, 20_000, 1, ErrInsufficientStock},
		{"buyer funds", activeOffer(), Buyer{HouseholdID: "buyer", WorldID: "world-1", LocationID: "location", SilverMilli: 1}, 60_000, 20_000, 1, ErrInsufficientFunds},
		{"own offer", activeOffer(), Buyer{HouseholdID: "seller-1", WorldID: "world-1", LocationID: "origin-1", SilverMilli: 100_000}, 60_000, 20_000, 1, ErrOwnOffer},
	}
	expires := Tick(2)
	expired := activeOffer()
	expired.ExpiresTick = &expires
	tests = append(tests, struct {
		name        string
		offer       Offer
		buyer       Buyer
		stock       QuantityMilli
		quantity    QuantityMilli
		currentTick Tick
		want        error
	}{"expired", expired, fundedBuyer(), 60_000, 20_000, 3, ErrOfferExpired})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EvaluatePurchase(tt.offer, tt.buyer, tt.stock, tt.quantity, tt.currentTick)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestArrivalTick(t *testing.T) {
	arrival, err := ArrivalTick(7, 2)
	if err != nil || arrival != 9 {
		t.Fatalf("arrival = %d, %v", arrival, err)
	}
	if _, err := ArrivalTick(7, 0); !errors.Is(err, ErrInvalidTravelTime) {
		t.Fatalf("error = %v, want ErrInvalidTravelTime", err)
	}
}
