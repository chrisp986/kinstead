<script lang="ts">
	import type { ChronicleEntry, ReportItem } from '$lib/api/generated';
	import { describeChronicleEntry } from '$lib/domain/chronicle';
	import { describeReportItem } from '$lib/domain/report';

	let {
		report
	}: {
		report: {
			recent_changes: ChronicleEntry[];
			attention: ReportItem[];
			decisions: ReportItem[];
		};
	} = $props();

	function anchor(target?: string): string {
		return `#${target === 'contracts' ? 'contracts' : target === 'politics' ? 'politics' : target === 'work' ? 'work' : target === 'farm' ? 'farm' : 'trade'}`;
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
		<section aria-labelledby="recent-heading">
			<h3 id="recent-heading">Recent changes</h3>
			{#if report.recent_changes.length === 0}<p class="empty">No recent changes.</p>{:else}<ul>
					{#each report.recent_changes.slice(0, 3) as entry (entry.id)}{@const description =
							describeChronicleEntry(entry)}
						<li><strong>{description.title}</strong><span>{description.detail}</span></li>{/each}
				</ul>{/if}
		</section>
		<section aria-labelledby="attention-heading">
			<h3 id="attention-heading">Needs attention</h3>
			{#if report.attention.length === 0}<p class="empty">Nothing urgent.</p>{:else}<ul>
					{#each report.attention as item (item.code + item.related_id)}<li
							class:critical={item.severity === 'critical'}
						>
							{describeReportItem(item)}
						</li>{/each}
				</ul>{/if}
		</section>
		<section aria-labelledby="decisions-heading">
			<h3 id="decisions-heading">Decide now</h3>
			{#if report.decisions.length === 0}<p class="empty">No immediate decisions.</p>{:else}<ol>
					{#each report.decisions as item (item.code + item.related_id)}<li>
							<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
							<a href={anchor(item.target)}>{describeReportItem(item)}</a>
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
	.empty {
		margin: 0;
		color: var(--ink-soft);
		font-size: 0.84rem;
	}
	@media (max-width: 800px) {
		.report-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
