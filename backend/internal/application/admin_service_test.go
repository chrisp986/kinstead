package application

import (
	"context"
	"testing"
	"time"

	"game/backend/internal/port"
)

type adminRepoStub struct{ worlds []port.AdminWorld }

func (s adminRepoStub) ListAdminWorlds(context.Context, port.AdminCursor, int) ([]port.AdminWorld, error) {
	return s.worlds, nil
}
func (adminRepoStub) GetAdminWorld(context.Context, string) (port.AdminWorld, error) {
	return port.AdminWorld{}, nil
}
func (adminRepoStub) ListAdminHouseholds(context.Context, string, port.AdminCursor, int) ([]port.AdminHousehold, error) {
	return nil, nil
}
func (adminRepoStub) GetAdminHousehold(context.Context, string) (port.AdminHousehold, error) {
	return port.AdminHousehold{}, nil
}
func (adminRepoStub) ListAdminTickDiagnostics(context.Context, string, int64, int) ([]port.AdminTickDiagnostic, error) {
	return nil, nil
}
func (adminRepoStub) GetAdminTickDiagnostic(context.Context, string, int64) (port.AdminTickDiagnostic, error) {
	return port.AdminTickDiagnostic{}, nil
}
func (adminRepoStub) ListAdminErrors(context.Context, port.AdminErrorFilter, port.AdminCursor, int) ([]port.AdminError, error) {
	return nil, nil
}
func (adminRepoStub) ListAdminAccounts(context.Context, string, port.AdminCursor, int) ([]port.AdminAccount, error) {
	return nil, nil
}
func (adminRepoStub) GetAdminAccount(context.Context, string) (port.AdminAccount, error) {
	return port.AdminAccount{}, nil
}
func (adminRepoStub) ListAdminSessions(context.Context, string, port.AdminCursor, int) ([]port.AdminSession, error) {
	return nil, nil
}

func TestAdminWorldStatusUsesInjectedTimeAndFacts(t *testing.T) {
	now := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	world := port.AdminWorld{ID: "00000000-0000-0000-0000-000000000001", Name: "Test", TickDurationSeconds: 3600, NextTickAt: now.Add(-time.Hour), LastCommittedAt: ptrTime(now.Add(-30 * time.Minute)), LatestHeartbeat: &port.AdminHeartbeat{LastSeenAt: now}}
	service := NewAdminService(adminRepoStub{worlds: []port.AdminWorld{world}})
	service.Now = func() time.Time { return now }
	items, _, err := service.Worlds(context.Background(), "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Status != "catching_up" || items[0].DueTickCount != 2 {
		t.Fatalf("world=%+v", items[0])
	}
}

func ptrTime(value time.Time) *time.Time { return &value }
