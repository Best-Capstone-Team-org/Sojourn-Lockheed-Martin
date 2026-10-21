<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import MissionFooter from "./MissionFooter.svelte";
	import MissionDetail from "./MissionDetail.svelte";
	import type { ObjectiveDetail } from "$lib/api";

	// dummy data TODO remove
	const writes = $state(100);
	const reads = $state(200);
	const maxWrites = $state(1000);
	const maxReads = $state(1000);
	let missionComplete = $state(false);

	let { objectives }: { objectives: Record<string, ObjectiveDetail> } =
		$props();

	// check if all objectives are complete
	$effect(() => {
		if (
			Object.values(objectives).every(
				(objective) => objective.status === "complete"
			)
		) {
			missionComplete = true;
		}
	});
</script>

<div class="flex h-full min-h-0 flex-col">
	<Box class="w-full shrink-0 p-2">
		<span class="text-xl font-bold">Mission Details</span>
	</Box>
	<div class="min-h-0 flex-1 overflow-y-auto">
		{#each Object.values(objectives) as objective (objective.id)}
			<MissionDetail {objective} />
		{/each}
	</div>
	<MissionFooter
		{writes}
		{reads}
		{maxWrites}
		{maxReads}
		{missionComplete}
	/>
</div>
