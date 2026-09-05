<script lang="ts">
	import type { CalendarEvent, ChronicleEntry, ReportItem } from '$lib/api/generated';
	import { resolve } from '$app/paths';
	import { calendarEventLabel } from '$lib/domain/calendar';
	import { describeChronicleEntry } from '$lib/domain/chronicle';
	import { describeReportItem } from '$lib/domain/report';
	import { formatRelativeGameDay } from '$lib/domain/time';

	let {
		report,
		householdId,
		calendar
	}: {
		report: {
			household_id: string;
			recent_changes: ChronicleEntry[];
			attention: ReportItem[];
			decisions: ReportItem[];
			game_day: number;
		};
		householdId?: string;
		calendar?: { events: CalendarEvent[] };
	} = $props();

	let upcoming = $derived.by(() => {
		const importance = { critical: 0, important: 1, context: 2 };
		return (calendar?.events ?? [])
			.filter((event) => event.game_day >= report.game_day)
			.toSorted((a, b) => {
				if (a.action_required !== b.action_required) return a.action_required ? -1 : 1;
				if (importance[a.importance] !== importance[b.importance]) {
					return importance[a.importance] - importance[b.importance];
				}
				if (a.game_day !== b.game_day) return a.game_day - b.game_day;
				return a.id.localeCompare(b.id);
			})
			.slice(0, 4);
	});
	let criticalAttention = $derived(
		report.attention.filter((item) => item.severity === 'critical').length
	);

	function decisionHref(target?: string): string {
		const id = householdId ?? report.household_id;
		const base = `/households/${id}`;
		if (target === 'politics') return resolve(`${base}/farm#politics` as `/households/${string}`);
		if (target === 'contracts')
			return resolve(`${base}/trade#contracts` as `/households/${string}`);
		if (target === 'work') return resolve(`${base}/work` as `/households/${string}`);
		if (target === 'farm') return resolve(`${base}/farm` as `/households/${string}`);
		return resolve(`${base}/trade` as `/households/${string}`);
	}
	function calendarHref(): string {
		const id = householdId ?? report.household_id;
		return resolve(`/households/${id}/calendar` as `/households/${string}`);
	}
</script>

