<script lang="ts">
	import type { Assignment, Character } from '$lib/api/generated';
	import { labelActivity } from '$lib/domain/format';

	let { characters, assignments }: { characters: Character[]; assignments: Assignment[] } =
		$props();

	function assignmentFor(characterId: string) {
		return assignments.find((value) => value.character_id === characterId);
	}

	function state(character: Character, assignment?: Assignment): string {
		if (character.labor_permille <= 0) return 'Unavailable';
		if (!assignment) return 'Available';
		if (assignment.activity === 'rest') return 'Resting';
		if (assignment.activity === 'ruler_service') return 'Ruler service';
		return assignment.status === 'planned' ? 'Planned' : 'Working';
	}
</script>

<section class="panel" aria-labelledby="members-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">People of the farmstead</p>
			<h2 id="members-heading">Household members</h2>
		</div>
	</div>
	{#if characters.length === 0}
		<p class="empty">No household members are recorded.</p>
	{:else}
		<div class="members">
			{#each characters as character (character.id)}
				{@const assignment = assignmentFor(character.id)}
				{@const currentState = state(character, assignment)}
				<article class="member">
					<div class="member-top">
						<div class="avatar" aria-hidden="true">{character.name.slice(0, 1)}</div>
						<div>
							<h3>{character.name}</h3>
							<p>
								{character.age} years{character.specialization
									? ` · ${labelActivity(character.specialization)}`
									: ''}
							</p>
						</div>
						<span class="state" data-state={currentState}>{currentState}</span>
					</div>
					{#if assignment}<p class="assignment">
							{labelActivity(assignment.activity)} · ticks {assignment.starts_tick}–{assignment.ends_tick}
						</p>{:else}<p class="assignment">No work planned</p>{/if}
					<div class="fatigue">
						<span>Fatigue {character.fatigue}</span>
						<progress max="100" value={character.fatigue} aria-label={`${character.name} fatigue`}
						></progress>
					</div>
				</article>
			{/each}
		</div>
	{/if}
</section>

<style>
	.members {
		display: grid;
		gap: 0.75rem;
		margin-top: 1.1rem;
	}
	.member {
		padding: 0.9rem;
		border: 1px solid var(--line-light);
		background: var(--surface);
	}
	.member-top {
		display: flex;
		align-items: center;
		gap: 0.7rem;
	}
	.avatar {
		display: grid;
		place-items: center;
		flex: 0 0 2.35rem;
		width: 2.35rem;
		height: 2.35rem;
		border-radius: 50%;
		background: var(--green);
		color: white;
		font-family: var(--font-display);
		font-size: 1.1rem;
	}
	h3 {
		margin: 0;
		font-family: var(--font-display);
		font-size: 1.08rem;
	}
	.member-top p {
		margin: 0.15rem 0 0;
		color: var(--ink-soft);
		font-size: 0.78rem;
	}
	.state {
		margin-left: auto;
		padding: 0.3rem 0.45rem;
		border: 1px solid var(--line);
		color: var(--ink-soft);
		font-size: 0.7rem;
		font-weight: 700;
	}
	.state[data-state='Working'],
	.state[data-state='Ruler service'] {
		border-color: var(--green);
		color: var(--green);
	}
	.state[data-state='Unavailable'] {
		color: var(--critical);
	}
	.assignment {
		margin: 0.75rem 0 0;
		color: var(--ink-soft);
		font-size: 0.8rem;
	}
	.fatigue {
		display: grid;
		grid-template-columns: auto 1fr;
		align-items: center;
		gap: 0.6rem;
		margin-top: 0.7rem;
		color: var(--ink-soft);
		font-size: 0.72rem;
	}
	progress {
		width: 100%;
		height: 0.4rem;
		accent-color: var(--ochre);
	}
	@media (min-width: 700px) {
		.members {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
	}
</style>
