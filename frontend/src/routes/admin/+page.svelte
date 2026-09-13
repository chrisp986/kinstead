<script lang="ts">
	import AdminPanel from '$lib/components/admin/AdminPanel.svelte';
	import AdminStatus from '$lib/components/admin/AdminStatus.svelte';
	import { resolve } from '$app/paths';
	let { data } = $props();
	const formatDate = (value: string) => new Date(value).toLocaleString();
</script>

<div class="overview">
	<header>
		<p class="eyebrow">Read-only operations</p>
		<h1>Developer console</h1>
		<p>
			World progress, household state, and operational evidence. This surface has no gameplay
			controls.
		</p>
		<a href={resolve('/admin')}>Refresh observations</a>
	</header>
	<AdminPanel title="World status">
		{#if data.worlds.length === 0}<p class="empty">No worlds are available.</p>{:else}
			<div class="table-wrap">
				<table>
					<caption class="sr-only">World health</caption><thead
						><tr><th>World</th><th>Tick</th><th>Next scheduled</th><th>Status</th></tr></thead
					><tbody>
						{#each data.worlds as world (world.id)}<tr
								><td
									><a href={resolve('/admin/worlds/[worldId]', { worldId: world.id })}
										>{world.name}</a
									><small>{world.simulation_model}</small></td
								><td>{world.current_tick}</td><td>{formatDate(world.next_tick_at)}</td><td
									><AdminStatus value={world.status} /></td
								></tr
							>{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</AdminPanel>
	<AdminPanel title="Household search">
		<form method="GET">
			<label
				><span>Search by name, ID, or owner</span><input
					name="q"
					value={data.query}
					maxlength="120"
				/></label
			><button type="submit">Search</button>
		</form>
		{#if data.households.length === 0}<p class="empty">
				No households matched this search.
			</p>{:else}<div class="table-wrap">
				<table>
					<thead><tr><th>Household</th><th>World</th><th>Owner</th><th>Tick</th></tr></thead><tbody
						>{#each data.households as household (household.id)}<tr
								><td
									><a
										href={resolve('/admin/households/[householdId]', { householdId: household.id })}
										>{household.name}</a
									></td
								><td>{household.world_name}</td><td>{household.owner_player_id ?? 'unassigned'}</td
								><td>{household.current_committed_tick}</td></tr
							>{/each}</tbody
					>
				</table>
			</div>{/if}
	</AdminPanel>
</div>

<style>
	.overview {
		display: grid;
		gap: 1rem;
	}
	header {
		padding: 1rem 0;
	}
	h1 {
		margin: 0.25rem 0;
		font: 2.4rem var(--font-display);
		font-weight: 500;
	}
	header p:last-child {
		color: var(--ink-soft);
	}
	.table-wrap {
		overflow-x: auto;
		margin-top: 1rem;
	}
	table {
		width: 100%;
		border-collapse: collapse;
		min-width: 560px;
	}
	th,
	td {
		padding: 0.7rem;
		border-bottom: 1px solid var(--line-light);
		text-align: left;
		vertical-align: top;
	}
	th {
		color: var(--ink-soft);
		font-size: 0.72rem;
		text-transform: uppercase;
	}
	td a {
		color: var(--green);
		font-weight: 800;
	}
	small {
		display: block;
		color: var(--ink-soft);
		margin-top: 0.2rem;
	}
	form {
		display: flex;
		gap: 0.75rem;
		align-items: end;
		margin-top: 1rem;
	}
	form label {
		flex: 1;
	}
	form button {
		flex: 0 0 auto;
	}
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
	}
	@media (max-width: 600px) {
		form {
			display: grid;
		}
	}
</style>
