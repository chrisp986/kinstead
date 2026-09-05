<script lang="ts">
	import type { Character, Assignment } from '$lib/api/generated';
	import { enhance } from '$app/forms';
	import { labelActivity, sentenceCase } from '$lib/domain/format';
	import ActionFeedback from '$lib/components/shell/ActionFeedback.svelte';
	import WorkerCard from './WorkerCard.svelte';
	import WorkAssignmentList from './WorkAssignmentList.svelte';

	let {
		characters,
		assignments,
		currentGameDay,
		feedback
	}: {
		characters: Character[];
		assignments: Assignment[];
		currentGameDay: number;
		feedback?: { success?: boolean; action?: string; message?: string } | null;
	} = $props();
	let working = $state(false);
	const workers = $derived(characters.filter((character) => character.labor_permille > 0));
	let selectedCharacterId = $state(
		characters.find((character) => character.labor_permille > 0)?.id ?? ''
	);
	let activity = $state('agriculture');
	let intensity = $state('normal');
	let duration = $state(3);
	let selectedWorker = $derived(workers.find((character) => character.id === selectedCharacterId));
	let intensityEffect = $derived(
		intensity === 'light'
			? '80% output · +2 fatigue'
			: intensity === 'high'
				? '120% output · +7 fatigue'
				: '100% output · +4 fatigue'
	);

	function assignmentFor(id: string) {
		return assignments.find((assignment) => assignment.character_id === id);
	}
</script>

<section class="panel" aria-labelledby="work-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">Labor decisions</p>
			<h2 id="work-heading">Choose who should do the next useful work</h2>
			<p class="section-copy">
				Start with the person: check fatigue and current work before changing the plan.
			</p>
		</div>
	</div>

	<div class="workers">
		{#each workers as character (character.id)}
			<WorkerCard
				{character}
				assignment={assignmentFor(character.id)}
				{currentGameDay}
				selected={selectedCharacterId === character.id}
				onselect={() => (selectedCharacterId = character.id)}
			/>
		{/each}
	</div>

	<WorkAssignmentList {assignments} {currentGameDay} />

	<form
		method="POST"
		action="?/assign"
		use:enhance={() => {
			working = true;
			return async ({ update }) => {
				await update();
				working = false;
			};
		}}
	>
		<div class="plan-heading">
			<div>
				<p class="eyebrow">Next assignment</p>
				<h3>Plan {selectedWorker?.name ?? 'household'} work</h3>
			</div>
			{#if selectedWorker}
				<div class:warning={selectedWorker.fatigue >= 50} class="worker-state">
					<strong>Fatigue {selectedWorker.fatigue}</strong>
					<span
						>{selectedWorker.fatigue >= 70
							? 'Reduced output'
							: selectedWorker.fatigue >= 50
								? 'Watch recovery'
								: 'Ready to work'}</span
					>
				</div>
			{/if}
		</div>

		<div class="form-grid">
			<label>
				<span>Who?</span>
				<select name="character_id" required bind:value={selectedCharacterId}>
					{#each workers as character (character.id)}
						<option value={character.id}>{character.name}</option>
					{/each}
				</select>
			</label>
			<label>
				<span>Activity</span>
				<select name="activity" required bind:value={activity}>
					{#each ['agriculture', 'fishing', 'woodcutting', 'rest'] as activityOption (activityOption)}
						<option value={activityOption}>{labelActivity(activityOption)}</option>
					{/each}
				</select>
			</label>
			<label>
				<span>Intensity</span>
				<select name="intensity" required bind:value={intensity}>
					{#each ['light', 'normal', 'high'] as intensityOption (intensityOption)}
						<option value={intensityOption}>{sentenceCase(intensityOption)}</option>
					{/each}
				</select>
			</label>
			<label>
				<span>Duration</span>
				<select name="duration_ticks" required bind:value={duration}>
					{#each [1, 3, 6, 12] as durationOption (durationOption)}
						<option value={durationOption}
							>{durationOption === 1 ? 'one period' : `${durationOption} periods`}</option
						>
					{/each}
				</select>
			</label>
		</div>

		<div class="plan-preview" aria-live="polite">
			<div>
				<span>Planned work</span>
				<strong>{selectedWorker?.name ?? 'Worker'} · {labelActivity(activity)}</strong>
			</div>
			<div>
				<span>Work pressure</span>
				<strong
					>{activity === 'rest'
						? 'Recovery · no production'
						: `${sentenceCase(intensity)} · ${intensityEffect}`}</strong
				>
			</div>
			<div>
				<span>Duration</span>
				<strong>{duration === 1 ? 'One period' : `${duration} periods`}</strong>
			</div>
		</div>

		<p class="decision-note">
			{activity === 'rest'
				? 'Rest trades production for recovery. Use it before fatigue begins to reduce output.'
				: intensity === 'high'
					? 'High intensity produces more now but pushes fatigue up fastest.'
					: intensity === 'light'
						? 'Light work preserves more stamina at the cost of immediate output.'
						: 'Normal intensity is the balanced default for output and fatigue.'}
		</p>

		<ActionFeedback feedback={feedback?.action === 'assign' ? feedback : null} />
		<button class="primary-action" type="submit" disabled={working || workers.length === 0}>
			{working ? 'Scheduling…' : `Assign ${selectedWorker?.name ?? 'work'}`}
		</button>
	</form>
</section>

<style>
	.section-copy {
		max-width: 42rem;
		margin: 0.35rem 0 0;
		color: var(--ink-soft);
		font-size: 0.84rem;
	}
	.workers {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.75rem;
		margin-top: 1rem;
	}
	form {
		margin-top: 1.25rem;
		padding-top: 1.15rem;
		border-top: 3px solid var(--green);
	}
	.plan-heading {
		display: flex;
		align-items: end;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 0.9rem;
	}
	.plan-heading h3 {
		margin: 0.1rem 0 0;
		font-family: var(--font-display);
		font-size: 1.25rem;
		font-weight: 500;
	}
	.worker-state {
		display: grid;
		justify-items: end;
		font-size: 0.72rem;
	}
	.worker-state strong {
		color: var(--positive);
	}
	.worker-state.warning strong {
		color: var(--warning);
	}
	.worker-state span {
		color: var(--ink-soft);
	}
	.form-grid {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 0.75rem;
	}
	.plan-preview {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 1px;
		margin-top: 0.9rem;
		border: 1px solid var(--line-light);
		background: var(--line-light);
	}
	.plan-preview > div {
		display: grid;
		gap: 0.2rem;
		padding: 0.8rem;
		background: var(--surface-muted);
	}
	.plan-preview span {
		color: var(--ink-soft);
		font-size: 0.67rem;
		font-weight: 800;
		letter-spacing: 0.07em;
		text-transform: uppercase;
	}
	.plan-preview strong {
		font-size: 0.8rem;
		line-height: 1.35;
	}
	.decision-note {
		margin: 0.75rem 0 0;
		padding-left: 0.7rem;
		border-left: 2px solid var(--ochre);
		color: var(--ink-soft);
		font-size: 0.78rem;
		line-height: 1.4;
	}
	.primary-action {
		width: 100%;
		margin-top: 0.9rem;
		min-height: 3rem;
	}
	@media (max-width: 760px) {
		.workers,
		.form-grid,
		.plan-preview {
			grid-template-columns: 1fr;
		}
	}
	@media (max-width: 480px) {
		.plan-heading {
			align-items: start;
		}
		.worker-state {
			justify-items: start;
		}
	}
</style>
