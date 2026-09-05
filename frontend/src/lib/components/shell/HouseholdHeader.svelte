<script lang="ts">
	import type { HouseholdReport } from '$lib/api/generated';
	import {
		formatCalendarPosition,
		formatRelativeGameDay,
		nextHalfYearStart,
		settingYear
	} from '$lib/domain/time';

	let { report }: { report: HouseholdReport } = $props();
</script>

<header class="household-header">
	<div>
		<p class="eyebrow">Household seat</p>
		<h1>{report.household_name}</h1>
		<p class="date">
			{settingYear(report.setting_start_year, report.calendar)} CE · {formatCalendarPosition(
				report.calendar.phase ||
					report.calendar.seasonal_phase ||
					report.calendar.production_season,
				report.calendar.week_of_half
			)}
		</p>
		<p class="next-half">
			{formatRelativeGameDay(report.calendar.game_day, nextHalfYearStart(report.calendar.game_day))} until
			{report.calendar.half_year === 'summer' ? 'winter' : 'summer'} begins
		</p>
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
	.next-half {
		margin: 0.2rem 0 0;
		color: var(--ink-soft);
		font-size: 0.78rem;
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
