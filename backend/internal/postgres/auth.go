//go:build postgres

package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"
)

// IssuePlayerSession is an operator-only provisioning operation. Raw session
// secrets are returned once; only their SHA-256 digest is stored in PostgreSQL.
func (s *Store) IssuePlayerSession(ctx context.Context, playerID string, lifetime time.Duration) (string, error) {
	if lifetime <= 0 || lifetime > 30*24*time.Hour {
		return "", errors.New("invalid session lifetime")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	_, err := s.Pool.Exec(ctx, `INSERT INTO player_sessions(token_hash,player_id,expires_at) VALUES($1,$2::uuid,now()+$3::bigint*interval '1 second')`, hash[:], playerID, int64(lifetime/time.Second))
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) AuthenticateSession(ctx context.Context, token string) (string, error) {
	hash := sha256.Sum256([]byte(token))
	var playerID string
	err := s.Pool.QueryRow(ctx, `SELECT player_id::text FROM player_sessions WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>now()`, hash[:]).Scan(&playerID)
	return playerID, err
}

func (s *Store) PlayerOwnsHousehold(ctx context.Context, playerID, householdID string) (bool, error) {
	if _, err := uuidParam(householdID); err != nil {
		return false, nil
	}
	var owns bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM households WHERE id=$1::uuid AND owner_player_id=$2::uuid)`, householdID, playerID).Scan(&owns)
	return owns, err
}

func (s *Store) PlayerHasWorld(ctx context.Context, playerID, worldID string) (bool, error) {
	if _, err := uuidParam(worldID); err != nil {
		return false, nil
	}
	var owns bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM households WHERE world_id=$1::uuid AND owner_player_id=$2::uuid)`, worldID, playerID).Scan(&owns)
	return owns, err
}

type OwnedHousehold struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s *Store) ListOwnedHouseholds(ctx context.Context, playerID string) ([]OwnedHousehold, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id::text,name FROM households WHERE owner_player_id=$1::uuid ORDER BY name,id`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []OwnedHousehold{}
	for rows.Next() {
		var h OwnedHousehold
		if err := rows.Scan(&h.ID, &h.Name); err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, rows.Err()
}
