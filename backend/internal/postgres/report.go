//go:build postgres

package postgres

import (
	"context"
	"encoding/json"

	"game/backend/internal/port"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ListRecentChronicleForReport(ctx context.Context, householdID string, fromTick int64, limit int) ([]port.ChronicleEntryRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 50 {
		limit = 50
	}
	var exists bool
	if err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM households WHERE id=$1::uuid)`, householdID).Scan(&exists); err != nil {
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
		LEFT JOIN characters subject ON subject.id=e.subject_character_id
		LEFT JOIN households related ON related.id=e.related_household_id
		WHERE e.household_id=$1::uuid AND e.occurred_tick >= $2
		ORDER BY e.occurred_tick DESC, e.created_at DESC, e.id DESC
		LIMIT $3`, householdID, fromTick, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]port.ChronicleEntryRecord, 0)
	for rows.Next() {
		var e port.ChronicleEntryRecord
		var data []byte
		if err := rows.Scan(&e.ID, &e.OccurredTick, &e.OccurredGameDay, &e.EntryType, &e.SubjectCharacterID, &e.SubjectCharacterName,
			&e.RelatedHouseholdID, &e.RelatedHouseholdName, &e.RelatedShipmentID, &e.RelatedAssignmentID,
			&e.RelatedContractID, &e.RelatedObligationID, &e.RelatedHouseholdDecisionID, &e.RelatedPoliticalActorID, &data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &e.Data); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *Store) ListPendingPoliticalDemandsForReport(ctx context.Context, householdID string) ([]port.PoliticalReportDemand, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT d.id::text, a.name, d.expires_tick
		FROM household_decisions d
		JOIN world_events e ON e.id=d.world_event_id
		JOIN political_actors a ON a.id=e.political_actor_id
		WHERE d.household_id=$1::uuid AND d.status='pending'
		ORDER BY d.expires_tick, d.id`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]port.PoliticalReportDemand, 0)
	for rows.Next() {
		var item port.PoliticalReportDemand
		if err := rows.Scan(&item.ID, &item.ActorName, &item.ExpiresTick); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListContractObligationsForReport(ctx context.Context, householdID string) ([]port.ContractReportObligation, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT o.id::text, o.resource_code, o.quantity_milli, o.due_arrival_tick,
		       s.expected_arrival_tick, o.due_game_day, s.expected_arrival_game_day
		FROM contract_obligations o
		JOIN contracts c ON c.id=o.contract_id
		LEFT JOIN shipments s ON s.id=o.shipment_id
		WHERE o.debtor_household_id=$1::uuid AND o.status IN ('pending','dispatched','late')
		ORDER BY o.due_arrival_tick, o.id`, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]port.ContractReportObligation, 0)
	for rows.Next() {
		var item port.ContractReportObligation
		var expected, expectedGameDay *int64
		if err := rows.Scan(&item.ID, &item.ResourceType, &item.QuantityMilli, &item.DueArrivalTick, &expected, &item.DueGameDay, &expectedGameDay); err != nil {
			return nil, err
		}
		item.ExpectedArrivalTick = expected
		item.ExpectedArrivalGameDay = expectedGameDay
		items = append(items, item)
	}
	return items, rows.Err()
}
