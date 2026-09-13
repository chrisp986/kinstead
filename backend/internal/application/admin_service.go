package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"game/backend/internal/port"
)

var ErrInvalidAdminRequest = errors.New("invalid admin request")

type AdminService struct {
	Repo port.AdminRepository
	Now  func() time.Time
}

func NewAdminService(repo port.AdminRepository) *AdminService {
	return &AdminService{Repo: repo, Now: time.Now}
}

func EncodeAdminCursor(cursor port.AdminCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(data)
}

func DecodeAdminCursor(value string) (port.AdminCursor, error) {
	if value == "" {
		return port.AdminCursor{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return port.AdminCursor{}, ErrInvalidAdminRequest
	}
	var cursor port.AdminCursor
	if err := json.Unmarshal(data, &cursor); err != nil || cursor.Name == "" || !validUUID(cursor.ID) {
		return port.AdminCursor{}, ErrInvalidAdminRequest
	}
	return cursor, nil
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, char := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

func (s *AdminService) Worlds(ctx context.Context, cursor string, limit int) ([]port.AdminWorld, string, error) {
	c, err := adminPage(cursor, limit)
	if err != nil {
		return nil, "", err
	}
	items, err := s.Repo.ListAdminWorlds(ctx, c, limit)
	for i := range items {
		decorateAdminWorld(&items[i], s.Now())
	}
	return items, nextAdminCursor(items, func(item port.AdminWorld) port.AdminCursor { return port.AdminCursor{Name: item.Name, ID: item.ID} }), err
}

func (s *AdminService) World(ctx context.Context, id string) (port.AdminWorld, error) {
	if !validUUID(id) {
		return port.AdminWorld{}, ErrInvalidAdminRequest
	}
	item, err := s.Repo.GetAdminWorld(ctx, id)
	if err == nil {
		decorateAdminWorld(&item, s.Now())
	}
	return item, err
}

const adminStaleHeartbeat = 30 * time.Second
const adminDelayedTicks = int64(2)

func decorateAdminWorld(item *port.AdminWorld, now time.Time) {
	item.StatusFacts = []string{}
	if item.TickDurationSeconds <= 0 {
		item.Status = "unknown"
		item.StatusFacts = append(item.StatusFacts, "invalid non-positive tick duration")
		return
	}
	if !item.NextTickAt.After(now) {
		item.DueTickCount = int64(now.Sub(item.NextTickAt)/(time.Duration(item.TickDurationSeconds)*time.Second)) + 1
	}
	item.StatusFacts = append(item.StatusFacts, fmt.Sprintf("due_tick_count=%d", item.DueTickCount))
	if item.DueTickCount == 0 {
		item.Status = "waiting"
		return
	}
	if item.LatestHeartbeat == nil || now.Sub(item.LatestHeartbeat.LastSeenAt) > adminStaleHeartbeat {
		item.Status = "worker_unavailable"
		item.StatusFacts = append(item.StatusFacts, "no heartbeat within 30 seconds")
		return
	}
	if item.DueTickCount > adminDelayedTicks {
		item.Status = "delayed"
		return
	}
	if item.LastCommittedAt != nil {
		item.Status = "catching_up"
		item.StatusFacts = append(item.StatusFacts, "committed progress observed")
		return
	}
	item.Status = "unknown"
	item.StatusFacts = append(item.StatusFacts, "overdue but committed progress is not recorded")
}

func (s *AdminService) Households(ctx context.Context, query, cursor string, limit int) ([]port.AdminHousehold, string, error) {
	if len(query) > 120 {
		return nil, "", ErrInvalidAdminRequest
	}
	c, err := adminPage(cursor, limit)
	if err != nil {
		return nil, "", err
	}
	items, err := s.Repo.ListAdminHouseholds(ctx, query, c, limit)
	return items, nextAdminCursor(items, func(item port.AdminHousehold) port.AdminCursor { return port.AdminCursor{Name: item.Name, ID: item.ID} }), err
}

func (s *AdminService) Household(ctx context.Context, id string) (port.AdminHousehold, error) {
	if !validUUID(id) {
		return port.AdminHousehold{}, ErrInvalidAdminRequest
	}
	return s.Repo.GetAdminHousehold(ctx, id)
}

func (s *AdminService) Ticks(ctx context.Context, id string, before *int64, limit int) ([]port.AdminTickDiagnostic, error) {
	if !validUUID(id) || limit <= 0 || limit > port.AdminPageSizeLimit {
		return nil, ErrInvalidAdminRequest
	}
	tick := int64(-1)
	if before != nil {
		tick = *before
	}
	if tick < -1 {
		return nil, ErrInvalidAdminRequest
	}
	return s.Repo.ListAdminTickDiagnostics(ctx, id, tick, limit)
}

func (s *AdminService) Tick(ctx context.Context, id string, tick int64) (port.AdminTickDiagnostic, error) {
	if !validUUID(id) || tick < 0 {
		return port.AdminTickDiagnostic{}, ErrInvalidAdminRequest
	}
	return s.Repo.GetAdminTickDiagnostic(ctx, id, tick)
}

func (s *AdminService) Errors(ctx context.Context, filter port.AdminErrorFilter, cursor string, limit int) ([]port.AdminError, string, error) {
	c, err := adminPage(cursor, limit)
	if err != nil {
		return nil, "", err
	}
	if filter.WorldID != "" && !validUUID(filter.WorldID) || filter.HouseholdID != "" && !validUUID(filter.HouseholdID) || len(filter.RequestID) > 128 || (filter.Source != "" && filter.Source != "api" && filter.Source != "worker") {
		return nil, "", ErrInvalidAdminRequest
	}
	items, err := s.Repo.ListAdminErrors(ctx, filter, c, limit)
	return items, nextAdminCursor(items, func(item port.AdminError) port.AdminCursor {
		return port.AdminCursor{Name: item.LastObservedAt.UTC().Format(time.RFC3339Nano), ID: item.ID}
	}), err
}

func (s *AdminService) Accounts(ctx context.Context, query, cursor string, limit int) ([]port.AdminAccount, string, error) {
	if len(query) > 120 {
		return nil, "", ErrInvalidAdminRequest
	}
	c, err := adminPage(cursor, limit)
	if err != nil {
		return nil, "", err
	}
	items, err := s.Repo.ListAdminAccounts(ctx, query, c, limit)
	return items, nextAdminCursor(items, func(item port.AdminAccount) port.AdminCursor {
		return port.AdminCursor{Name: item.ExternalAuthSubject, ID: item.PlayerID}
	}), err
}

func (s *AdminService) Account(ctx context.Context, id string) (port.AdminAccount, error) {
	if !validUUID(id) {
		return port.AdminAccount{}, ErrInvalidAdminRequest
	}
	return s.Repo.GetAdminAccount(ctx, id)
}

func (s *AdminService) Sessions(ctx context.Context, id, cursor string, limit int) ([]port.AdminSession, string, error) {
	if !validUUID(id) {
		return nil, "", ErrInvalidAdminRequest
	}
	c, err := adminPage(cursor, limit)
	if err != nil {
		return nil, "", err
	}
	items, err := s.Repo.ListAdminSessions(ctx, id, c, limit)
	return items, nextAdminCursor(items, func(item port.AdminSession) port.AdminCursor {
		return port.AdminCursor{Name: item.CreatedAt.UTC().Format(time.RFC3339Nano), ID: item.ID}
	}), err
}

func adminPage(cursor string, limit int) (port.AdminCursor, error) {
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > port.AdminPageSizeLimit {
		return port.AdminCursor{}, ErrInvalidAdminRequest
	}
	return DecodeAdminCursor(cursor)
}

func nextAdminCursor[T any](items []T, get func(T) port.AdminCursor) string {
	if len(items) < 1 {
		return ""
	}
	return EncodeAdminCursor(get(items[len(items)-1]))
}
