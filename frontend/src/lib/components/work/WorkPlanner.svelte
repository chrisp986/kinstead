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
		feedback
	}: {
		characters: Character[];
		assignments: Assignment[];
		feedback?: { success?: boolean; action?: string; message?: string } | null;
	} = $props();
	let working = $state(false);
	const workers = $derived(characters.filter((character) => character.labor_permille > 0));
	function assignmentFor(id: string) {
		return assignments.find((assignment) => assignment.character_id === id);
	}
</script>

<section class="panel" aria-labelledby="work-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">Labor decisions</p>
			<h2 id="work-heading">Who is working?</h2>
		</div>
	</div>
	<div class="workers">
		{#each workers as character (character.id)}<WorkerCard
				{character}
				assignment={assignmentFor(character.id)}
			/>{/each}
	</div>
	<WorkAssignmentList {assignments} />
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
		<h3>Plan next work</h3>
		<div class="form-grid">
			<label
				><span>Who?</span><select name="character_id" required
					>{#each workers as character (character.id)}<option value={character.id}
							>{character.name}</option
						>{/each}</select
				></label
			>
			<label
				><span>Activity</span><select name="activity" required
					>{#each ['agriculture', 'fishing', 'woodcutting', 'rest'] as activity (activity)}<option
							value={activity}>{labelActivity(activity)}</option
						>{/each}</select
				></label
			>
			<label
				><span>Intensity</span><select name="intensity" required
					>{#each ['light', 'normal', 'high'] as intensity (intensity)}<option
							value={intensity}
							selected={intensity === 'normal'}>{sentenceCase(intensity)}</option
						>{/each}</select
				></label
			>
			<label
				><span>Duration</span><select name="duration_ticks" required
					>{#each [1, 3, 6, 12] as duration (duration)}<option
							value={duration}
							selected={duration === 3}>{duration} {duration === 1 ? 'tick' : 'ticks'}</option
						>{/each}</select
				></label
			>
		</div>
		<div class="hints">
			<span>Light · 80% output · +2 fatigue</span><span>Normal · 100% output · +4 fatigue</span
			><span>High · 120% output · +7 fatigue</span><span>Rest · no production · recovery</span>
		</div>
		<ActionFeedback feedback={feedback?.action === 'assign' ? feedback : null} />
		<button type="submit" disabled={working || workers.length === 0}
			>{working ? 'Scheduling…' : 'Schedule work'}</button
		>
	</form>
</section>

<style>
	.workers {
		display: grid;
		gap: 0.75rem;
		margin-top: 1rem;
	}
	form {
		margin-top: 1.2rem;
		padding-top: 1.1rem;
		border-top: 3px solid var(--green);
	}
	form h3 {
		margin: 0 0 0.9rem;
		font-family: var(--font-display);
		font-size: 1.15rem;
	}
	.form-grid {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 0.75rem;
	}
	.hints {
		display: grid;
		gap: 0.25rem;
		margin-top: 0.75rem;
		color: var(--ink-soft);
		font-size: 0.72rem;
	}
	button {
		width: 100%;
		margin-top: 0.9rem;
		min-height: 3rem;
	}
	@media (max-width: 760px) {
		.form-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
