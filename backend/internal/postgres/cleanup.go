//go:build postgres

package postgres

import (
	"context"
	"errors"
	"time"
)

type DeveloperConsoleCleanupResult struct{ TickDiagnostics, OperationalErrors, StaleHeartbeats int64 }

func (s *Store) CleanupDeveloperConsole(ctx context.Context, observationAge, heartbeatAge time.Duration, batch int) (DeveloperConsoleCleanupResult, error) {
	if observationAge <= 0 || heartbeatAge <= 0 || batch <= 0 || batch > 5000 {
		return DeveloperConsoleCleanupResult{}, errors.New("invalid developer-console cleanup bounds")
	}
	var result DeveloperConsoleCleanupResult
	for _, item := range []struct {
		table  string
		cutoff time.Time
		target *int64
	}{
		{"household_tick_diagnostics", time.Now().Add(-observationAge), &result.TickDiagnostics},
		{"operational_errors", time.Now().Add(-observationAge), &result.OperationalErrors},
		{"worker_heartbeats", time.Now().Add(-heartbeatAge), &result.StaleHeartbeats},
	} {
		for {
			// The table names are compile-time constants above; values remain
			// parameterized and each delete is bounded by the caller's batch.
			query := "DELETE FROM " + item.table + " WHERE ctid IN (SELECT ctid FROM " + item.table + " WHERE "
			if item.table == "worker_heartbeats" {
				query += "last_seen_at < $1"
			} else {
				query += "recorded_at < $1"
				if item.table == "operational_errors" {
					query = "DELETE FROM operational_errors WHERE ctid IN (SELECT ctid FROM operational_errors WHERE last_observed_at < $1"
				}
			}
			query += " LIMIT $2)"
			command, err := s.Pool.Exec(ctx, query, item.cutoff, batch)
			if err != nil {
				return result, err
			}
			count := command.RowsAffected()
			*item.target += count
			if count < int64(batch) {
				break
			}
		}
	}
	return result, nil
}
