<script lang="ts">
	import type { Character, HouseholdForecast, WorkPlan } from '$lib/api/generated';
	import { enhance } from '$app/forms';
	import { labelActivity } from '$lib/domain/format';
	import ActionFeedback from '$lib/components/shell/ActionFeedback.svelte';
	import WorkerCard from './WorkerCard.svelte';
	import WorkAssignmentList from './WorkAssignmentList.svelte';
	import { formatHour, formatMoment, formatUTCOffset } from '$lib/domain/time';

	let {
		characters,
		currentGameDay,
		workPlan,
		feedback
	}: {
		characters: Character[];
		currentGameDay: number;
		workPlan: WorkPlan;
		feedback?: {
			success?: boolean;
			action?: string;
			message?: string;
			preview?: HouseholdForecast;
		} | null;
	} = $props();
	let working = $state(false);
	let selectedCharacterId = $state('');
	let activity = $state<'agriculture' | 'fishing' | 'woodcutting'>('agriculture');
	let previewForm: HTMLFormElement;
	let previewVersion = 0;
	let lastWorkerId = '';
	const workers = $derived(
		characters.filter((character) => character.labor_permille > 0 && character.status !== 'dead')
	);
	let selectedWorker = $derived(
		workers.find((character) => character.id === selectedCharacterId) ?? workers[0]
	);
	let selectedOccupation = $derived(selectedWorker?.occupation);
	let expectedRevision = $derived(selectedOccupation?.revision ?? 1);

	$effect(() => {
		if (workers.length === 0) {
			selectedCharacterId = '';
			return;
		}
		if (!workers.some((character) => character.id === selectedCharacterId))
			selectedCharacterId = workers[0].id;
	});
	$effect(() => {
		if (!selectedWorker || selectedWorker.id === lastWorkerId) return;
		lastWorkerId = selectedWorker.id;
		activity =
			selectedWorker.occupation?.pending_activity ??
			selectedWorker.occupation?.activity ??
			'agriculture';
	});
	$effect(() => {
		const intent = [selectedCharacterId, activity];
		if (!previewForm || !intent[0]) return;
		previewVersion += 1;
		const timer = setTimeout(() => previewForm.requestSubmit(), 250);
		return () => clearTimeout(timer);
	});
	function dutyFor(id: string) {
		return workPlan.temporary_duties.find((duty) => duty.character_id === id);
	}
	function stock(milli: number): string {
		return `${(milli / 1000).toFixed(1)} units`;
	}
</script>

