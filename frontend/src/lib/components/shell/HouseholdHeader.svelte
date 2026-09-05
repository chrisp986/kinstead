<script lang="ts">
	import { resolve } from '$app/paths';
	import type { HouseholdReport } from '$lib/api/generated';
	import {
		formatCalendarPosition,
		formatRelativeGameDay,
		nextHalfYearStart,
		settingYear
	} from '$lib/domain/time';

	let { report }: { report: HouseholdReport } = $props();
	let workerCount = $derived(
		report.characters.filter((character) => character.labor_permille > 0).length
	);
	let assignedCount = $derived.by(() => {
		const workerIds = new Set(
			report.characters
				.filter((character) => character.labor_permille > 0)
				.map((character) => character.id)
		);
		return new Set(
			report.assignments
				.filter((assignment) => workerIds.has(assignment.character_id))
				.map((assignment) => assignment.character_id)
		).size;
	});
	let tiredCount = $derived(
		report.characters.filter((character) => character.fatigue >= 50).length
	);
	let matterCount = $derived(report.attention.length + report.decisions.length);
	let supplyTone = $derived(
		report.supply_days < 7
			? 'emergency'
			: report.supply_days < 15
				? 'critical'
				: report.supply_days <= 30
					? 'warning'
					: 'safe'
	);
	let supplyState = $derived(
		report.supply_days < 7
			? 'Emergency'
			: report.supply_days < 15
				? 'Critical'
				: report.supply_days <= 30
					? 'Strained'
					: 'Secure'
	);

	function householdHref(suffix = ''): string {
		return resolve(`/households/${report.household_id}${suffix}` as `/households/${string}`);
	}
</script>

<header class="household-header">
	<div class="identity">
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

	<div class="status-strip" aria-label="Household status">
		<a class={`status ${supplyTone}`} href={householdHref('/farm')}>
			<span>Provisions</span>
			<strong>{report.supply_days} days</strong>
			<small>{supplyState}</small>
		</a>
		<a class="status" href={householdHref('/work')}>
			<span>Labor</span>
			<strong>{assignedCount}/{workerCount}</strong>
			<small>workers planned</small>
		</a>
		<a class:tired={tiredCount > 0} class="status" href={householdHref('/work')}>
			<span>Fatigue</span>
			<strong>{tiredCount}</strong>
			<small>{tiredCount === 1 ? 'person strained' : 'people strained'}</small>
		</a>
		<a class:urgent={matterCount > 0} class="status" href={householdHref()}>
			<span>Matters</span>
			<strong>{matterCount}</strong>
			<small>{matterCount === 1 ? 'needs review' : 'need review'}</small>
		</a>
	</div>
</header>

<style>
	.household-header {
		display: grid;
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
	.status-strip {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		border: 1px solid var(--line-light);
		background: var(--surface);
	}
	.status {
		display: grid;
		gap: 0.1rem;
		min-width: 0;
		padding: 0.65rem 0.75rem;
		border-right: 1px solid var(--line-light);
		color: var(--ink);
		text-decoration: none;
	}
	.status:last-child {
		border-right: 0;
	}
	.status:hover,
	.status:focus-visible {
		background: var(--surface-muted);
	}
	.status span,
	.status small {
		color: var(--ink-soft);
		font-size: 0.67rem;
		line-height: 1.2;
	}
	.status span {
		font-weight: 800;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.status strong {
		font-family: var(--font-display);
		font-size: 1.18rem;
		font-weight: 600;
		line-height: 1.1;
	}
	.status.safe strong {
		color: var(--positive);
	}
	.status.warning strong,
	.status.tired strong {
		color: var(--warning);
	}
	.status.critical strong,
	.status.emergency strong,
	.status.urgent strong {
		color: var(--critical);
	}
	.status.emergency {
		background: color-mix(in srgb, var(--critical) 8%, var(--surface));
	}
	@media (min-width: 900px) {
		.household-header {
			grid-template-columns: minmax(15rem, 1fr) minmax(30rem, 1.45fr);
			align-items: end;
			padding-top: 1.75rem;
		}
		h1 {
			font-size: clamp(2rem, 4vw, 2.65rem);
		}
	}
	@media (max-width: 620px) {
		.status-strip {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
		.status:nth-child(2) {
			border-right: 0;
		}
		.status:nth-child(-n + 2) {
			border-bottom: 1px solid var(--line-light);
		}
	}
</style>
