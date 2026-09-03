package application

import (
	"context"
	"testing"

	relationshipdomain "game/backend/internal/domain/relationship"
	"game/backend/internal/port"
)

type relationshipReaderStub struct{ records []port.RelationshipRecord }

func (s relationshipReaderStub) ListRelationshipsForHousehold(context.Context, string) ([]port.RelationshipRecord, error) {
	return s.records, nil
}

func TestRelationshipServiceAddsStanding(t *testing.T) {
	service := NewRelationshipService(relationshipReaderStub{records: []port.RelationshipRecord{{
		SourceHouseholdID: "a", TargetHouseholdID: "b", Trust: 30,
	}}})
	values, err := service.ListForHousehold(context.Background(), "a")
	if err != nil || len(values) != 1 || values[0].Standing != relationshipdomain.StandingFavorable {
		t.Fatalf("relationships = %+v, %v", values, err)
	}
}
