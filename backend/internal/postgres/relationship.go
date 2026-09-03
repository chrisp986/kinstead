//go:build postgres

package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"game/backend/internal/port"
	sqlcdb "game/backend/internal/postgres/db"
)

func (s *Store) ListRelationshipsForHousehold(ctx context.Context, householdID string) ([]port.RelationshipRecord, error) {
	id, err := uuidParam(householdID)
	if err != nil {
		return nil, err
	}
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM households WHERE id = $1::uuid)`, id).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, pgx.ErrNoRows
	}
	queries := sqlcdb.New(s.Pool)
	rows, err := queries.ListRelationshipsForHousehold(ctx, id)
	if err != nil {
		return nil, err
	}
	values := make([]port.RelationshipRecord, 0, len(rows))
	for _, row := range rows {
		record := port.RelationshipRecord{
			WorldID: row.WorldID, SourceHouseholdID: row.SourceHouseholdID, SourceHouseholdName: row.SourceHouseholdName,
			TargetHouseholdID: row.TargetHouseholdID, TargetHouseholdName: row.TargetHouseholdName, Trust: int(row.Trust),
		}
		if row.FirstInteractionTick.Valid {
			value := row.FirstInteractionTick.Int64
			record.FirstInteractionTick = &value
		}
		sourceID, err := uuidParam(row.SourceHouseholdID)
		if err != nil {
			return nil, err
		}
		targetID, err := uuidParam(row.TargetHouseholdID)
		if err != nil {
			return nil, err
		}
		events, err := queries.ListRelationshipEventsBetween(ctx, sqlcdb.ListRelationshipEventsBetweenParams{
			Column1: sourceID, Column2: targetID,
		})
		if err != nil {
			return nil, err
		}
		record.Events = make([]port.RelationshipEventRecord, 0, len(events))
		for _, event := range events {
			data := make(map[string]any)
			if err := json.Unmarshal(event.Data, &data); err != nil {
				return nil, err
			}
			record.Events = append(record.Events, port.RelationshipEventRecord{
				ID: event.ID, EventType: event.EventType, TrustDelta: int(event.TrustDelta), OccurredTick: event.OccurredTick,
				RelatedContractID: optionalString(event.RelatedContractID), RelatedShipmentID: optionalString(event.RelatedShipmentID),
				RelatedObligationID: optionalString(event.RelatedObligationID), Data: data,
			})
		}
		values = append(values, record)
	}
	return values, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
