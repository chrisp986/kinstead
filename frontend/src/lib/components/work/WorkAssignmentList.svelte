<script lang="ts">
	import type { Character, TemporaryDuty } from '$lib/api/generated';
	import { formatMoment } from '$lib/domain/time';
	let {
		duties,
		characters,
		currentGameDay
	}: { duties: TemporaryDuty[]; characters: Character[]; currentGameDay: number } = $props();
	function characterName(id: string): string {
		return characters.find((character) => character.id === id)?.name ?? 'Household member';
	}
</script>

{#if duties.length > 0}<section class="assignment-list" aria-labelledby="planned-heading">
		<h3 id="planned-heading">Temporary commitments</h3>
		<ul>
			{#each duties as duty (duty.id)}<li>
					<strong>{characterName(duty.character_id)}</strong><span
						>Jarl service · returns {formatMoment(duty.ends, currentGameDay)}</span
					>
				</li>{/each}
		</ul>
	</section>{:else}<p class="empty">
		No temporary commitments. Occupations continue automatically.
	</p>{/if}

<style>
	.assignment-list,
	.empty {
		margin-top: 1rem;
		padding-top: 1rem;
		border-top: 1px solid var(--line-light);
	}
	h3 {
		margin: 0;
		font-family: var(--font-display);
		font-size: 1.1rem;
	}
	ul {
		display: grid;
		gap: 0.5rem;
		margin: 0.7rem 0 0;
		padding: 0;
		list-style: none;
	}
	li {
		display: flex;
		justify-content: space-between;
		gap: 1rem;
		color: var(--ink-soft);
		font-size: 0.78rem;
	}
	li strong {
		color: var(--ink);
	}
	.empty {
		color: var(--ink-soft);
		font-size: 0.78rem;
	}
	@media (max-width: 480px) {
		li {
			align-items: flex-start;
			flex-direction: column;
			gap: 0.15rem;
		}
	}
</style>
