//go:build postgres

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	politicsdomain "game/backend/internal/domain/politics"
	"game/backend/internal/port"
	sqlcdb "game/backend/internal/postgres/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func normalizeTransactionError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "40001", "40P01":
			return fmt.Errorf("%w: %s", port.ErrConcurrentTransaction, pgErr.Message)
		}
	}
	return err
}

func politicalEvent(row sqlcdb.LoadPoliticalEventsStartingTickRow) port.PoliticalEventRecord {
	expires := int64(0)
	if row.EndsTick.Valid {
		expires = row.EndsTick.Int64
	}
	return port.PoliticalEventRecord{ID: row.ID, WorldID: row.WorldID, LocationID: row.LocationID, PoliticalActorID: row.PoliticalActorID, EventType: row.EventType, StartsTick: row.StartsTick, ExpiresTick: expires, Parameters: row.Parameters, ActorName: row.ActorName, ActorType: row.ActorType}
}
func politicalDecision(row sqlcdb.LoadExpiringPoliticalDecisionsRow) port.PoliticalDecisionRecord {
	var selected *string
	if row.SelectedOption.Valid {
		v := row.SelectedOption.String
		selected = &v
	}
	var delta *int
	if row.StandingDelta.Valid {
		v := int(row.StandingDelta.Int32)
		delta = &v
	}
	return port.PoliticalDecisionRecord{ID: row.ID, HouseholdID: row.HouseholdID, WorldID: row.WorldID, WorldEventID: row.WorldEventID, DecisionType: row.DecisionType, AvailableFromTick: row.AvailableFromTick, ExpiresTick: row.ExpiresTick, Status: row.Status, SelectedOption: selected, StandingDelta: delta, Parameters: row.Parameters, PoliticalActorID: row.PoliticalActorID, EventType: row.EventType}
}

func (t *worldTickTx) LoadPoliticalEventsStartingTick(ctx context.Context, worldID string, tick int64) ([]port.PoliticalEventRecord, error) {
	id, e := uuidParam(worldID)
	if e != nil {
		return nil, e
	}
	rows, e := sqlcdb.New(t.tx).LoadPoliticalEventsStartingTick(ctx, sqlcdb.LoadPoliticalEventsStartingTickParams{WorldID: id, Tick: tick})
	if e != nil {
		return nil, normalizeTransactionError(e)
	}
	out := make([]port.PoliticalEventRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, politicalEvent(r))
	}
	return out, nil
}
func (t *worldTickTx) ListHouseholdsForPoliticalEvent(ctx context.Context, eventID string) ([]string, error) {
	id, e := uuidParam(eventID)
	if e != nil {
		return nil, e
	}
	rows, e := sqlcdb.New(t.tx).ListHouseholdsForPoliticalEvent(ctx, id)
	if e != nil {
		return nil, normalizeTransactionError(e)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.HouseholdID)
	}
	return out, nil
}
func (t *worldTickTx) InsertPoliticalDecision(ctx context.Context, d port.PoliticalDecisionRecord) (bool, error) {
	h, e := uuidParam(d.HouseholdID)
	if e != nil {
		return false, e
	}
	w, e := uuidParam(d.WorldID)
	if e != nil {
		return false, e
	}
	ev, e := uuidParam(d.WorldEventID)
	if e != nil {
		return false, e
	}
	n, e := sqlcdb.New(t.tx).InsertHouseholdDecision(ctx, sqlcdb.InsertHouseholdDecisionParams{HouseholdID: h, WorldID: w, WorldEventID: ev, DecisionType: d.DecisionType, AvailableFromTick: d.AvailableFromTick, ExpiresTick: d.ExpiresTick, Parameters: d.Parameters})
	return n == 1, normalizeTransactionError(e)
}
func (t *worldTickTx) LoadExpiringPoliticalDecisions(ctx context.Context, worldID string, tick int64) ([]port.PoliticalDecisionRecord, error) {
	id, e := uuidParam(worldID)
	if e != nil {
		return nil, e
	}
	rows, e := sqlcdb.New(t.tx).LoadExpiringPoliticalDecisions(ctx, sqlcdb.LoadExpiringPoliticalDecisionsParams{WorldID: id, Tick: tick})
	if e != nil {
		return nil, normalizeTransactionError(e)
	}
	out := make([]port.PoliticalDecisionRecord, 0, len(rows))
	for _, r := range rows {
		d := politicalDecision(r)
		d.CurrentTick = tick
		out = append(out, d)
	}
	return out, nil
}
func (t *worldTickTx) AutoResolvePoliticalDecision(ctx context.Context, d port.PoliticalDecisionRecord, tick int64, option string, delta int) (bool, error) {
	id, e := uuidParam(d.ID)
	if e != nil {
		return false, e
	}
	n, e := sqlcdb.New(t.tx).AutoResolvePoliticalDecision(ctx, sqlcdb.AutoResolvePoliticalDecisionParams{DecisionID: id, SelectedOption: pgtype.Text{String: option, Valid: true}, ResolvedTick: pgtype.Int8{Int64: tick, Valid: true}, StandingDelta: pgtype.Int4{Int32: int32(delta), Valid: true}})
	return n == 1, normalizeTransactionError(e)
}
func (t *worldTickTx) ApplyPoliticalScoreDelta(ctx context.Context, w, h, a string, delta int) error {
	return applyPoliticalScore(ctx, t.tx, w, h, a, delta)
}
func (t *worldTickTx) InsertPoliticalChronicle(ctx context.Context, h string, tick int64, entry, decision, actor, assignment string, data []byte) error {
	return insertPoliticalChronicle(ctx, t.tx, h, tick, entry, decision, actor, assignment, data)
}
func (t *worldTickTx) InsertPoliticalReceivedChronicle(ctx context.Context, h string, tick int64, event, actor string, data []byte) error {
	hi, e := uuidParam(h)
	if e != nil {
		return e
	}
	ei, e := uuidParam(event)
	if e != nil {
		return e
	}
	ai, e := uuidParam(actor)
	if e != nil {
		return e
	}
	return normalizeTransactionError(sqlcdb.New(t.tx).InsertPoliticalReceivedChronicle(ctx, sqlcdb.InsertPoliticalReceivedChronicleParams{HouseholdID: hi, OccurredTick: tick, WorldEventID: ei, ActorID: ai, Data: data}))
}

