<script lang="ts">
	import type { Assignment, Character } from '$lib/api/generated';
	import { labelActivity } from '$lib/domain/format';

	let { character, assignment }: { character: Character; assignment?: Assignment } = $props();
	const current = $derived(
		assignment && assignment.starts_tick <= assignment.ends_tick ? assignment : undefined
	);
</script>

<article class="worker">
	<div class="worker-heading">
		<h3>{character.name}</h3>
		<span>{character.labor_permille === 1000 ? 'Full capacity' : 'Partial capacity'}</span>
	</div>
	<div class="fatigue">
		<span>Fatigue {character.fatigue}</span><progress
			max="100"
			value={character.fatigue}
			aria-label={`${character.name} fatigue`}
		></progress>
	</div>
	<p>
		<strong>Current</strong>
		{current
			? `${labelActivity(current.activity)} · ticks ${current.starts_tick}–${current.ends_tick}`
			: 'Available'}
	</p>
</article>

<style>
	.worker {
		padding: 0.85rem;
		border: 1px solid var(--line-light);
		background: var(--surface);
	}
	.worker-heading {
		display: flex;
		justify-content: space-between;
		gap: 0.5rem;
		align-items: baseline;
	}
	h3 {
		margin: 0;
		font-family: var(--font-display);
		font-size: 1.1rem;
	}
	.worker-heading span {
		color: var(--ink-soft);
		font-size: 0.7rem;
	}
	.fatigue {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 0.55rem;
		align-items: center;
		margin-top: 0.7rem;
		color: var(--ink-soft);
		font-size: 0.72rem;
	}
	progress {
		width: 100%;
		height: 0.4rem;
		accent-color: var(--ochre);
	}
	p {
		margin: 0.7rem 0 0;
		color: var(--ink-soft);
		font-size: 0.78rem;
	}
	p strong {
		color: var(--ink);
	}
</style>
