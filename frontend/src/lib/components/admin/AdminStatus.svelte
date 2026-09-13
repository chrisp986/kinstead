<script lang="ts">
	let { value }: { value: string } = $props();
</script>

<span class={`status status-${value}`}>{value.replaceAll('_', ' ')}</span>

<style>
	.status {
		display: inline-block;
		padding: 0.25rem 0.45rem;
		border: 1px solid var(--line);
		font-size: 0.72rem;
		font-weight: 800;
		text-transform: capitalize;
	}
	.status-waiting,
	.status-catching_up,
	.status-valid {
		color: var(--positive);
	}
	.status-delayed,
	.status-expired {
		color: var(--warning);
	}
	.status-worker_unavailable,
	.status-revoked {
		color: var(--critical);
	}
</style>