type politicsTx struct{ tx pgx.Tx }

func (s *Store) BeginPoliticsResponse(ctx context.Context) (port.PoliticsResponseTransaction, error) {
	tx, e := s.Begin(ctx)
	if e != nil {
		return nil, normalizeTransactionError(e)
	}
	return &politicsTx{tx: tx}, nil
}
func (t *politicsTx) LoadPoliticalDecision(ctx context.Context, decision, household string) (port.PoliticalDecisionRecord, error) {
	d, e := uuidParam(decision)
	if e != nil {
		return port.PoliticalDecisionRecord{}, e
	}
	h, e := uuidParam(household)
	if e != nil {
		return port.PoliticalDecisionRecord{}, e
	}
	r, e := sqlcdb.New(t.tx).LockPoliticalDecision(ctx, sqlcdb.LockPoliticalDecisionParams{DecisionID: d, HouseholdID: h})
	if e != nil {
		return port.PoliticalDecisionRecord{}, normalizeTransactionError(e)
	}
	out := port.PoliticalDecisionRecord{ID: r.ID, HouseholdID: r.HouseholdID, WorldID: r.WorldID, WorldEventID: r.WorldEventID, DecisionType: r.DecisionType, AvailableFromTick: r.AvailableFromTick, ExpiresTick: r.ExpiresTick, Status: r.Status, Parameters: r.Parameters, PoliticalActorID: r.PoliticalActorID, EventType: r.EventType, CurrentTick: r.CurrentTick}
	if r.SelectedOption.Valid {
		v := r.SelectedOption.String
		out.SelectedOption = &v
	}
	if r.StandingDelta.Valid {
		v := int(r.StandingDelta.Int32)
		out.StandingDelta = &v
	}
	return out, nil
}
func (t *politicsTx) LoadCharacterForPolitics(ctx context.Context, c, h string) (port.PoliticalCharacterRecord, error) {
	ci, e := uuidParam(c)
	if e != nil {
		return port.PoliticalCharacterRecord{}, e
	}
	hi, e := uuidParam(h)
	if e != nil {
		return port.PoliticalCharacterRecord{}, e
	}
	r, e := sqlcdb.New(t.tx).LoadPoliticalCharacter(ctx, sqlcdb.LoadPoliticalCharacterParams{CharacterID: ci, HouseholdID: hi})
	if e != nil {
		return port.PoliticalCharacterRecord{}, normalizeTransactionError(e)
	}
	return port.PoliticalCharacterRecord{ID: r.ID, HouseholdID: r.HouseholdID, Status: r.Status, LaborCapacityMilli: int64(r.LaborCapacityMilli)}, nil
}
func (t *politicsTx) ResourceQuantity(ctx context.Context, h, r string) (int64, error) {
	hi, e := uuidParam(h)
	if e != nil {
		return 0, e
	}
	quantity, err := sqlcdb.New(t.tx).LockResourceStock(ctx, sqlcdb.LockResourceStockParams{HouseholdID: hi, ResourceCode: r})
	return quantity, normalizeTransactionError(err)
}
func (t *politicsTx) DeductResource(ctx context.Context, h, r string, a int64) error {
	hi, e := uuidParam(h)
	if e != nil {
		return e
	}
	q, e := sqlcdb.New(t.tx).LockResourceStock(ctx, sqlcdb.LockResourceStockParams{HouseholdID: hi, ResourceCode: r})
	if e != nil {
		return normalizeTransactionError(e)
	}
	if q < a {
		return fmt.Errorf("%w: %s", politicsdomain.ErrInsufficientResources, r)
	}
	return normalizeTransactionError(sqlcdb.New(t.tx).DeductResourceStock(ctx, sqlcdb.DeductResourceStockParams{HouseholdID: hi, ResourceCode: r, Amount: a}))
}
func (t *politicsTx) AssignmentOverlaps(ctx context.Context, c string, start, end int64) (bool, error) {
	ci, e := uuidParam(c)
	if e != nil {
		return false, e
	}
	overlap, err := sqlcdb.New(t.tx).AssignmentOverlaps(ctx, sqlcdb.AssignmentOverlapsParams{CharacterID: ci, StartsTick: start, EndsTick: end})
	return overlap, normalizeTransactionError(err)
}
func (t *politicsTx) CreateRulerServiceAssignment(ctx context.Context, h, c string, start, end int64, decision string) (string, error) {
	hi, e := uuidParam(h)
	if e != nil {
		return "", e
	}
	ci, e := uuidParam(c)
	if e != nil {
		return "", e
	}
	meta, _ := json.Marshal(map[string]string{"source": "political_demand", "decision_id": decision})
	assignment, err := sqlcdb.New(t.tx).CreateRulerServiceAssignment(ctx, sqlcdb.CreateRulerServiceAssignmentParams{HouseholdID: hi, CharacterID: ci, StartsTick: start, EndsTick: end, Metadata: meta})
	return assignment, normalizeTransactionError(err)
}
func (t *politicsTx) ResolvePoliticalDecision(ctx context.Context, d, opt string, tick int64, delta int, assignment string) (bool, error) {
	id, e := uuidParam(d)
	if e != nil {
		return false, e
	}
	n, e := sqlcdb.New(t.tx).ResolvePoliticalDecision(ctx, sqlcdb.ResolvePoliticalDecisionParams{DecisionID: id, SelectedOption: pgtype.Text{String: opt, Valid: true}, ResolvedTick: pgtype.Int8{Int64: tick, Valid: true}, StandingDelta: pgtype.Int4{Int32: int32(delta), Valid: true}, AssignmentID: assignment})
	return n == 1, normalizeTransactionError(e)
}
func (t *politicsTx) ApplyPoliticalScoreDelta(ctx context.Context, w, h, a string, delta int) error {
	return applyPoliticalScore(ctx, t.tx, w, h, a, delta)
}
func (t *politicsTx) InsertPoliticalChronicle(ctx context.Context, h string, tick int64, entry, decision, actor, assignment string, data []byte) error {
	return insertPoliticalChronicle(ctx, t.tx, h, tick, entry, decision, actor, assignment, data)
}
func (t *politicsTx) InsertPoliticalReceivedChronicle(ctx context.Context, h string, tick int64, event, actor string, data []byte) error {
	return fmt.Errorf("received chronicle is tick-owned")
}
func (t *politicsTx) Commit(ctx context.Context) error {
	return normalizeTransactionError(t.tx.Commit(ctx))
}
func (t *politicsTx) Rollback(ctx context.Context) error { return t.tx.Rollback(ctx) }

