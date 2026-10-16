<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import MissionDetail from "$lib/components/ground-system/MissionDetail.svelte";
	import MissionFooter from "./MissionFooter.svelte";
	import type { MissionDetailType } from "$lib/types";

	// dummy data TODO remove
	const writes = $state(100);
	const reads = $state(200);
	const maxWrites = $state(1000);
	const maxReads = $state(1000);

	let { missionDetails }: { missionDetails: MissionDetailType[] } = $props();
</script>

<div class="flex h-full min-h-0 flex-col">
	<Box class="w-full shrink-0 p-2">
		<span class="text-xl font-bold">Mission Details</span>
	</Box>
	<div class="min-h-0 flex-1 overflow-y-auto">
		{#each missionDetails as missionDetail (missionDetail.id)}
			<MissionDetail
				title={missionDetail.title}
				status={missionDetail.status}
				description={missionDetail.description}
				diagnostic={missionDetail.diagnostic}
			/>
		{/each}
	</div>
	<MissionFooter
		{writes}
		{reads}
		{maxWrites}
		{maxReads}
	/>
</div>
