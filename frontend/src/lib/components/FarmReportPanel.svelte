<script lang="ts">
	import type { ChronicleEntry, ReportItem } from '$lib/api/generated';
	import { resolve } from '$app/paths';
	import { describeChronicleEntry } from '$lib/domain/chronicle';
	import { describeReportItem } from '$lib/domain/report';

	let {
		report,
		householdId
	}: {
		report: {
			household_id: string;
			recent_changes: ChronicleEntry[];
			attention: ReportItem[];
			decisions: ReportItem[];
		};
		householdId?: string;
	} = $props();

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
							{describeReportItem(item)}
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
								<span>{describeReportItem(item)}</span>
								<span class="review">Review decision →</span>
							</a>
						</li>{/each}
				</ol>{/if}
		</section>
	</div>
</section>

<style>
	.report-panel {
		grid-column: 1 / -1;
	}
	.report-grid {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
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
			grid-template-columns: 1fr;
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
</style>