func applyPoliticalScore(ctx context.Context, tx pgx.Tx, w, h, a string, delta int) error {
	wi, e := uuidParam(w)
	if e != nil {
		return e
	}
	hi, e := uuidParam(h)
	if e != nil {
		return e
	}
	ai, e := uuidParam(a)
	if e != nil {
		return e
	}
	return normalizeTransactionError(sqlcdb.New(tx).ApplyPoliticalScoreDelta(ctx, sqlcdb.ApplyPoliticalScoreDeltaParams{WorldID: wi, HouseholdID: hi, PoliticalActorID: ai, ScoreDelta: delta}))
}
func insertPoliticalChronicle(ctx context.Context, tx pgx.Tx, h string, tick int64, entry, decision, actor, assignment string, data []byte) error {
	hi, e := uuidParam(h)
	if e != nil {
		return e
	}
	di, e := uuidParam(decision)
	if e != nil {
		return e
	}
	ai, e := uuidParam(actor)
	if e != nil {
		return e
	}
	_, e = sqlcdb.New(tx).InsertPoliticalChronicle(ctx, sqlcdb.InsertPoliticalChronicleParams{HouseholdID: hi, OccurredTick: tick, EntryType: entry, DecisionID: di, ActorID: ai, AssignmentID: assignment, Data: data})
	return normalizeTransactionError(e)
}

