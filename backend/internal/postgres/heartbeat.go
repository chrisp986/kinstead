//go:build postgres

package postgres

import (
	"context"
	"time"
)

func (s *Store) UpsertWorkerHeartbeat(ctx context.Context, instanceID string, startedAt, lastSeenAt time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO worker_heartbeats(instance_id,started_at,last_seen_at)
		VALUES ($1::uuid,$2,$3)
		ON CONFLICT (instance_id) DO UPDATE SET last_seen_at=EXCLUDED.last_seen_at
	`, instanceID, startedAt, lastSeenAt)
	return err
}
