# Project Documentation

Canonical project context for coding agents and contributors.

Read in this order:

1.  `AGENTS.md` --- implementation constraints
2.  `docs/project-spec.md` --- product/game specification
3.  `docs/architecture.md` --- technical architecture
4.  `docs/balance-v0.3.md` --- frozen deterministic balance baseline
5.  `docs/roadmap.md` --- implementation sequence

For Codex tasks, use:

> Read `AGENTS.md` and the relevant files under `docs/` before making
> changes. Treat them as authoritative unless the task explicitly
> overrides them. Implement only the requested milestone and keep tests
> passing.

## Local development

From the repository root, start PostgreSQL, the backend API, the worker, and
the SvelteKit frontend together:

```bash
# Recommended playtest environment: 60 seconds/tick (the default)
./scripts/dev.sh playtest

# Normal development world: 4 hours/tick
./scripts/dev.sh normal

# Fast debugging: 15 seconds/tick
./scripts/dev.sh fast

# Custom wall-clock duration for legacy development profiles
./scripts/dev.sh 30

# Recreate the disposable local database first
./scripts/dev.sh playtest --reset
```

Without `--reset`, the existing Bjornvik world and its state are preserved.
Duration overrides continue to apply to older development profiles; the
`monthly_seasons_v1` seed always remains at one 3,600-second tick and keeps its
anchored due-time sequence. With `--reset`, the local PostgreSQL volume is
recreated and the development seed is applied from scratch. PostgreSQL remains
running when the launcher exits.

Tick speed changes only real-world scheduling. It does not change historical
days per tick, historical dates/seasons, or the frozen v0.3 balancing calendar.

The household Calendar screen is the player-facing planning view. `Upcoming`
groups deadlines and events by urgency and season. Monthly-season worlds show
the current season, its remaining days, and the next seasonal boundary without
presenting modern dates as the historical year. Older worlds retain their
year-cycle view.

Local launcher requirements:

- Linux or WSL
- Bash >= 5.1
- Docker with Docker Compose v2
- Go, Node/npm, and curl
- `setsid` from util-linux

The launcher uses Bash process groups for cleanup and is not currently
supported by stock macOS Bash.
