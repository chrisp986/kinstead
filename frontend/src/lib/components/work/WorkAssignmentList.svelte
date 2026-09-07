<script lang="ts">
	import type { Assignment } from '$lib/api/generated';
	import { formatRelativeGameDay } from '$lib/domain/time';
	let { assignments, currentGameDay }: { assignments: Assignment[]; currentGameDay: number } = $props();
	let duties = $derived(assignments.filter((assignment) => assignment.activity === 'ruler_service'));
</script>

{#if duties.length > 0}<section class="assignment-list" aria-labelledby="planned-heading"><h3 id="planned-heading">Temporary commitments</h3><ul>{#each duties as assignment (assignment.id)}<li><strong>{assignment.character}</strong><span>Jarl service · returns {formatRelativeGameDay(currentGameDay, assignment.ends_game_day ?? currentGameDay)}</span></li>{/each}</ul></section>{:else}<p class="empty">No temporary commitments. Occupations continue automatically.</p>{/if}

<style>
	.assignment-list, .empty { margin-top: 1rem; padding-top: 1rem; border-top: 1px solid var(--line-light); } h3 { margin: 0; font-family: var(--font-display); font-size: 1.1rem; } ul { display: grid; gap: 0.5rem; margin: 0.7rem 0 0; padding: 0; list-style: none; } li { display: flex; justify-content: space-between; gap: 1rem; color: var(--ink-soft); font-size: 0.78rem; } li strong { color: var(--ink); } .empty { color: var(--ink-soft); font-size: 0.78rem; } @media (max-width: 480px) { li { align-items: flex-start; flex-direction: column; gap: 0.15rem; } }
</style>
