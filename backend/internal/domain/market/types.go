package market

import (
	"errors"
	"math"
)

type OfferID string
type WorldID string
type HouseholdID string
type LocationID string
type ResourceType string
type QuantityMilli int64
type MoneyMilli int64
type Tick int64
type OfferStatus string

const (
	OfferActive    OfferStatus = "active"
	OfferFilled    OfferStatus = "filled"
	OfferCancelled OfferStatus = "cancelled"
	OfferExpired   OfferStatus = "expired"
)

var (
	ErrInvalidOffer       = errors.New("invalid market offer")
	ErrInvalidQuantity    = errors.New("purchase quantity must be positive")
	ErrOfferUnavailable   = errors.New("market offer is unavailable")
	ErrOfferExpired       = errors.New("market offer has expired")
	ErrInsufficientOffer  = errors.New("purchase quantity exceeds offer quantity")
	ErrInsufficientFunds  = errors.New("buyer has insufficient funds")
	ErrInsufficientStock  = errors.New("seller has insufficient stock")
	ErrOwnOffer           = errors.New("household cannot buy its own offer")
	ErrInvalidTravelTime  = errors.New("travel time must be positive")
	ErrArithmeticOverflow = errors.New("market arithmetic overflow")
)

type Offer struct {
	ID                     OfferID
	WorldID                WorldID
	SellerHouseholdID      HouseholdID
	OriginLocationID       LocationID
	ResourceType           ResourceType
	QuantityRemainingMilli QuantityMilli
	PricePerUnitMilli      MoneyMilli
	CreatedTick            Tick
	ExpiresTick            *Tick
	Status                 OfferStatus
}

type Buyer struct {
	HouseholdID HouseholdID
	WorldID     WorldID
	LocationID  LocationID
	SilverMilli MoneyMilli
}

type Purchase struct {
	Offer         Offer
	Buyer         Buyer
	QuantityMilli QuantityMilli
	CostMilli     MoneyMilli
	CurrentTick   Tick
}

func (o Offer) Validate() error {
	if o.ID == "" || o.WorldID == "" || o.SellerHouseholdID == "" ||
		o.OriginLocationID == "" || o.ResourceType == "" || o.PricePerUnitMilli <= 0 ||
		o.CreatedTick < 0 || (o.ExpiresTick != nil && *o.ExpiresTick <= o.CreatedTick) {
		return ErrInvalidOffer
	}
	switch o.Status {
	case OfferActive:
		if o.QuantityRemainingMilli <= 0 {
			return ErrInvalidOffer
		}
	case OfferFilled:
		if o.QuantityRemainingMilli != 0 {
			return ErrInvalidOffer
		}
	case OfferCancelled, OfferExpired:
		if o.QuantityRemainingMilli < 0 {
			return ErrInvalidOffer
		}
	default:
		return ErrInvalidOffer
	}
	return nil
}

func CostMilli(quantity QuantityMilli, pricePerUnit MoneyMilli) (MoneyMilli, error) {
	if quantity <= 0 {
		return 0, ErrInvalidQuantity
	}
	if pricePerUnit <= 0 {
		return 0, ErrInvalidOffer
	}
	if int64(quantity) > math.MaxInt64/int64(pricePerUnit) {
		return 0, ErrArithmeticOverflow
	}
	product := int64(quantity) * int64(pricePerUnit)
	cost := product / 1000
	if product%1000 != 0 {
		cost++
	}
	return MoneyMilli(cost), nil
}

func EvaluatePurchase(offer Offer, buyer Buyer, sellerStock QuantityMilli, quantity QuantityMilli, currentTick Tick) (Purchase, error) {
	if err := offer.Validate(); err != nil {
		return Purchase{}, err
	}
	if quantity <= 0 {
		return Purchase{}, ErrInvalidQuantity
	}
	if offer.Status != OfferActive {
		return Purchase{}, ErrOfferUnavailable
	}
	if offer.ExpiresTick != nil && currentTick > *offer.ExpiresTick {
		return Purchase{}, ErrOfferExpired
	}
	if buyer.HouseholdID == "" || buyer.WorldID != offer.WorldID || buyer.LocationID == "" {
		return Purchase{}, ErrInvalidOffer
	}
	if buyer.HouseholdID == offer.SellerHouseholdID {
		return Purchase{}, ErrOwnOffer
	}
	if quantity > offer.QuantityRemainingMilli {
		return Purchase{}, ErrInsufficientOffer
	}
	if sellerStock < quantity {
		return Purchase{}, ErrInsufficientStock
	}
	cost, err := CostMilli(quantity, offer.PricePerUnitMilli)
	if err != nil {
		return Purchase{}, err
	}
	if buyer.SilverMilli < cost {
		return Purchase{}, ErrInsufficientFunds
	}

	updated := offer
	updated.QuantityRemainingMilli -= quantity
	if updated.QuantityRemainingMilli == 0 {
		updated.Status = OfferFilled
	}
	return Purchase{Offer: updated, Buyer: buyer, QuantityMilli: quantity, CostMilli: cost, CurrentTick: currentTick}, nil
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
