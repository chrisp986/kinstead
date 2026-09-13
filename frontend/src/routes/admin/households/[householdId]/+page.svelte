<script lang="ts">
	import AdminPanel from '$lib/components/admin/AdminPanel.svelte';
	import { resolve } from '$app/paths';
	let { data } = $props();
	const json = (value: unknown) => JSON.stringify(value);
	const date = (value: string) => new Date(value).toLocaleString();
</script>

<div class="page">
	<a href={resolve('/admin')}>← Console overview</a>
	<header>
		<p class="eyebrow">Consistent database snapshot</p>
		<h1>{data.household.name}</h1>
		<p>{data.household.world_name} · owner {data.household.owner_player_id ?? 'unassigned'}</p>
		<p class="note">
			Captured {date(data.household.snapshot_captured_at)} at committed tick {data.household
				.current_committed_tick}. Current activity explanations are simulation-boundary state, not
			real-time observations.
		</p>
		<a href={resolve('/admin/households/[householdId]', { householdId: data.household.id })}
			>Refresh snapshot</a
		>
	</header>
	<AdminPanel title="Clock and resources"
		><dl>
			<div>
				<dt>Model</dt>
				<dd>{data.household.simulation_model}</dd>
			</div>
			<div>
				<dt>Season</dt>
				<dd>{data.household.season}</dd>
			</div>
			<div>
				<dt>Game moment</dt>
				<dd>
					{data.household.game_moment
						? `day ${data.household.game_moment.day}, hour ${data.household.game_moment.hour}`
						: 'unavailable for legacy model'}
				</dd>
			</div>
		</dl>
		<div class="resource-grid">
			{#each Object.entries(data.household.resources_milli) as [resource, amount] (resource)}<div>
					<strong>{resource}</strong><span>{amount} milli-units</span>
				</div>{/each}
		</div>
		<p class="note">Pending output: {json(data.household.pending_output_milli)}</p></AdminPanel
	>
	<AdminPanel title="Characters and activity"
		><div class="table-wrap">
			<table>
				<thead><tr><th>Character</th><th>Activity</th><th>Reason</th><th>Fatigue</th></tr></thead
				><tbody
					>{#each data.household.characters as character (character.id)}<tr
							><td>{character.name}<small>{character.status}, age {character.age}</small></td><td
								>{character.current_activity}</td
							><td
								><strong>{character.activity_reason_code}</strong><small
									>{character.activity_reason}</small
								></td
							><td>{character.fatigue}</td></tr
						>{/each}</tbody
				>
			</table>
		</div>
		{#if data.household.characters.length === 0}<p class="empty">
				No characters in this snapshot.
			</p>{/if}</AdminPanel
	>
	<AdminPanel title="Commitments and shipments"
		><p>Temporary duties: {data.household.temporary_duties.length}</p>
		<div class="shipments">
			<div>
				<h3>Incoming</h3>
				{#if data.household.incoming_shipments.length === 0}<p class="empty">
						None recorded.
					</p>{:else}<ul>
						{#each data.household.incoming_shipments as shipment (shipment.id)}<li>
								{shipment.resource_type} · {shipment.quantity_milli} · {shipment.status}
							</li>{/each}
					</ul>{/if}
			</div>
			<div>
				<h3>Outgoing</h3>
				{#if data.household.outgoing_shipments.length === 0}<p class="empty">
						None recorded.
					</p>{:else}<ul>
						{#each data.household.outgoing_shipments as shipment (shipment.id)}<li>
								{shipment.resource_type} · {shipment.quantity_milli} · {shipment.status}
							</li>{/each}
					</ul>{/if}
			</div>
		</div></AdminPanel
	>
	<AdminPanel title="Recorded tick accounting"
		><p class="note">
			{data.household.history_coverage.statement} Only recorded diagnostics are shown; missing rows are
			not reconstructed.
		</p>
		{#if data.diagnostics.length === 0}<p class="empty">No tick diagnostics recorded.</p>{:else}<div
				class="table-wrap"
			>
				<table>
					<thead><tr><th>Tick</th><th>Interval</th><th>Recorded</th><th>Details</th></tr></thead
					><tbody
						>{#each data.diagnostics as diagnostic (diagnostic.tick)}<tr
								><td>{diagnostic.tick}</td><td
									>day {diagnostic.interval_start_day} → day {diagnostic.interval_end_day}</td
								><td>{date(diagnostic.recorded_at)}</td><td
									><details>
										<summary>View accounting</summary><code>{json(diagnostic.details)}</code>
									</details></td
								></tr
							>{/each}</tbody
					>
				</table>
			</div>{/if}</AdminPanel
	>
</div>

<style>
	.page {
		display: grid;
		gap: 1rem;
	}
	a {
		color: var(--green);
		font-weight: 700;
	}
	header {
		padding: 0.5rem 0;
	}
	h1 {
		margin: 0.25rem 0;
		font: 2.2rem var(--font-display);
		font-weight: 500;
	}
	header p {
		color: var(--ink-soft);
	}
	.note {
		color: var(--ink-soft);
		font-size: 0.85rem;
	}
	dl {
		display: flex;
		gap: 2rem;
		flex-wrap: wrap;
		margin: 1rem 0;
	}
	dt {
		color: var(--ink-soft);
		font-size: 0.75rem;
		font-weight: 800;
		text-transform: uppercase;
	}
	dd {
		margin: 0.3rem 0 0;
	}
	.resource-grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
		gap: 0.5rem;
	}
	.resource-grid div {
		display: grid;
		gap: 0.3rem;
		padding: 0.8rem;
		background: var(--surface-muted);
	}
	.resource-grid span,
	small {
		display: block;
		color: var(--ink-soft);
		font-size: 0.8rem;
	}
	.table-wrap {
		overflow-x: auto;
	}
	table {
		width: 100%;
		min-width: 620px;
		border-collapse: collapse;
	}
	th,
	td {
		padding: 0.65rem;
		border-bottom: 1px solid var(--line-light);
		text-align: left;
		vertical-align: top;
	}
	th {
		color: var(--ink-soft);
		font-size: 0.72rem;
		text-transform: uppercase;
	}
	code {
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
	.shipments {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 2rem;
	}
	h3 {
		font: 1.25rem var(--font-display);
	}
	@media (max-width: 600px) {
		.shipments {
			grid-template-columns: 1fr;
		}
	}
</style>
