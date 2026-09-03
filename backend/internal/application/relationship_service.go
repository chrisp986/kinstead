package application

import (
	"context"
	"fmt"

	relationshipdomain "game/backend/internal/domain/relationship"
	"game/backend/internal/port"
)

type RelationshipProjection struct {
	port.RelationshipRecord
	Standing relationshipdomain.Standing `json:"standing"`
}

type RelationshipService struct {
	Reader port.RelationshipReader
}

func NewRelationshipService(reader port.RelationshipReader) *RelationshipService {
	return &RelationshipService{Reader: reader}
}

func (s *RelationshipService) ListForHousehold(ctx context.Context, householdID string) ([]RelationshipProjection, error) {
	records, err := s.Reader.ListRelationshipsForHousehold(ctx, householdID)
	if err != nil {
		return nil, err
	}
	values := make([]RelationshipProjection, 0, len(records))
	for _, record := range records {
		standing, err := relationshipdomain.StandingForTrust(record.Trust)
		if err != nil {
			return nil, fmt.Errorf("relationship %s -> %s: %w", record.SourceHouseholdID, record.TargetHouseholdID, err)
		}
		values = append(values, RelationshipProjection{RelationshipRecord: record, Standing: standing})
	}
	return values, nil
}
