//go:build postgres

package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"game/backend/internal/port"
)

const householdChronicleLimit = 50

type ChronicleEntryRecord = port.ChronicleEntryRecord

// ListHouseholdChronicle returns the newest structured household facts first.
// Text rendering intentionally remains a client concern so facts can be localized.
func (s *Store) ListHouseholdChronicle(ctx context.Context, householdID string) ([]ChronicleEntryRecord, error) {
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM households WHERE id = $1::uuid)`, householdID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, pgx.ErrNoRows
	}

	rows, err := s.Pool.Query(ctx, `
		SELECT e.id::text, e.occurred_tick, e.occurred_game_day, e.entry_type,
               e.subject_character_id::text, subject.name,
               e.related_household_id::text, related.name,
               e.related_shipment_id::text, e.related_assignment_id::text,
               e.related_contract_id::text, e.related_obligation_id::text,
               e.related_household_decision_id::text, e.related_political_actor_id::text,
               e.data
        FROM chronicle_entries e
        LEFT JOIN characters subject ON subject.id = e.subject_character_id
        LEFT JOIN households related ON related.id = e.related_household_id
        WHERE e.household_id = $1::uuid
        ORDER BY e.occurred_tick DESC, e.created_at DESC, e.id DESC
        LIMIT $2
    `, householdID, householdChronicleLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]ChronicleEntryRecord, 0)
	for rows.Next() {
		var entry ChronicleEntryRecord
		var data []byte
		if err := rows.Scan(
			&entry.ID, &entry.OccurredTick, &entry.OccurredGameDay, &entry.EntryType,
			&entry.SubjectCharacterID, &entry.SubjectCharacterName,
			&entry.RelatedHouseholdID, &entry.RelatedHouseholdName,
			&entry.RelatedShipmentID, &entry.RelatedAssignmentID,
			&entry.RelatedContractID, &entry.RelatedObligationID,
			&entry.RelatedHouseholdDecisionID, &entry.RelatedPoliticalActorID, &data,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &entry.Data); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
