//go:build postgres

package postgres

import (
	"context"

	"game/backend/internal/port"
)

func (s *Store) RecordOperationalError(ctx context.Context, value port.OperationalErrorRecord) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO operational_errors(source,error_code,message,request_id,worker_instance_id,world_id,household_id,tick,stage,fingerprint)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,NULLIF($7,'')::uuid,$8,NULLIF($9,''),$10)
		ON CONFLICT (fingerprint) DO UPDATE SET
			occurrence_count=operational_errors.occurrence_count+1,
			last_observed_at=now(),message=EXCLUDED.message
	`, value.Source, value.ErrorCode, value.Message, value.RequestID, value.WorkerInstanceID, value.WorldID, value.HouseholdID, value.Tick, value.Stage, value.Fingerprint)
	return err
}
