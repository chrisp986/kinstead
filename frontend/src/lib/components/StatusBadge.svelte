<script lang="ts">
	import { sentenceCase } from '$lib/domain/format';

	let { status }: { status: string } = $props();
	let tone = $derived(
		status === 'arrived' || status === 'completed' || status === 'active'
			? 'positive'
			: status === 'broken' || status === 'late'
				? 'critical'
				: status === 'cancelled' || status === 'rejected'
					? 'muted'
					: 'pending'
	);
</script>

<span class="badge {tone}">{sentenceCase(status)}</span>

<style>
	.badge {
		display: inline-flex;
		align-items: center;
		border: 1px solid var(--line);
		border-radius: 999px;
		padding: 0.18rem 0.55rem;
		font-size: 0.72rem;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
	}
	.positive {
		border-color: #6d8b69;
		background: #e8efe2;
		color: #365139;
	}
	.pending {
		border-color: #b99454;
		background: #f5ead4;
		color: #664c22;
	}
	.muted {
		background: var(--surface-muted);
		color: var(--ink-soft);
	}
	.critical {
		border-color: #b66d61;
		background: #f5ded6;
		color: var(--critical);
	}
</style>