func (s *Store) GetHouseholdPolitics(ctx context.Context, householdID string) (port.HouseholdPoliticsProjection, error) {
	id, e := uuidParam(householdID)
	if e != nil {
		return port.HouseholdPoliticsProjection{}, e
	}
	exists, e := sqlcdb.New(s.Pool).HouseholdExists(ctx, id)
	if e != nil {
		return port.HouseholdPoliticsProjection{}, e
	}
	if !exists {
		return port.HouseholdPoliticsProjection{}, pgx.ErrNoRows
	}
	r, e := sqlcdb.New(s.Pool).ListPoliticalRelationshipsForHousehold(ctx, id)
	if e != nil {
		return port.HouseholdPoliticsProjection{}, e
	}
	d, e := sqlcdb.New(s.Pool).ListPoliticalDecisionsForHousehold(ctx, id)
	if e != nil {
		return port.HouseholdPoliticsProjection{}, e
	}
	out := port.HouseholdPoliticsProjection{Relationships: make([]port.PoliticalRelationshipRecord, 0), Decisions: make([]port.PoliticalDecisionProjection, 0)}
	for _, v := range r {
		out.Relationships = append(out.Relationships, port.PoliticalRelationshipRecord{PoliticalActorID: v.PoliticalActorID, ActorName: v.ActorName, ActorType: v.ActorType, Score: int(v.Standing), Standing: string(politicsdomain.DeriveStanding(politicsdomain.Score(v.Standing))), UpdatedAt: v.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00")})
	}
	for _, v := range d {
		var sel *string
		if v.SelectedOption.Valid {
			x := v.SelectedOption.String
			sel = &x
		}
		var sd *int
		if v.StandingDelta.Valid {
			x := int(v.StandingDelta.Int32)
			sd = &x
		}
		demand := politicsdomain.DemandType(v.DecisionType)
		terms := politicsdomain.DefaultTerms(demand)
		if len(v.Parameters) != 0 {
			if err := json.Unmarshal(v.Parameters, &terms); err != nil {
				return port.HouseholdPoliticsProjection{}, err
			}
		}
		if err := terms.Validate(demand); err != nil {
			return port.HouseholdPoliticsProjection{}, err
		}
		options := make([]port.PoliticalOption, 0)
		for _, o := range politicsdomain.AvailableOptions(demand) {
			resolution, _ := politicsdomain.ResolveChoiceWithTerms(demand, o, terms)
			options = append(options, port.PoliticalOption{Code: string(o), ResourceCode: resolution.ResourceCode, ResourceMilli: resolution.ResourceMilli, StandingDelta: resolution.StandingDelta, ServiceTicks: resolution.ServiceTicks, RequiresCharacter: resolution.RequiresWorker})
		}
		params := map[string]any{}
		_ = json.Unmarshal(v.Parameters, &params)
		projection := port.PoliticalDecisionProjection{ID: v.ID, DemandType: v.DecisionType, Status: v.Status, ActorID: v.PoliticalActorID, ActorName: v.ActorName, ActorType: v.ActorType, AvailableFromTick: v.AvailableFromTick, ExpiresTick: v.ExpiresTick, SelectedOption: sel, StandingDelta: sd, Parameters: params, Options: options}
		if demand == politicsdomain.DemandLaborService && v.Status == string(politicsdomain.StatusPending) {
			did, _ := uuidParam(v.ID)
			eligible, ee := sqlcdb.New(s.Pool).ListEligiblePoliticalCharacters(ctx, did)
			if ee != nil {
				return port.HouseholdPoliticsProjection{}, ee
			}
			for _, c := range eligible {
				projection.EligibleCharacters = append(projection.EligibleCharacters, port.PoliticalServiceCandidate{
					ID:            c.ID,
					Name:          c.Name,
					LaborPermille: int64(c.LaborCapacityMilli),
				})
			}
		}
		out.Decisions = append(out.Decisions, projection)
		found := false
		for _, rel := range out.Relationships {
			if rel.PoliticalActorID == v.PoliticalActorID {
				found = true
				break
			}
		}
		if !found {
			out.Relationships = append(out.Relationships, port.PoliticalRelationshipRecord{PoliticalActorID: v.PoliticalActorID, ActorName: v.ActorName, ActorType: v.ActorType, Score: 0, Standing: "neutral"})
		}
	}
	return out, nil
}
