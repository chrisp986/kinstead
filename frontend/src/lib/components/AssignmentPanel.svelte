<script lang="ts">
	import type { Assignment, Character } from '$lib/api/generated';
	import { labelActivity, sentenceCase } from '$lib/domain/format';
	import { enhance } from '$app/forms';
	import StatusBadge from './StatusBadge.svelte';

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
	let activeAssignments = $derived(
		assignments.filter((assignment) => assignment.status !== 'completed')
	);
	function currentAssignment(characterId: string) {
		return activeAssignments.find((assignment) => assignment.character_id === characterId);
	}
</script>

<section class="panel" aria-labelledby="household-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">People and work</p>
			<h2 id="household-heading">Household</h2>
		</div>
	</div>
	<div class="people">
		{#each characters as character (character.id)}
			{@const assignment = currentAssignment(character.id)}
			<article class="person">
				<div class="avatar" aria-hidden="true">{character.name.slice(0, 1)}</div>
				<div class="person-copy">
					<div class="person-heading">
						<strong>{character.name}</strong>{#if assignment}<StatusBadge
								status={assignment.status}
							/>{/if}
					</div>
					<p>
						{assignment
							? `${labelActivity(assignment.activity)}, ticks ${assignment.starts_tick}–${assignment.ends_tick}`
							: character.labor_permille > 0
								? 'Available for work'
								: 'Not in the labor pool'}
					</p>
					<div class="fatigue">
						<span>Fatigue {character.fatigue}</span><progress
							max="100"
							value={character.fatigue}
							aria-label={`${character.name} fatigue`}
						></progress>
					</div>
				</div>
			</article>
		{/each}
	</div>
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
		<h3>Schedule work</h3>
		<div class="form-grid">
			<label
				><span>Household member</span><select name="character_id" required
					>{#each characters.filter((character) => character.labor_permille > 0) as character (character.id)}<option
							value={character.id}>{character.name}</option
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
		<div class="form-footer">
			{#if feedback?.action === 'assign'}<p
					class:success={feedback.success}
					class="feedback"
					role="status"
				>
					{feedback.message}
				</p>{:else}<p class="hint">Work begins next tick. Existing plans cannot overlap.</p>{/if}
			<button type="submit" disabled={working}>{working ? 'Scheduling…' : 'Schedule work'}</button>
		</div>
	</form>
</section>

<style>
	.people {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.75rem;
		margin-top: 1.25rem;
	}
	.person {
		display: flex;
		gap: 0.85rem;
		padding: 0.9rem;
		border: 1px solid var(--line-light);
	}
	.avatar {
		display: grid;
		place-items: center;
		flex: 0 0 2.5rem;
		height: 2.5rem;
		border-radius: 50%;
		background: var(--green);
		color: white;
		font-family: var(--font-display);
		font-size: 1.15rem;
	}
	.person-copy {
		min-width: 0;
		flex: 1;
	}
	.person-heading {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
	}
	.person p {
		margin: 0.2rem 0 0.7rem;
		color: var(--ink-soft);
		font-size: 0.82rem;
	}
	.fatigue {
		display: grid;
		grid-template-columns: auto 1fr;
		align-items: center;
		gap: 0.6rem;
		color: var(--ink-soft);
		font-size: 0.72rem;
	}
	progress {
		width: 100%;
		height: 0.35rem;
		accent-color: var(--ochre);
	}
	form {
		margin-top: 1.25rem;
		padding: 1.15rem;
		background: var(--surface-muted);
		border-top: 3px solid var(--green);
	}
	form h3 {
		margin: 0 0 1rem;
		font-family: var(--font-display);
	}
	.form-grid {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 0.8rem;
	}
	.form-footer {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		margin-top: 1rem;
	}
	.hint,
	.feedback {
		margin: 0;
		color: var(--ink-soft);
		font-size: 0.8rem;
	}
	.feedback:not(.success) {
		color: var(--critical);
	}
	.feedback.success {
		color: var(--positive);
	}
	@media (max-width: 760px) {
		.people,
		.form-grid {
			grid-template-columns: 1fr;
		}
		.form-footer {
			align-items: stretch;
			flex-direction: column;
		}
	}
</style>
