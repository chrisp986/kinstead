package port

import "context"

type RelationshipEventRecord struct {
	ID                  string         `json:"id"`
	EventType           string         `json:"event_type"`
	TrustDelta          int            `json:"trust_delta"`
	OccurredTick        int64          `json:"occurred_tick"`
	OccurredGameDay     int64          `json:"occurred_game_day"`
	RelatedContractID   *string        `json:"related_contract_id,omitempty"`
	RelatedShipmentID   *string        `json:"related_shipment_id,omitempty"`
	RelatedObligationID *string        `json:"related_obligation_id,omitempty"`
	Data                map[string]any `json:"data"`
}

type RelationshipRecord struct {
	WorldID              string                    `json:"world_id"`
	SourceHouseholdID    string                    `json:"source_household_id"`
	SourceHouseholdName  string                    `json:"source_household_name"`
	TargetHouseholdID    string                    `json:"target_household_id"`
	TargetHouseholdName  string                    `json:"target_household_name"`
	Trust                int                       `json:"trust"`
	FirstInteractionTick *int64                    `json:"first_interaction_tick,omitempty"`
	Events               []RelationshipEventRecord `json:"events"`
}

type RelationshipReader interface {
	ListRelationshipsForHousehold(context.Context, string) ([]RelationshipRecord, error)
}
