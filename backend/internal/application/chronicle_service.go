package application

import (
	"context"

	"game/backend/internal/port"
)

type ChronicleService struct {
	Store port.ChronicleReader
}

func NewChronicleService(store port.ChronicleReader) *ChronicleService {
	return &ChronicleService{Store: store}
}

func (s *ChronicleService) ListForHousehold(ctx context.Context, householdID string) ([]port.ChronicleEntryRecord, error) {
	return s.Store.ListHouseholdChronicle(ctx, householdID)
}
