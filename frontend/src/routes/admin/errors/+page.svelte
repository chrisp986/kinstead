<script lang="ts">
	import AdminPanel from '$lib/components/admin/AdminPanel.svelte';
	let { data } = $props();
	const date = (value: string) => new Date(value).toLocaleString();
</script>

<div class="page">
	<header>
		<p class="eyebrow">Operational evidence</p>
		<h1>Errors</h1>
		<p>
			Sanitized API and worker failures, bounded and rate-aggregated. Copy request IDs to correlate
			logs.
		</p>
	</header>
	<AdminPanel title="Filters"
		><form method="GET">
			<label><span>World ID</span><input name="world_id" value={data.filters.worldId} /></label
			><label
				><span>Household ID</span><input
					name="household_id"
					value={data.filters.householdId}
				/></label
			><label
				><span>Request ID</span><input name="request_id" value={data.filters.requestId} /></label
			><label
				><span>Source</span><select name="source"
					><option value="">Any</option><option value="api" selected={data.filters.source === 'api'}
						>API</option
					><option value="worker" selected={data.filters.source === 'worker'}>Worker</option
					></select
				></label
			><button type="submit">Apply</button>
		</form></AdminPanel
	><AdminPanel title="Recorded errors"
		>{#if data.errors.length === 0}<p class="empty">No errors matched these filters.</p>{:else}<div
				class="table-wrap"
			>
				<table>
					<thead
						><tr><th>Observed</th><th>Source / stage</th><th>Error</th><th>Identifiers</th></tr
						></thead
					><tbody
						>{#each data.errors as item (item.id)}<tr
								><td
									>{date(item.last_observed_at)}<small>{item.occurrence_count} occurrence(s)</small
									></td
								><td>{item.source}<br />{item.stage ?? 'stage unavailable'}</td><td
									><strong>{item.error_code}</strong><br />{item.message}</td
								><td
									>{#if item.request_id}<code>{item.request_id}</code>{/if}{#if item.world_id}<small
											>world {item.world_id}</small
										>{/if}{#if item.household_id}<small>household {item.household_id}</small
										>{/if}</td
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
	header {
		padding: 0.5rem 0;
	}
	h1 {
		margin: 0.25rem 0;
		font: 2.2rem var(--font-display);
	}
	header p:last-child {
		color: var(--ink-soft);
	}
	form {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 0.75rem;
		align-items: end;
		margin-top: 1rem;
	}
	.table-wrap {
		overflow-x: auto;
	}
	table {
		width: 100%;
		min-width: 760px;
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
	small {
		display: block;
		color: var(--ink-soft);
	}
	code {
		overflow-wrap: anywhere;
	}
	@media (max-width: 800px) {
		form {
			grid-template-columns: 1fr 1fr;
		}
	}
	@media (max-width: 500px) {
		form {
			grid-template-columns: 1fr;
		}
	}
</style>