<section id="farm" class="panel report-panel" aria-labelledby="decision-heading">
	<header class="briefing-heading">
		<div>
			<p class="eyebrow">Priority briefing</p>
			<h2 id="decision-heading">What needs your decision?</h2>
			<p>
				Resolve the few choices that change the household before reviewing background information.
			</p>
		</div>
		<div
			class:clear={report.decisions.length === 0}
			class="matter-count"
			aria-label="Immediate decisions"
		>
			<strong>{report.decisions.length}</strong>
			<span>{report.decisions.length === 1 ? 'decision' : 'decisions'}</span>
		</div>
	</header>

	<section class="decisions" aria-labelledby="decisions-heading">
		<h3 id="decisions-heading" class="sr-label">Immediate decisions</h3>
		{#if report.decisions.length === 0}
			<div class="calm-state">
				<strong>No immediate decision is waiting.</strong>
				<span
					>Use the risks and timeline below to prepare before pressure turns into a deadline.</span
				>
			</div>
		{:else}
			<ol>
				{#each report.decisions as item (item.code + item.related_id)}
					<li>
						<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
						<a
							class:critical={item.severity === 'critical'}
							class="decision-card"
							href={decisionHref(item.target)}
						>
							<span class="decision-kicker">{item.target ?? 'Household decision'}</span>
							<strong>{describeReportItem(item, report.game_day)}</strong>
							<span class="review">Review and decide →</span>
						</a>
					</li>
				{/each}
			</ol>
		{/if}
	</section>

	<div class="support-grid">
		<section class="attention" aria-labelledby="attention-heading">
			<div class="support-heading">
				<h3 id="attention-heading">Risks to watch</h3>
				{#if criticalAttention > 0}<span class="alert-count">{criticalAttention} critical</span
					>{/if}
			</div>
			{#if report.attention.length === 0}
				<p class="empty">Nothing currently needs attention.</p>
			{:else}
				<ul class="attention-list">
					{#each report.attention as item (item.code + item.related_id)}
						<li class:critical={item.severity === 'critical'}>
							<span class="risk-marker" aria-hidden="true"></span>
							<span>{describeReportItem(item, report.game_day)}</span>
						</li>
					{/each}
				</ul>
			{/if}
		</section>

		<section class="upcoming" aria-labelledby="upcoming-heading">
			<div class="support-heading">
				<h3 id="upcoming-heading">Next on the calendar</h3>
				<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
				<a class="calendar-link" href={calendarHref()}>Full calendar →</a>
			</div>
			{#if upcoming.length === 0}
				<p class="empty">No upcoming calendar events.</p>
			{:else}
				<ol class="timeline">
					{#each upcoming as event (event.id)}
						<li class:action={event.action_required}>
							<div class="timeline-marker" aria-hidden="true"></div>
							<div class="timeline-copy">
								<strong>{formatRelativeGameDay(report.game_day, event.game_day)}</strong>
								<span>{calendarEventLabel(event)}</span>
								{#if event.action_required}<small>Action required</small>{/if}
							</div>
						</li>
					{/each}
				</ol>
			{/if}
		</section>
	</div>

	<section class="recent" aria-labelledby="recent-heading">
		<h3 id="recent-heading">Since your last visit</h3>
		{#if report.recent_changes.length === 0}
			<p class="empty">No recent changes.</p>
		{:else}
			<ul>
				{#each report.recent_changes.slice(0, 4) as entry (entry.id)}
					{@const description = describeChronicleEntry(entry)}
					<li><strong>{description.title}</strong><span>{description.detail}</span></li>
				{/each}
			</ul>
		{/if}
	</section>
</section>

<style>
	.report-panel {
		grid-column: 1 / -1;
	}
	.briefing-heading {
		display: flex;
		align-items: start;
		justify-content: space-between;
		gap: 1rem;
	}
	.briefing-heading h2 {
		margin: 0.15rem 0 0;
		font-family: var(--font-display);
		font-size: clamp(1.45rem, 4vw, 1.9rem);
		font-weight: 500;
	}
	.briefing-heading p:not(.eyebrow) {
		max-width: 42rem;
		margin: 0.4rem 0 0;
		color: var(--ink-soft);
		font-size: 0.86rem;
	}
	.matter-count {
		display: grid;
		min-width: 5.25rem;
		padding: 0.65rem;
		border: 1px solid color-mix(in srgb, var(--critical) 45%, var(--line-light));
		background: color-mix(in srgb, var(--critical) 7%, var(--surface));
		text-align: center;
	}
	.matter-count.clear {
		border-color: var(--line-light);
		background: var(--surface-muted);
	}
	.matter-count strong {
		color: var(--critical);
		font-family: var(--font-display);
		font-size: 1.6rem;
		line-height: 1;
	}
	.matter-count.clear strong {
		color: var(--positive);
	}
	.matter-count span {
		margin-top: 0.15rem;
		color: var(--ink-soft);
		font-size: 0.68rem;
		font-weight: 700;
		text-transform: uppercase;
	}
	.decisions {
		margin-top: 1.1rem;
	}
	.decisions ol {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.8rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.decision-card {
		display: grid;
		gap: 0.45rem;
		min-height: 9rem;
		padding: 1rem;
		border: 1px solid var(--line);
		border-top: 3px solid var(--ochre);
		background: var(--surface);
		color: var(--ink);
		text-decoration: none;
	}
	.decision-card.critical {
		border-top-color: var(--critical);
	}
	.decision-card:hover,
	.decision-card:focus-visible {
		background: #edf2e8;
	}
	.decision-kicker {
		color: var(--ochre);
		font-size: 0.68rem;
		font-weight: 800;
		letter-spacing: 0.09em;
		text-transform: uppercase;
	}
	.decision-card.critical .decision-kicker {
		color: var(--critical);
	}
	.decision-card strong {
		font-family: var(--font-display);
		font-size: 1.08rem;
		font-weight: 500;
		line-height: 1.35;
	}
	.decision-card .review {
		align-self: end;
		margin-top: auto;
		color: var(--green);
		font-size: 0.78rem;
		font-weight: 800;
	}
	.calm-state {
		display: grid;
		gap: 0.25rem;
		padding: 1rem;
		border: 1px solid var(--line-light);
		border-left: 3px solid var(--positive);
		background: var(--surface-muted);
	}
	.calm-state span {
		color: var(--ink-soft);
		font-size: 0.82rem;
	}
	.support-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.8rem;
		margin-top: 1.25rem;
	}
	.support-grid > section,
	.recent {
		padding: 1rem;
		background: var(--surface-muted);
	}
	.support-heading {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 0.75rem;
	}
	.support-grid h3,
	.recent h3 {
		margin: 0 0 0.75rem;
		font-family: var(--font-display);
		font-size: 1.08rem;
		font-weight: 600;
	}
	.alert-count {
		color: var(--critical);
		font-size: 0.7rem;
		font-weight: 800;
		text-transform: uppercase;
	}
	.attention-list,
	.timeline,
	.recent ul {
		display: grid;
		gap: 0.65rem;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.attention-list li {
		display: grid;
		grid-template-columns: 0.55rem 1fr;
		gap: 0.55rem;
		align-items: start;
		color: var(--ink-soft);
		font-size: 0.84rem;
	}
	.risk-marker {
		width: 0.5rem;
		height: 0.5rem;
		margin-top: 0.3rem;
		border-radius: 50%;
		background: var(--warning);
	}
	.attention-list li.critical {
		color: var(--critical);
	}
	.attention-list li.critical .risk-marker {
		background: var(--critical);
	}
	.timeline li {
		position: relative;
		display: grid;
		grid-template-columns: 0.75rem 1fr;
		gap: 0.55rem;
	}
	.timeline li:not(:last-child)::after {
		position: absolute;
		top: 0.75rem;
		bottom: -0.7rem;
		left: 0.34rem;
		width: 1px;
		background: var(--line);
		content: '';
	}
	.timeline-marker {
		position: relative;
		z-index: 1;
		width: 0.7rem;
		height: 0.7rem;
		margin-top: 0.2rem;
		border: 2px solid var(--green);
		border-radius: 50%;
		background: var(--surface-muted);
	}
	.timeline li.action .timeline-marker {
		border-color: var(--critical);
		background: var(--critical);
	}
	.timeline-copy {
		display: grid;
		gap: 0.1rem;
	}
	.timeline-copy strong {
		color: var(--ink);
		font-size: 0.78rem;
	}
	.timeline-copy span {
		color: var(--ink-soft);
		font-size: 0.84rem;
	}
	.timeline-copy small {
		color: var(--critical);
		font-size: 0.67rem;
		font-weight: 800;
		text-transform: uppercase;
	}
	.calendar-link {
		color: var(--green);
		font-size: 0.72rem;
		font-weight: 800;
		white-space: nowrap;
	}
	.recent {
		margin-top: 0.8rem;
		border-top: 1px solid var(--line-light);
	}
	.recent li {
		display: grid;
		grid-template-columns: minmax(9rem, 0.7fr) minmax(0, 1.3fr);
		gap: 0.75rem;
		color: var(--ink-soft);
		font-size: 0.82rem;
	}
	.recent li strong {
		color: var(--ink);
	}
	.empty {
		margin: 0;
		color: var(--ink-soft);
		font-size: 0.84rem;
	}
	.sr-label {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
		white-space: nowrap;
		border: 0;
	}
	@media (max-width: 760px) {
		.decisions ol,
		.support-grid {
			grid-template-columns: 1fr;
		}
		.recent li {
			grid-template-columns: 1fr;
			gap: 0.15rem;
		}
	}
	@media (max-width: 520px) {
		.briefing-heading {
			align-items: center;
		}
		.briefing-heading p:not(.eyebrow) {
			display: none;
		}
		.matter-count {
			min-width: 4.5rem;
		}
		.decision-card {
			min-height: 8rem;
		}
	}
</style>
