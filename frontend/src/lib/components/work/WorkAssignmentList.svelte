<script lang="ts">
	import type { Assignment } from '$lib/api/generated';
	import { labelActivity } from '$lib/domain/format';
	import { formatRelativeGameDay } from '$lib/domain/time';

	let { assignments, currentGameDay }: { assignments: Assignment[]; currentGameDay: number } =
		$props();
</script>

{#if assignments.length > 0}
	<section class="assignment-list" aria-labelledby="planned-heading">
		<h3 id="planned-heading">Planned work</h3>
		<ul>
			{#each assignments as assignment (assignment.id)}
				<li>
					<strong>{assignment.character}</strong><span
						>{labelActivity(assignment.activity)} · {formatRelativeGameDay(
							currentGameDay,
							assignment.starts_game_day ?? currentGameDay
						)}–{formatRelativeGameDay(
							currentGameDay,
							assignment.ends_game_day ?? currentGameDay
						)}</span
					>
				</li>
			{/each}
		</ul>
	</section>
{/if}

<style>
	.assignment-list {
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
	@media (max-width: 480px) {
		li {
			align-items: flex-start;
			flex-direction: column;
			gap: 0.15rem;
		}
	}
</style>
