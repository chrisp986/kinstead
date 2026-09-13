<script lang="ts">
	import AdminPanel from '$lib/components/admin/AdminPanel.svelte';
	import { resolve } from '$app/paths';
	let { data } = $props();
	const date = (value: string) => new Date(value).toLocaleString();
</script>

<div class="page">
	<header>
		<p class="eyebrow">Identity and ownership</p>
		<h1>Accounts</h1>
		<p>Safe account fields and household ownership links. Session secrets are never displayed.</p>
	</header>
	<AdminPanel title="Account search"
		><form method="GET">
			<label
				><span>Player ID or external subject</span><input
					name="q"
					value={data.query}
					maxlength="120"
				/></label
			><button type="submit">Search</button>
		</form>
		{#if data.accounts.length === 0}<p class="empty">No accounts matched this search.</p>{:else}<div
				class="table-wrap"
			>
				<table>
					<thead><tr><th>Player</th><th>External subject</th><th>Created</th></tr></thead><tbody
						>{#each data.accounts as account (account.player_id)}<tr
								><td
									><a href={resolve('/admin/accounts/[playerId]', { playerId: account.player_id })}
										>{account.player_id}</a
									></td
								><td>{account.external_auth_subject}</td><td>{date(account.created_at)}</td></tr
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
		font-weight: 500;
	}
	header p:last-child {
		color: var(--ink-soft);
	}
	form {
		display: flex;
		align-items: end;
		gap: 0.75rem;
		margin-top: 1rem;
	}
	form label {
		flex: 1;
	}
	.table-wrap {
		overflow-x: auto;
		margin-top: 1rem;
	}
	table {
		width: 100%;
		min-width: 620px;
		border-collapse: collapse;
	}
	th,
	td {
		padding: 0.7rem;
		border-bottom: 1px solid var(--line-light);
		text-align: left;
	}
	th {
		color: var(--ink-soft);
		font-size: 0.72rem;
		text-transform: uppercase;
	}
	a {
		color: var(--green);
		font-weight: 700;
	}
	@media (max-width: 600px) {
		form {
			display: grid;
		}
	}
</style>
