# Developer console implementation checklist

Scope: protected, read-only operator console at `/admin`. This checklist is
repository-local progress tracking and is not an authorization mechanism.

## Stages

- [x] 1. Administrator authorization, membership migration, provisioning CLI.
- [x] 2. Consistent world and household inspection projections.
- [x] 3. Worker heartbeat and world health.
- [x] 4. Tick diagnostics and resource accounting (tracked balances are validated; see limitation).
- [x] 5. Request correlation and operational errors (API internal failures and worker aggregation).
- [x] 6. Safe account and session inspection.
- [x] 7. OpenAPI-generated frontend integration, documentation, and browser E2E coverage.

## Baseline (2026-09-13)

- Commit: `8a570f46b7706412109647e328f90cf4a57c7bd8`.
- Worktree: clean at discovery.
- `cd backend && env GOCACHE=/tmp/kinstead-go-cache go test ./...`: passed.
- `cd frontend && npm run check`: passed, 0 errors and 0 warnings.
- `cd frontend && npm run lint`: passed.
- PostgreSQL-tagged and prepared-database integration checks: passed against local `game_test`.

## Stage verification log

- Stage 1: `cd backend && env GOCACHE=/tmp/kinstead-go-cache go test -tags postgres ./internal/httpapi ./internal/postgres ./cmd/admin`: passed.
- Stage 2/3: `cd backend && env GOCACHE=/tmp/kinstead-go-cache go test -tags postgres ./...`: passed; local migrations 000024–000027 applied successfully.
- API generation: `cd frontend && npm run generate:api`: passed.
- Frontend integration: `cd frontend && npm run check`: passed after admin route integration.
- Local PostgreSQL tagged integration run: passed after creating and migrating local `game_test`.
- `cd frontend && npm run test:e2e`: the first run was blocked by missing Chromium libraries; with the prepared local Playwright libraries, the final full suite passed all 9 tests.

## Known limitations

Tick diagnostics are persisted atomically with gameplay and include opening/
closing stocks, shipment arrivals, production, actual/requested consumption,
pending output, and simulation-produced character outcomes. Tracked stock and
pending balances are validated before persistence. They are labeled `partial`
because politics and any future non-stock stage attribution are not currently
represented as signed resource adjustments. No balancing value is invented to
force an equation to balance.

## Verification log

Commands and stage results are appended as implementation proceeds. Do not
mark a stage complete until its endpoint, generated types where applicable,
and focused tests are present.
