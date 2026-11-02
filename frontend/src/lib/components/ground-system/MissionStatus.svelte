<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import MissionFooter from "./MissionFooter.svelte";
	import MissionDetail from "./MissionDetail.svelte";
	import type { ObjectiveDetail } from "$lib/api";

	let {
		objectives,
		writes,
		reads,
		maxWrites,
		maxReads,
	}: {
		objectives: Record<string, ObjectiveDetail>;
		writes: number;
		reads: number;
		maxWrites: number;
		maxReads: number;
	} = $props();

	const missionComplete = $derived(
		Object.values(objectives).length > 0 &&
			Object.values(objectives).every(
				(objective) => objective.status === "complete"
			)
	);
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
