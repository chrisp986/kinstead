import { createServer } from 'node:http';

const householdId = '00000000-0000-0000-0000-000000000020';
const sellerId = '00000000-0000-0000-0000-000000000021';
const worldId = '00000000-0000-0000-0000-000000000001';
const offerId = '00000000-0000-0000-0000-000000000302';
const assignments = [];
const chronicleEntries = [
	{
		id: '00000000-0000-0000-0000-000000000501',
		occurred_tick: 0,
		entry_type: 'assignment_completed',
		subject_character_id: '00000000-0000-0000-0000-000000000101',
		subject_character_name: 'Bjorn',
		data: { activity: 'agriculture', starts_tick: 0, ends_tick: 0 }
	}
];
let offerQuantity = 10_000;
const contractId = '00000000-0000-0000-0000-000000000601';
const obligationId = '00000000-0000-0000-0000-000000000602';
const contracts = [
	{
		id: contractId,
		world_id: worldId,
		party_a_household_id: sellerId,
		party_b_household_id: householdId,
		starts_tick: 2,
		ends_tick: 2,
		interval_ticks: 1,
		status: 'proposed',
		terms: [
			{
				debtor_household_id: householdId,
				creditor_household_id: sellerId,
				resource_type: 'wood',
				quantity_milli: 5_000
			}
		],
		obligations: []
	}
];
const relationships = [
	{
		world_id: worldId,
		source_household_id: sellerId,
		source_household_name: 'Hrafnstead',
		target_household_id: householdId,
		target_household_name: 'Bjornvik',
		trust: -7,
		standing: 'neutral',
		events: [
			{
				id: '00000000-0000-0000-0000-000000000701',
				event_type: 'contract_obligation_fulfilled',
				trust_delta: 2,
				occurred_tick: 10,
				data: { due_arrival_tick: 10, actual_fulfillment_tick: 10 }
			},
			{
				id: '00000000-0000-0000-0000-000000000702',
				event_type: 'contract_obligation_late',
				trust_delta: -1,
				occurred_tick: 21,
				data: { due_arrival_tick: 20, actual_fulfillment_tick: 21 }
			},
			{
				id: '00000000-0000-0000-0000-000000000703',
				event_type: 'contract_obligation_broken',
				trust_delta: -8,
				occurred_tick: 35,
				data: { due_arrival_tick: 32 }
			}
		]
	}
];

function shipment(id, quantity) {
	return {
		id,
		world_id: worldId,
		sender_household_id: sellerId,
		receiver_household_id: householdId,
		origin_location_id: '00000000-0000-0000-0000-000000000011',
		destination_location_id: '00000000-0000-0000-0000-000000000010',
		resource_type: 'provisions',
		quantity_milli: quantity,
		departure_tick: 0,
		expected_arrival_tick: 2,
		transport_cost_milli: 0,
		status: 'in_transit'
	};
}

const shipments = [shipment('00000000-0000-0000-0000-000000000301', 30_000)];
const characters = [
	{
		id: '00000000-0000-0000-0000-000000000101',
		name: 'Bjorn',
		labor_permille: 1000,
		fatigue: 12,
		specialization: 'agriculture'
	},
	{
		id: '00000000-0000-0000-0000-000000000102',
		name: 'Astrid',
		labor_permille: 1000,
		fatigue: 8,
		specialization: 'fishing'
	},
	{ id: '00000000-0000-0000-0000-000000000105', name: 'Sven', labor_permille: 0, fatigue: 0 }
];
const politicalResponses = new Map();

function send(response, status, body) {
	response.writeHead(status, { 'content-type': 'application/json' });
	response.end(JSON.stringify(body));
}

async function readBody(request) {
	const chunks = [];
	for await (const chunk of request) chunks.push(chunk);
	return JSON.parse(Buffer.concat(chunks).toString('utf8'));
}

