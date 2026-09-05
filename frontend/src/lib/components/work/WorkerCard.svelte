<script lang="ts">
	import type { Assignment, Character } from '$lib/api/generated';
	import { labelActivity } from '$lib/domain/format';
	import { formatRelativeGameDay } from '$lib/domain/time';

	let {
		character,
		assignment,
		currentGameDay,
		selected = false,
		onselect
	}: {
		character: Character;
		assignment?: Assignment;
		currentGameDay: number;
		selected?: boolean;
		onselect?: () => void;
	} = $props();
	const current = $derived(assignment);
	let fatigueTone = $derived(
		character.fatigue >= 85
			? 'critical'
			: character.fatigue >= 70
				? 'warning'
				: character.fatigue >= 50
					? 'strained'
					: 'safe'
	);
	let fatigueLabel = $derived(
		character.fatigue >= 85
			? 'Exhausted'
			: character.fatigue >= 70
				? 'Tired'
				: character.fatigue >= 50
					? 'Fatigue building'
					: 'Ready'
	);
	let fatigueEffect = $derived(
		character.fatigue >= 85
			? '-25% work output · higher event risk'
			: character.fatigue >= 70
				? '-10% work output'
				: character.fatigue >= 50
					? 'Output is still normal; watch recovery'
					: 'Normal work output'
	);
</script>

<article class:selected class={`worker ${fatigueTone}`}>
	<div class="worker-heading">
		<div>
			<h3>{character.name}</h3>
			<span>{character.labor_permille === 1000 ? 'Full capacity' : 'Partial capacity'}</span>
		</div>
		<span class="condition">{fatigueLabel}</span>
	</div>
	<div class="fatigue">
		<div class="fatigue-copy">
			<span>Fatigue {character.fatigue}</span>
			<small>{fatigueEffect}</small>
		</div>
		<progress max="100" value={character.fatigue} aria-label={`${character.name} fatigue`}
		></progress>
	</div>
	<p class="current-work">
		<strong>Current plan</strong>
		<span>
			{current
				? `${labelActivity(current.activity)} · ${formatRelativeGameDay(currentGameDay, current.starts_game_day ?? currentGameDay)}`
				: 'Available for new work'}
		</span>
	</p>
	{#if onselect}
		<button class="plan-button" type="button" aria-pressed={selected} onclick={onselect}>
			{selected ? 'Planning for this person' : `Plan ${character.name}'s work`}
		</button>
	{/if}
</article>

<style>
	.worker {
		padding: 0.9rem;
		border: 1px solid var(--line-light);
		border-left: 3px solid var(--positive);
		background: var(--surface);
	}
	.worker.strained,
	.worker.warning {
		border-left-color: var(--warning);
	}
	.worker.critical {
		border-left-color: var(--critical);
	}
	.worker.selected {
		outline: 2px solid var(--green);
		outline-offset: 2px;
	}
	.worker-heading {
		display: flex;
		justify-content: space-between;
		gap: 0.75rem;
		align-items: start;
	}
	.worker-heading > div {
		display: grid;
		gap: 0.1rem;
	}
	h3 {
		margin: 0;
		font-family: var(--font-display);
		font-size: 1.15rem;
	}
	.worker-heading span {
		color: var(--ink-soft);
		font-size: 0.7rem;
	}
	.condition {
		padding: 0.2rem 0.4rem;
		border: 1px solid var(--line-light);
		background: var(--surface-muted);
		font-weight: 800;
		white-space: nowrap;
	}
	.warning .condition,
	.strained .condition {
		color: var(--warning);
	}
	.critical .condition {
		color: var(--critical);
	}
	.fatigue {
		display: grid;
		gap: 0.4rem;
		margin-top: 0.8rem;
	}
	.fatigue-copy {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 0.75rem;
		color: var(--ink-soft);
		font-size: 0.72rem;
	}
	.fatigue-copy small {
		text-align: right;
	}
	progress {
		width: 100%;
		height: 0.45rem;
		accent-color: var(--ochre);
	}
	.current-work {
		display: grid;
		gap: 0.15rem;
		margin: 0.75rem 0 0;
		font-size: 0.78rem;
	}
	.current-work strong {
		color: var(--ink);
	}
	.current-work span {
		color: var(--ink-soft);
	}
	.plan-button {
		width: 100%;
		min-height: 2.45rem;
		margin-top: 0.8rem;
		padding: 0.45rem 0.65rem;
		border: 1px solid var(--line);
		background: transparent;
		color: var(--green);
		font-size: 0.76rem;
		font-weight: 800;
	}
	.plan-button:hover,
	.plan-button:focus-visible,
	.plan-button[aria-pressed='true'] {
		background: var(--surface-muted);
	}
	@media (max-width: 480px) {
		.fatigue-copy {
			display: grid;
			gap: 0.1rem;
		}
		.fatigue-copy small {
			text-align: left;
		}
	}
</style>
