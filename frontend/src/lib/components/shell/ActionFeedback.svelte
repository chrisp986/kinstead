<script lang="ts">
	let { feedback }: { feedback?: { success?: boolean; message?: string } | null } = $props();
</script>

{#if feedback?.message}
	<p class:success={feedback.success} class="action-feedback" role="status" aria-live="polite">
		{feedback.message}
	</p>
{/if}

<style>
	.action-feedback {
		margin: 0.75rem 0;
		padding: 0.75rem 0.9rem;
		border-left: 3px solid var(--critical);
		background: #f5ded6;
		color: var(--critical);
		font-size: 0.85rem;
	}
	.action-feedback.success {
		border-left-color: var(--positive);
		background: #e8efe2;
		color: var(--positive);
	}
</style>