<section class="panel" aria-labelledby="work-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">Household occupations</p>
			<h2 id="work-heading">Set the routine once</h2>
			<p class="section-copy">
				Today’s work: {formatHour(workPlan.workday.start_hour)}–{formatHour(
					workPlan.workday.end_hour
				)}. Production deposits automatically when the workday ends.
			</p>
			<p class="section-copy">
				World time uses fixed {formatUTCOffset(workPlan.world_utc_offset_minutes)}; your local
				daylight-saving changes do not alter game outcomes.
			</p>
		</div>
	</div>
	<div class="workers">
		{#each workers as character (character.id)}<WorkerCard
				{character}
				duty={dutyFor(character.id)}
				{currentGameDay}
				workday={workPlan.workday}
				selected={selectedWorker?.id === character.id}
				onselect={() => (selectedCharacterId = character.id)}
			/>{/each}
	</div>
	<WorkAssignmentList duties={workPlan.temporary_duties} {characters} {currentGameDay} />
	<form
		method="POST"
		action="?/occupationPreview"
		bind:this={previewForm}
		use:enhance={({ submitter }) => {
			const version = previewVersion;
			const changing = submitter?.getAttribute('formaction') === '?/changeOccupation';
			working = changing;
			return async ({ update }) => {
				if (!changing && version !== previewVersion) return;
				await update({ reset: changing });
				working = false;
			};
		}}
	>
		<div class="plan-heading">
			<div>
				<p class="eyebrow">Persistent plan</p>
				<h3>{selectedWorker?.name ?? 'Household'} occupation</h3>
			</div>
			{#if selectedWorker}<div class="worker-state">
					<strong>Next work start</strong><span
						>{formatMoment(workPlan.next_working_period, currentGameDay)} · automatic</span
					>
				</div>{/if}
		</div>
		<input type="hidden" name="expected_revision" value={expectedRevision} />
		<div class="form-grid">
			<label
				><span>Who?</span><select
					name="character_id"
					required
					bind:value={selectedCharacterId}
					aria-label="Character"
					>{#each workers as character (character.id)}<option value={character.id}
							>{character.name}</option
						>{/each}</select
				></label
			><label
				><span>Occupation</span><select name="activity" required bind:value={activity}
					>{#each ['agriculture', 'fishing', 'woodcutting'] as option (option)}<option
							value={option}>{labelActivity(option)}</option
						>{/each}</select
				></label
			>
		</div>
		{#if selectedOccupation}<p class="pending">
				Current: <strong>{labelActivity(selectedOccupation.activity)}</strong
				>{#if selectedOccupation.pending_activity && selectedOccupation.effective_game_day !== undefined && selectedOccupation.effective_hour !== undefined}
					· pending {labelActivity(selectedOccupation.pending_activity)}
					{formatMoment(
						{ day: selectedOccupation.effective_game_day, hour: selectedOccupation.effective_hour },
						currentGameDay
					)}{/if}
			</p>{/if}
		{#if feedback?.action === 'occupationPreview' && feedback.preview}<div
				class="plan-preview"
				aria-live="polite"
			>
				<div>
					<span>Baseline after 7 days</span><strong
						>{stock(feedback.preview.baseline_ending_stocks.provisions_milli)} food · {stock(
							feedback.preview.baseline_ending_stocks.wood_milli
						)} wood</strong
					>
				</div>
				<div>
					<span>Proposed after 7 days</span><strong
						>{stock(feedback.preview.proposed_ending_stocks.provisions_milli)} food · {stock(
							feedback.preview.proposed_ending_stocks.wood_milli
						)} wood</strong
					>
				</div>
				<div>
					<span>Minimum food stock</span><strong
						>{stock(feedback.preview.proposed_minimum_provisions_milli)}</strong
					>
				</div>
			</div>
			{#if feedback.preview.warnings.length}<p class="preview-warning" role="alert">
					{feedback.preview.warnings[0].message}
				</p>{/if}{:else if feedback?.action === 'occupationPreview' && feedback.message}<p
				class="preview-warning"
				role="alert"
			>
				{feedback.message}
			</p>{/if}
		<p class="decision-note">
			Choose the regular role that prepares the household for the next season. Temporary commitments
			pause home production without replacing this occupation.
		</p>
		<ActionFeedback feedback={feedback?.action === 'changeOccupation' ? feedback : null} /><button
			class="primary-action"
			type="submit"
			formaction="?/changeOccupation"
			disabled={working || !selectedWorker}>{working ? 'Updating…' : 'Change occupation'}</button
		>
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
		justify-content: space-between;
		gap: 1rem;
		align-items: start;
	}
	h3 {
		margin: 0.15rem 0 0;
		font-family: var(--font-display);
		font-size: 1.35rem;
	}
	.worker-state {
		display: grid;
		gap: 0.15rem;
		text-align: right;
		color: var(--ink-soft);
		font-size: 0.75rem;
	}
	.worker-state strong {
		color: var(--ink);
	}
	.form-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.75rem;
		margin-top: 1rem;
	}
	label {
		display: grid;
		gap: 0.35rem;
		color: var(--ink-soft);
		font-size: 0.75rem;
		font-weight: 800;
	}
	select {
		width: 100%;
		min-height: 2.55rem;
		border: 1px solid var(--line);
		background: var(--surface);
		color: var(--ink);
		padding: 0.5rem;
	}
	.pending {
		margin: 0.8rem 0 0;
		color: var(--ink-soft);
		font-size: 0.8rem;
	}
	.pending strong {
		color: var(--ink);
	}
	.plan-preview {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 0.6rem;
		margin-top: 1rem;
	}
	.plan-preview div {
		display: grid;
		gap: 0.25rem;
		padding: 0.7rem;
		background: var(--surface-muted);
		border: 1px solid var(--line-light);
	}
	.plan-preview span {
		color: var(--ink-soft);
		font-size: 0.68rem;
	}
	.plan-preview strong {
		font-size: 0.8rem;
	}
	.preview-warning {
		margin: 0.75rem 0 0;
		color: var(--critical);
		font-size: 0.8rem;
	}
	.decision-note {
		margin: 1rem 0;
		color: var(--ink-soft);
		font-size: 0.78rem;
	}
	.primary-action {
		width: 100%;
		min-height: 2.7rem;
		border: 0;
		background: var(--green);
		color: white;
		font-weight: 800;
	}
	@media (max-width: 620px) {
		.workers,
		.form-grid,
		.plan-preview {
			grid-template-columns: 1fr;
		}
		.plan-heading {
			flex-direction: column;
		}
		.worker-state {
			text-align: left;
		}
	}
</style>
