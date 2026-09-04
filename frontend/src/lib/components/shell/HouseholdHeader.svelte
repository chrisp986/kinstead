<script lang="ts">
	import type { HouseholdReport } from '$lib/api/generated';
	import { formatGameDay, formatPhase } from '$lib/domain/time';

	let { report }: { report: HouseholdReport } = $props();
</script>

<header class="household-header">
	<div>
		<p class="eyebrow">Household seat</p>
		<h1>{report.household_name}</h1>
		<p class="date">{formatGameDay(report.calendar)} · {formatPhase(report.calendar)}</p>
	</div>
	<div class="tick" aria-label={`Game day ${report.game_day ?? report.tick}`}>
		<span>Game day</span>
		<strong>{report.game_day ?? report.tick}</strong>
	</div>
</header>

<style>
	.household-header {
		display: flex;
		align-items: end;
		justify-content: space-between;
		gap: 1rem;
		padding: 1.5rem 0 1rem;
		border-bottom: 1px solid var(--line);
	}
	h1 {
		margin: 0.15rem 0 0;
		font-family: var(--font-display);
		font-size: clamp(1.9rem, 6vw, 2.5rem);
		font-weight: 500;
		letter-spacing: -0.03em;
		line-height: 1;
	}
	.date {
		margin: 0.35rem 0 0;
		color: var(--ink-soft);
		font-size: 0.88rem;
		text-transform: capitalize;
	}
	.tick {
		display: grid;
		justify-items: end;
		min-width: 4.5rem;
	}
	.tick span {
		color: var(--ink-soft);
		font-size: 0.65rem;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.tick strong {
		font-family: var(--font-display);
		font-size: 1.8rem;
		line-height: 1;
	}
	@media (min-width: 768px) {
		.household-header {
			padding-top: 1.75rem;
		}
		h1 {
			font-size: clamp(2rem, 4vw, 2.65rem);
		}
	}
</style>
