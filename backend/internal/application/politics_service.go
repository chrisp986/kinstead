package application

import (
	"context"
	"encoding/json"
	"fmt"

	"game/backend/internal/domain/politics"
	"game/backend/internal/port"
)

type RespondPoliticalDemandCommand struct{ DecisionID, HouseholdID, Option, CharacterID string }

type PoliticsService struct {
	Store  port.PoliticsRepository
	Reader port.PoliticsReader
}

func NewPoliticsService(store port.PoliticsRepository, reader port.PoliticsReader) *PoliticsService {
	return &PoliticsService{Store: store, Reader: reader}
}

func (s *PoliticsService) GetHouseholdPolitics(ctx context.Context, householdID string) (port.HouseholdPoliticsProjection, error) {
	return s.Reader.GetHouseholdPolitics(ctx, householdID)
}

func (s *PoliticsService) Respond(ctx context.Context, cmd RespondPoliticalDemandCommand) error {
	tx, e := s.Store.BeginPoliticsResponse(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	d, e := tx.LoadPoliticalDecision(ctx, cmd.DecisionID, cmd.HouseholdID)
	if e != nil {
		return e
	}
	if d.Status != string(politics.StatusPending) {
		return fmt.Errorf("political demand is already resolved")
	}
	if err := politics.ResponseAllowed(d.CurrentTick, d.ExpiresTick); err != nil {
		return err
	}
	demand := politics.DemandType(d.EventType)
	r, e := politics.ResolveChoice(demand, politics.Option(cmd.Option))
	if e != nil {
		return e
	}
	var assignment string
	if r.RequiresWorker {
		if cmd.CharacterID == "" {
			return fmt.Errorf("serve requires a character")
		}
		c, e := tx.LoadCharacterForPolitics(ctx, cmd.CharacterID, cmd.HouseholdID)
		if e != nil {
			return e
		}
		if c.Status != "active" || c.LaborCapacityMilli != 1000 {
			return fmt.Errorf("character is not eligible for service")
		}
		start, end := d.ExpiresTick, d.ExpiresTick+r.ServiceTicks-1
		overlap, e := tx.AssignmentOverlaps(ctx, cmd.CharacterID, start, end)
		if e != nil {
			return e
		}
		if overlap {
			return fmt.Errorf("service assignment overlaps existing work")
		}
		assignment, e = tx.CreateRulerServiceAssignment(ctx, cmd.HouseholdID, cmd.CharacterID, start, end, cmd.DecisionID)
		if e != nil {
			return e
		}
	} else if r.ResourceCode != "" {
		if e := tx.DeductResource(ctx, cmd.HouseholdID, r.ResourceCode, r.ResourceMilli); e != nil {
			return e
		}
	}
	resolved, e := tx.ResolvePoliticalDecision(ctx, cmd.DecisionID, cmd.Option, d.CurrentTick, r.StandingDelta, assignment)
	if e != nil {
		return e
	}
	if !resolved {
		return fmt.Errorf("political demand changed during response")
	}
	if e := tx.ApplyPoliticalScoreDelta(ctx, d.WorldID, cmd.HouseholdID, d.PoliticalActorID, r.StandingDelta); e != nil {
		return e
	}
	data, _ := json.Marshal(map[string]any{"actor_id": d.PoliticalActorID, "demand_type": d.EventType, "selected_option": cmd.Option, "standing_delta": r.StandingDelta, "resource_code": r.ResourceCode, "resource_milli": r.ResourceMilli, "service_character_id": cmd.CharacterID, "service_assignment_id": assignment, "deadline_tick": d.ExpiresTick})
	if e := tx.InsertPoliticalChronicle(ctx, cmd.HouseholdID, d.CurrentTick, "political_demand_resolved", cmd.DecisionID, d.PoliticalActorID, assignment, data); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
