//go:build postgres

package postgres

import (
	"context"

	"game/backend/internal/port"
)

// LoadCalendarContext reads only the household's range-relevant source rows.
// Calendar events are assembled by the application layer so this adapter does
// not become a second source of temporal truth.
func (s *Store) LoadCalendarContext(ctx context.Context, householdID string, from, to int64) (port.CalendarContext, error) {
	snapshot, err := s.GetHouseholdReport(ctx, householdID)
	if err != nil {
		return port.CalendarContext{}, err
	}
	contextValue := port.CalendarContext{Snapshot: snapshot}
	id, err := uuidParam(householdID)
	if err != nil {
		return port.CalendarContext{}, err
	}

	// Include a short margin so a dispatch deadline can be shown even when the
	// delivery itself is just outside the requested half-year window.
	rows, err := s.Pool.Query(ctx, `
		SELECT o.id::text, o.contract_id::text, o.debtor_household_id::text,
		       o.creditor_household_id::text,
		       CASE WHEN o.debtor_household_id = $1::uuid THEN creditor.name ELSE debtor.name END,
		       o.resource_code, o.quantity_milli, o.due_game_day,
		       COALESCE(o.shipment_id::text, ''), o.status,
		       CASE lr.distance_class
		         WHEN 'neighbor' THEN 1 WHEN 'local' THEN 2
		         WHEN 'near_regional' THEN 3 WHEN 'regional' THEN 5
		         WHEN 'far_regional' THEN 8 ELSE 0 END AS travel_ticks
		FROM contract_obligations o
		JOIN contracts c ON c.id = o.contract_id AND c.status IN ('active', 'broken')
		JOIN households debtor ON debtor.id = o.debtor_household_id
		JOIN households creditor ON creditor.id = o.creditor_household_id
		LEFT JOIN location_routes lr
		  ON lr.world_id = c.world_id
		 AND lr.origin_location_id = debtor.location_id
		 AND lr.destination_location_id = creditor.location_id
		WHERE (o.debtor_household_id = $1::uuid OR o.creditor_household_id = $1::uuid)
		  AND o.due_game_day BETWEEN $2 AND $3
		  AND o.status IN ('pending', 'dispatched', 'late')
		ORDER BY o.due_game_day, o.id
	`, id, from, to+91)
	if err != nil {
		return port.CalendarContext{}, err
	}
	for rows.Next() {
		var item port.CalendarObligationRecord
		var travelTicks int64
		if err := rows.Scan(&item.ID, &item.ContractID, &item.DebtorHouseholdID, &item.CreditorHouseholdID,
			&item.CounterpartyName, &item.ResourceType, &item.QuantityMilli, &item.DueGameDay,
			&item.ShipmentID, &item.Status, &travelTicks); err != nil {
			rows.Close()
			return port.CalendarContext{}, err
		}
		item.LatestDispatchGameDay = item.DueGameDay - ceilTravelDays(travelTicks, snapshot.GameDaysPerTickNum, snapshot.GameDaysPerTickDen)
		contextValue.Obligations = append(contextValue.Obligations, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return port.CalendarContext{}, err
	}

	shipmentRows, err := s.Pool.Query(ctx, `
		SELECT s.id::text, s.sender_household_id::text, s.receiver_household_id::text,
		       CASE WHEN s.sender_household_id = $1::uuid THEN receiver.name ELSE sender.name END,
		       s.resource_code, s.quantity_milli, s.departure_game_day,
		       s.expected_arrival_game_day, s.actual_arrival_game_day, s.status
		FROM shipments s
		JOIN households sender ON sender.id = s.sender_household_id
		JOIN households receiver ON receiver.id = s.receiver_household_id
		WHERE (s.sender_household_id = $1::uuid OR s.receiver_household_id = $1::uuid)
		  AND (s.expected_arrival_game_day BETWEEN $2 AND $3
		       OR s.actual_arrival_game_day BETWEEN $2 AND $3)
		ORDER BY COALESCE(s.actual_arrival_game_day, s.expected_arrival_game_day), s.id
	`, id, from, to)
	if err != nil {
		return port.CalendarContext{}, err
	}
	for shipmentRows.Next() {
		var item port.CalendarShipmentRecord
		if err := shipmentRows.Scan(&item.ID, &item.SenderHouseholdID, &item.ReceiverHouseholdID,
			&item.CounterpartyName, &item.ResourceType, &item.QuantityMilli, &item.DepartureGameDay,
			&item.ExpectedArrivalGameDay, &item.ActualArrivalGameDay, &item.Status); err != nil {
			shipmentRows.Close()
			return port.CalendarContext{}, err
		}
		contextValue.Shipments = append(contextValue.Shipments, item)
	}
	shipmentRows.Close()
	if err := shipmentRows.Err(); err != nil {
		return port.CalendarContext{}, err
	}

	deadlineRows, err := s.Pool.Query(ctx, `
		SELECT d.id::text, d.decision_type, d.expires_game_day
		FROM household_decisions d
		WHERE d.household_id = $1::uuid AND d.status = 'pending'
		ORDER BY d.expires_tick, d.id
	`, id)
	if err != nil {
		return port.CalendarContext{}, err
	}
	for deadlineRows.Next() {
		var item port.CalendarDeadlineRecord
		if err := deadlineRows.Scan(&item.ID, &item.Title, &item.DeadlineGameDay); err != nil {
			deadlineRows.Close()
			return port.CalendarContext{}, err
		}
		item.Kind, item.Category, item.Importance = "political_deadline", "politics", "critical"
		item.Title = "Answer the Jarl"
		contextValue.Deadlines = append(contextValue.Deadlines, item)
	}
	deadlineRows.Close()
	if err := deadlineRows.Err(); err != nil {
		return port.CalendarContext{}, err
	}

	assignmentRows, err := s.Pool.Query(ctx, `
		SELECT id::text, activity_type, ends_tick
		FROM assignments
		WHERE household_id = $1::uuid AND status IN ('planned', 'active')
		ORDER BY ends_tick, id
	`, id)
	if err != nil {
		return port.CalendarContext{}, err
	}
	for assignmentRows.Next() {
		var item port.CalendarAssignmentRecord
		if err := assignmentRows.Scan(&item.ID, &item.Activity, &item.EndsTick); err != nil {
			assignmentRows.Close()
			return port.CalendarContext{}, err
		}
		item.Importance = "context"
		contextValue.Assignments = append(contextValue.Assignments, item)
	}
	assignmentRows.Close()
	if err := assignmentRows.Err(); err != nil {
		return port.CalendarContext{}, err
	}
	return contextValue, nil
}

func ceilTravelDays(ticks, numerator, denominator int64) int64 {
	if ticks <= 0 || numerator <= 0 || denominator <= 0 {
		return 0
	}
	return (ticks*numerator + denominator - 1) / denominator
}

var _ port.CalendarReader = (*Store)(nil)
