//go:build postgres

package application

import (
	"context"

	"game/backend/internal/postgres"
)

type ChronicleService struct {
	Store *postgres.Store
}

func NewChronicleService(store *postgres.Store) *ChronicleService {
	return &ChronicleService{Store: store}
}

func (s *ChronicleService) ListForHousehold(ctx context.Context, householdID string) ([]postgres.ChronicleEntryRecord, error) {
	return s.Store.ListHouseholdChronicle(ctx, householdID)
}
