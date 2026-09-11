<script lang="ts">
	import type { Character, TemporaryDuty, Workday } from '$lib/api/generated';
	import { labelActivity } from '$lib/domain/format';
	import { formatHour, formatMoment } from '$lib/domain/time';
	let {
		character,
		duty,
		workday,
		currentGameDay,
		selected = false,
		onselect
	}: {
		character: Character;
		duty?: TemporaryDuty;
		workday: Workday;
		currentGameDay: number;
		selected?: boolean;
		onselect?: () => void;
	} = $props();
	let occupation = $derived(character.occupation);
	let status = $derived(
		character.status === 'ill'
			? 'Ill · recovering'
			: character.status === 'absent'
				? 'Absent · recovering'
				: character.fatigue >= 85
					? 'Exhausted'
					: character.fatigue >= 70
						? 'Tired'
						: character.fatigue >= 50
							? 'Fatigue building'
							: 'Available'
	);
	let tone = $derived(
		character.fatigue >= 85 ? 'critical' : character.fatigue >= 50 ? 'warning' : 'safe'
	);
</script>

<article class:selected class={`worker ${tone}`}>
	<div class="worker-heading">
		<div>
			<h3>{character.name}</h3>
			<span
				>{character.labor_permille === 1000
					? 'Full capacity'
					: character.labor_permille > 0
						? 'Partial capacity'
						: 'Not a home worker'}</span
			>
		</div>
		<span class="condition">{status}</span>
	</div>
	<div class="fatigue">
		<div class="fatigue-copy">
			<span>Fatigue {character.fatigue}</span><small
				>{character.fatigue >= 70 ? 'Output reduced' : 'Stable recovery cycle'}</small
			>
		</div>
		<progress max="100" value={character.fatigue} aria-label={`${character.name} fatigue`}
		></progress>
	</div>
	<p class="current-work">
		<strong>Regular occupation</strong><span
			>{occupation ? labelActivity(occupation.activity) : 'Not configured'}</span
		>{#if occupation?.pending_activity}<small
				>Pending: {labelActivity(occupation.pending_activity)}</small
			>{/if}
	</p>
	<p class="current-work">
		<strong>Current activity</strong><span
			>{labelActivity(character.current_activity ?? 'rest')} · {character.current_activity_reason ??
				'outside the working period'}</span
		>
	</p>
	<p class="current-work">
		<strong>Today’s working period</strong><span
			>{formatHour(workday.start_hour)}–{formatHour(workday.end_hour)}</span
		>
	</p>
	{#if duty}<p class="current-work">
			<strong>Temporary commitment</strong><span
				>Jarl service · returns {formatMoment(duty.ends, currentGameDay)}</span
			>
		</p>{/if}{#if onselect}<button
			class="plan-button"
			type="button"
			aria-pressed={selected}
			onclick={onselect}
			>{selected ? 'Planning for this person' : `Plan ${character.name}'s occupation`}</button
		>{/if}
</article>

<style>
	.worker {
		padding: 0.9rem;
		border: 1px solid var(--line-light);
		border-left: 3px solid var(--positive);
		background: var(--surface);
	}
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
	.fatigue {
		display: grid;
		gap: 0.4rem;
		margin-top: 0.8rem;
	}
	.fatigue-copy {
		display: flex;
		justify-content: space-between;
		gap: 0.75rem;
		color: var(--ink-soft);
		font-size: 0.72rem;
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
	.current-work span,
	.current-work small {
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
	}
</style>
