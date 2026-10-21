<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import Led from "./Led.svelte";
	import Screen from "../Screen/Screen.svelte";
	import type { ObjectiveDetail } from "$lib/api";

	let { objective }: { objective: ObjectiveDetail } = $props();
</script>

<Box class="w-full p-2">
	<div class="flex items-center justify-between">
		<p class="font-semibold">{objective.name}</p>
		<div class="flex items-center gap-2">
			<span>{objective.status}</span>
			{#if objective.status === "active"}
				<Led color="yellow" />
			{:else if objective.status === "complete"}
				<Led color="green" />
			{:else if objective.status === "failed"}
				<Led color="red" />
			{:else}
				<Led />
			{/if}
		</div>
	</div>

	{#if objective.status !== "locked"}
		<hr
			class="my-1 border-t border-black/35 shadow-[0_1px_0_rgba(255,255,255,0.4)]"
		/>
		<div class="p-2">
			<p class="mb-2 text-sm">{objective.description}</p>

			{#if objective.hint && objective.status !== "complete"}
				<Screen class="p-3">
					<p class="text-sm">{objective.hint.join("\n")}</p>
				</Screen>
			{/if}
		</div>
	{/if}
</Box>