createServer(async (request, response) => {
	const url = new URL(request.url ?? '/', 'http://127.0.0.1:9080');
	if (request.method === 'GET' && url.pathname === `/api/households/${householdId}/report`) {
		return send(response, 200, {
			household_id: householdId,
			household_name: 'Bjornvik',
			world_id: worldId,
			tick: 0,
			historical_date: '0980-01-01',
			season: 'winter',
			supply_days: 30.6,
			resources: { provisions: 150, wood: 20, trade_goods: 4, silver: 30 },
			characters,
			assignments,
			alerts: [],
			change_window: { from_tick: 0, to_tick: 0 },
			recent_changes: chronicleEntries,
			attention: [
				{
					code: 'political_demand_due',
					severity: 'critical',
					target: 'politics',
					related_id: '00000000-0000-0000-0000-000000000801',
					due_tick: 5,
					data: { actor_name: 'Jarl Eirik' }
				}
			],
			decisions: [
				{
					code: 'respond_political_demand',
					severity: 'critical',
					target: 'politics',
					related_id: '00000000-0000-0000-0000-000000000801',
					due_tick: 5,
					data: { actor_name: 'Jarl Eirik' }
				},
				{
					code: 'dispatch_contract_obligation',
					severity: 'warning',
					target: 'contracts',
					related_id: obligationId,
					due_tick: 2,
					data: { resource_type: 'wood', quantity_milli: 5000 }
				},
				{
					code: 'secure_provisions',
					severity: 'critical',
					target: 'trade',
					data: { supply_days: 6.5 }
				}
			]
		});
	}
	if (request.method === 'GET' && url.pathname === `/api/households/${householdId}/shipments`) {
		return send(response, 200, { shipments });
	}
	if (request.method === 'GET' && url.pathname === `/api/households/${householdId}/chronicle`) {
		return send(response, 200, { entries: chronicleEntries });
	}
	if (request.method === 'GET' && url.pathname === `/api/households/${householdId}/contracts`) {
		return send(response, 200, { contracts });
	}
	if (request.method === 'GET' && url.pathname === `/api/households/${householdId}/relationships`) {
		return send(response, 200, { relationships });
	}
	if (request.method === 'GET' && url.pathname === `/api/households/${householdId}/politics`) {
		return send(response, 200, {
			relationships: [],
			decisions: [
				{
					id: '00000000-0000-0000-0000-000000000801',
					demand_type: 'political_levy',
					status:
						politicalResponses.get('00000000-0000-0000-0000-000000000801')?.status ?? 'pending',
					selected_option: politicalResponses.get('00000000-0000-0000-0000-000000000801')
						?.selected_option,
					actor_id: '00000000-0000-0000-0000-000000000901',
					actor_name: 'Jarl Eirik',
					actor_type: 'jarl',
					available_from_tick: 1,
					expires_tick: 5,
					parameters: {},
					options: [
						{ code: 'pay_wood', resource_code: 'wood', resource_milli: 18000, standing_delta: 10 },
						{
							code: 'pay_silver',
							resource_code: 'silver',
							resource_milli: 6000,
							standing_delta: 10
						},
						{ code: 'refuse', standing_delta: -5 }
					]
				},
				{
					id: '00000000-0000-0000-0000-000000000802',
					demand_type: 'political_levy',
					status:
						politicalResponses.get('00000000-0000-0000-0000-000000000802')?.status ?? 'pending',
					selected_option: politicalResponses.get('00000000-0000-0000-0000-000000000802')
						?.selected_option,
					actor_id: '00000000-0000-0000-0000-000000000901',
					actor_name: 'Jarl Eirik',
					actor_type: 'jarl',
					available_from_tick: 1,
					expires_tick: 5,
					parameters: {},
					options: [
						{ code: 'pay_wood', resource_code: 'wood', resource_milli: 13000, standing_delta: 7 },
						{
							code: 'pay_silver',
							resource_code: 'silver',
							resource_milli: 4000,
							standing_delta: 7
						},
						{ code: 'refuse', standing_delta: -3 }
					]
				},
				{
					id: '00000000-0000-0000-0000-000000000803',
					demand_type: 'political_labor_service',
					status:
						politicalResponses.get('00000000-0000-0000-0000-000000000803')?.status ?? 'pending',
					selected_option: politicalResponses.get('00000000-0000-0000-0000-000000000803')
						?.selected_option,
					actor_id: '00000000-0000-0000-0000-000000000901',
					actor_name: 'Jarl Eirik',
					actor_type: 'jarl',
					available_from_tick: 1,
					expires_tick: 5,
					parameters: {},
					eligible_characters: [
						{ id: characters[0].id, name: characters[0].name, labor_permille: 1000 }
					],
					options: [
						{ code: 'serve', service_ticks: 6, standing_delta: 7, requires_character: true },
						{ code: 'refuse', standing_delta: -3 }
					]
				},
				{
					id: '00000000-0000-0000-0000-000000000804',
					demand_type: 'political_labor_service',
					status:
						politicalResponses.get('00000000-0000-0000-0000-000000000804')?.status ?? 'pending',
					selected_option: politicalResponses.get('00000000-0000-0000-0000-000000000804')
						?.selected_option,
					actor_id: '00000000-0000-0000-0000-000000000901',
					actor_name: 'Jarl Eirik',
					actor_type: 'jarl',
					available_from_tick: 1,
					expires_tick: 5,
					parameters: {},
					eligible_characters: [],
					options: [
						{ code: 'serve', service_ticks: 6, standing_delta: 7, requires_character: true },
						{ code: 'refuse', standing_delta: -3 }
					]
				}
			]
		});
	}
	if (request.method === 'POST' && url.pathname.startsWith('/api/political-demands/')) {
		const body = await readBody(request);
		if (body.option === 'serve' && !body.character_id) {
			return send(response, 409, { error: 'political_demand_conflict' });
		}
		const decisionId = url.pathname.split('/').pop();
		politicalResponses.set(decisionId, { status: 'resolved', selected_option: body.option });
		return send(response, 200, { status: 'resolved' });
	}
	if (request.method === 'GET' && url.pathname === '/api/market/offers') {
		const offers =
			offerQuantity > 0
				? [
						{
							id: offerId,
							world_id: worldId,
							seller_household_id: sellerId,
							origin_location_id: '00000000-0000-0000-0000-000000000011',
							resource_type: 'provisions',
							quantity_remaining_milli: offerQuantity,
							price_per_unit_milli: 1500,
							created_tick: 0,
							expires_tick: 12,
							status: 'active'
						}
					]
				: [];
		return send(response, 200, { offers });
	}
	if (request.method === 'POST' && url.pathname === `/api/households/${householdId}/assignments`) {
		const body = await readBody(request);
		const character = characters.find((value) => value.id === body.character_id);
		const assignment = {
			id: '00000000-0000-0000-0000-000000000401',
			character_id: body.character_id,
			character: character?.name ?? 'Unknown',
			activity: body.activity,
			intensity: body.intensity,
			starts_tick: 1,
			ends_tick: body.duration_ticks,
			status: 'planned'
		};
		assignments.push(assignment);
		chronicleEntries.unshift({
			id: '00000000-0000-0000-0000-000000000502',
			occurred_tick: 0,
			entry_type: 'assignment_scheduled',
			subject_character_id: body.character_id,
			subject_character_name: character?.name,
			related_assignment_id: assignment.id,
			data: {
				activity: body.activity,
				intensity: body.intensity,
				starts_tick: assignment.starts_tick,
				ends_tick: assignment.ends_tick
			}
		});
		return send(response, 201, assignment);
	}
	if (request.method === 'POST' && url.pathname === `/api/market/offers/${offerId}/purchase`) {
		const body = await readBody(request);
		offerQuantity -= body.quantity_milli;
		const created = shipment('00000000-0000-0000-0000-000000000402', body.quantity_milli);
		shipments.push(created);
		chronicleEntries.unshift({
			id: '00000000-0000-0000-0000-000000000503',
			occurred_tick: 0,
			entry_type: 'market_purchase',
			related_household_id: sellerId,
			related_household_name: 'Hrafnstead',
			related_shipment_id: created.id,
			data: {
				resource_type: 'provisions',
				quantity_milli: body.quantity_milli,
				cost_milli: (body.quantity_milli * 1500) / 1000
			}
		});
		return send(response, 201, {
			cost_milli: (body.quantity_milli * 1500) / 1000,
			offer: {
				id: offerId,
				world_id: worldId,
				seller_household_id: sellerId,
				origin_location_id: created.origin_location_id,
				resource_type: 'provisions',
				quantity_remaining_milli: offerQuantity,
				price_per_unit_milli: 1500,
				created_tick: 0,
				expires_tick: 12,
				status: offerQuantity === 0 ? 'filled' : 'active'
			},
			shipment: created
		});
	}
	if (request.method === 'POST' && url.pathname === `/api/contracts/${contractId}/respond`) {
		const body = await readBody(request);
		contracts[0].status = body.decision === 'accept' ? 'active' : 'rejected';
		if (body.decision === 'accept') {
			contracts[0].obligations = [
				{
					id: obligationId,
					contract_id: contractId,
					debtor_household_id: householdId,
					creditor_household_id: sellerId,
					resource_type: 'wood',
					quantity_milli: 5_000,
					due_arrival_tick: 2,
					status: 'pending'
				}
			];
		}
		return send(response, 200, contracts[0]);
	}
	if (
		request.method === 'POST' &&
		url.pathname === `/api/contract-obligations/${obligationId}/dispatch`
	) {
		const created = {
			...shipment('00000000-0000-0000-0000-000000000603', 5_000),
			sender_household_id: householdId,
			receiver_household_id: sellerId,
			origin_location_id: '00000000-0000-0000-0000-000000000010',
			destination_location_id: '00000000-0000-0000-0000-000000000011',
			resource_type: 'wood',
			transport_cost_milli: 1_000
		};
		shipments.push(created);
		contracts[0].obligations[0] = {
			...contracts[0].obligations[0],
			shipment_id: created.id,
			status: 'dispatched'
		};
		return send(response, 201, { obligation: contracts[0].obligations[0], shipment: created });
	}
	return send(response, 404, { error: 'not_found' });
}).listen(9080, '127.0.0.1');
