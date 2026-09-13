<script lang="ts">
	import AdminPanel from '$lib/components/admin/AdminPanel.svelte';
	import AdminStatus from '$lib/components/admin/AdminStatus.svelte';
	import { resolve } from '$app/paths';
	let { data } = $props();
	const date = (value: string) => new Date(value).toLocaleString();
</script>

<div class="page">
	<a href={resolve('/admin')}>← Console overview</a>
	<header>
		<p class="eyebrow">World inspection</p>
		<h1>{data.world.name}</h1>
		<p>{data.world.simulation_model} · current tick {data.world.current_tick}</p>
		<a href={resolve('/admin/worlds/[worldId]', { worldId: data.world.id })}>Refresh observations</a
		>
	</header>
	<AdminPanel title="Progress"
		><dl>
			<div>
				<dt>Status</dt>
				<dd><AdminStatus value={data.world.status} /></dd>
			</div>
			<div>
				<dt>Game day</dt>
				<dd>{data.world.current_game_day}</dd>
			</div>
			<div>
				<dt>Due ticks</dt>
				<dd>{data.world.due_tick_count}</dd>
			</div>
			<div>
				<dt>Next scheduled</dt>
				<dd>{date(data.world.next_tick_at)}</dd>
			</div>
			<div>
				<dt>Last committed</dt>
				<dd>
					{data.world.last_committed_at ? date(data.world.last_committed_at) : 'not recorded'}
				</dd>
			</div>
		</dl>
		<ul>
			{#each data.world.status_facts as fact (fact)}<li>{fact}</li>{/each}
		</ul></AdminPanel
	>
	<AdminPanel title="Worker heartbeat observations"
		>{#if data.world.heartbeats.length === 0}<p class="empty">
				No heartbeat has been recorded.
			</p>{:else}<div class="table-wrap">
				<table>
					<thead><tr><th>Instance</th><th>Started</th><th>Last seen</th></tr></thead><tbody
						>{#each data.world.heartbeats as heartbeat (heartbeat.instance_id)}<tr
								><td>{heartbeat.instance_id}</td><td>{date(heartbeat.started_at)}</td><td
									>{date(heartbeat.last_seen_at)}</td
								></tr
							>{/each}</tbody
					>
				</table>
			</div>{/if}</AdminPanel
	>
	<AdminPanel title="Recent failures"
		>{#if data.world.recent_failures.length === 0}<p class="empty">
				No recorded failures for this world.
			</p>{:else}<ul>
				{#each data.world.recent_failures as failure (failure.id)}<li>
						<strong>{failure.error_code}</strong> · {failure.stage ?? 'stage unavailable'} · {failure.message}
					</li>{/each}
			</ul>{/if}</AdminPanel
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
	header p:last-child {
		color: var(--ink-soft);
	}
	dl {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
		gap: 1rem;
		margin: 1rem 0 0;
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
	ul {
		padding-left: 1.2rem;
		color: var(--ink-soft);
	}
	.table-wrap {
		overflow-x: auto;
	}
	table {
		width: 100%;
		min-width: 600px;
		border-collapse: collapse;
	}
	th,
	td {
		padding: 0.65rem;
		border-bottom: 1px solid var(--line-light);
		text-align: left;
	}
</style>
