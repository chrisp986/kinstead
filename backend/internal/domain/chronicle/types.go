package chronicle

// Stable production Chronicle fact types. Facts are persisted as structured
// data; clients are responsible for rendering localized prose.
const (
	AssignmentScheduled = "assignment_scheduled"
	AssignmentCompleted = "assignment_completed"

	MarketPurchase = "market_purchase"
	MarketSale     = "market_sale"

	ShipmentArrived   = "shipment_arrived"
	ShipmentCancelled = "shipment_cancelled"

	ContractProposed            = "contract_proposed"
	ContractAccepted            = "contract_accepted"
	ContractRejected            = "contract_rejected"
	ContractShipmentDispatched  = "contract_shipment_dispatched"
	ContractObligationFulfilled = "contract_obligation_fulfilled"
	ContractObligationLate      = "contract_obligation_late"
	ContractObligationBroken    = "contract_obligation_broken"

	PoliticalDemandReceived     = "political_demand_received"
	PoliticalDemandResolved     = "political_demand_resolved"
	PoliticalDemandAutoResolved = "political_demand_auto_resolved"

	EmergencyFoodWorkScheduled = "emergency_food_work_scheduled"
	EmergencyWorkOverridden    = "emergency_work_overridden"
)
