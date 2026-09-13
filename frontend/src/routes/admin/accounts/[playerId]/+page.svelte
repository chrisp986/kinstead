<script lang="ts">
	import AdminPanel from '$lib/components/admin/AdminPanel.svelte';
	import AdminStatus from '$lib/components/admin/AdminStatus.svelte';
	import { resolve } from '$app/paths';
	let { data } = $props();
	const date = (value: string) => new Date(value).toLocaleString();
</script>

<div class="page">
	<a href={resolve('/admin/accounts')}>← Accounts</a>
	<header>
		<p class="eyebrow">Safe account inspection</p>
		<h1>{data.account.player_id}</h1>
		<p>{data.account.external_auth_subject}</p>
		<a href={resolve('/admin/accounts/[playerId]', { playerId: data.account.player_id })}
			>Refresh account</a
		>
	</header>
	<AdminPanel title="Owned households"
		>{#if data.account.households.length === 0}<p class="empty">
				No household ownership links.
			</p>{:else}<ul>
				{#each data.account.households as household (household.id)}<li>
						<a href={resolve('/admin/households/[householdId]', { householdId: household.id })}
							>{household.name}</a
						>
						<small>{household.id}</small>
					</li>{/each}
			</ul>{/if}</AdminPanel
	><AdminPanel title="Sessions"
		><p class="note">
			Session IDs are safe UUIDs. Token values and hashes are intentionally unavailable.
		</p>
		{#if data.sessions.length === 0}<p class="empty">No sessions recorded.</p>{:else}<div
				class="table-wrap"
			>
				<table>
					<thead
						><tr
							><th>Session ID</th><th>Created</th><th>Expires</th><th>Last seen</th><th>Status</th
							></tr
						></thead
					><tbody
						>{#each data.sessions as session (session.id)}<tr
								><td>{session.id}</td><td>{date(session.created_at)}</td><td
									>{date(session.expires_at)}</td
								><td>{session.last_seen_at ? date(session.last_seen_at) : 'not observed'}</td><td
									><AdminStatus value={session.status} /></td
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
		font: 1.7rem var(--font-display);
		overflow-wrap: anywhere;
	}
	header p:last-child,
	.note,
	small {
		color: var(--ink-soft);
	}
	small {
		display: block;
		font-size: 0.78rem;
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
	}
	th {
		color: var(--ink-soft);
		font-size: 0.72rem;
		text-transform: uppercase;
	}
</style>
