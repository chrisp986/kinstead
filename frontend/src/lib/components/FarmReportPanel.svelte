<script lang="ts">
	import type { CalendarEvent, ChronicleEntry, ReportItem } from '$lib/api/generated';
	import { resolve } from '$app/paths';
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
			.slice(0, 3);
	});

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

<section id="farm" class="panel report-panel" aria-labelledby="farm-report-heading">
	<div class="section-heading">
		<div>
			<p class="eyebrow">Return briefing</p>
			<h2 id="farm-report-heading">Farm report</h2>
		</div>
	</div>
	<div class="report-grid">
		<section class="recent" aria-labelledby="recent-heading">
			<h3 id="recent-heading">Recent changes</h3>
			{#if report.recent_changes.length === 0}<p class="empty">No recent changes.</p>{:else}<ul>
					{#each report.recent_changes.slice(0, 3) as entry (entry.id)}{@const description =
							describeChronicleEntry(entry)}
						<li><strong>{description.title}</strong><span>{description.detail}</span></li>{/each}
				</ul>{/if}
		</section>
		<section class="attention" aria-labelledby="attention-heading">
			<h3 id="attention-heading">Needs attention</h3>
			{#if report.attention.length === 0}<p class="empty">Nothing urgent.</p>{:else}<ul>
					{#each report.attention as item (item.code + item.related_id)}<li
							class:critical={item.severity === 'critical'}
						>
							{describeReportItem(item, report.game_day)}
						</li>{/each}
				</ul>{/if}
		</section>
		<section class="decisions" aria-labelledby="decisions-heading">
			<h3 id="decisions-heading">Decide now</h3>
			{#if report.decisions.length === 0}<p class="empty">No immediate decisions.</p>{:else}<ol>
					{#each report.decisions as item (item.code + item.related_id)}<li>
							<!-- decisionHref() resolves the dynamic household route and anchor. -->
							<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
							<a class="decision-card" href={decisionHref(item.target)}>
								<strong>{item.target ?? 'Household decision'}</strong>
								<span>{describeReportItem(item, report.game_day)}</span>
								<span class="review">Review decision →</span>
							</a>
						</li>{/each}
				</ol>{/if}
		</section>
		<section class="upcoming" aria-labelledby="upcoming-heading">
			<h3 id="upcoming-heading">Upcoming</h3>
			{#if upcoming.length === 0}
				<p class="empty">No upcoming calendar events.</p>
			{:else}
				<ul>
					{#each upcoming as event (event.id)}
						<li class:critical={event.action_required}>
							<strong>{formatRelativeGameDay(report.game_day, event.game_day)}</strong>
							<span>{event.title}</span>
						</li>
					{/each}
				</ul>
			{/if}
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a class="calendar-link" href={calendarHref()}>View calendar →</a>
		</section>
	</div>
</section>

<style>
	.report-panel {
		grid-column: 1 / -1;
	}
	.report-grid {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 1rem;
		margin-top: 1rem;
	}
	.report-grid section {
		padding: 0.9rem;
		background: var(--surface-muted);
	}
	.decisions {
		order: 3;
	}
	.attention {
		order: 2;
	}
	.recent {
		order: 1;
	}
	.report-grid h3 {
		margin: 0 0 0.6rem;
		font-family: var(--font-display);
		font-size: 1.05rem;
	}
	.report-grid ul,
	.report-grid ol {
		display: grid;
		gap: 0.55rem;
		margin: 0;
		padding-left: 1.1rem;
	}
	.report-grid li {
		color: var(--ink-soft);
		font-size: 0.84rem;
	}
	.report-grid li strong,
	.report-grid li span {
		display: block;
	}
	.report-grid li strong {
		color: var(--ink);
	}
	.report-grid li.critical {
		color: var(--critical);
	}
	.upcoming ul {
		display: grid;
		gap: 0.55rem;
		margin: 0;
		padding-left: 1.1rem;
	}
	.upcoming li strong,
	.upcoming li span {
		display: block;
	}
	.upcoming li strong {
		color: var(--ink);
	}
	.upcoming li {
		color: var(--ink-soft);
		font-size: 0.84rem;
	}
	.calendar-link {
		display: inline-block;
		margin-top: 0.8rem;
		color: var(--green);
		font-size: 0.78rem;
		font-weight: 700;
	}
	.report-grid a {
		color: var(--green);
		font-weight: 700;
	}
	.decision-card {
		display: grid;
		gap: 0.2rem;
		min-height: 7rem;
		padding: 0.85rem;
		border: 1px solid var(--line);
		background: var(--surface);
		text-decoration: none;
	}
	.decision-card:hover,
	.decision-card:focus-visible {
		background: #edf2e8;
	}
	.decision-card strong {
		font-size: 0.68rem;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.decision-card span {
		color: var(--ink);
		font-size: 0.9rem;
	}
	.decision-card .review {
		margin-top: auto;
		color: var(--green);
		font-size: 0.78rem;
	}
	.empty {
		margin: 0;
		color: var(--ink-soft);
		font-size: 0.84rem;
	}
	@media (max-width: 800px) {
		.report-grid {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
		.decisions {
			order: 1;
		}
		.attention {
			order: 2;
		}
		.recent {
			order: 3;
		}
	}
	@media (max-width: 520px) {
		.report-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
