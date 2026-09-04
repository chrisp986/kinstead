<script lang="ts">
	import HouseholdHeader from '$lib/components/shell/HouseholdHeader.svelte';
	import HouseholdNav from '$lib/components/shell/HouseholdNav.svelte';

	let { data, children } = $props();
</script>

<HouseholdHeader report={data.report} />
<HouseholdNav householdId={data.report.household_id} />
<div class="household-content">{@render children()}</div>

<style>
	.household-content {
		padding-bottom: 1.5rem;
	}
	@media (max-width: 767px) {
		.household-content {
			padding-bottom: calc(var(--mobile-nav-height) + env(safe-area-inset-bottom) + 1rem);
		}
	}
</style>
