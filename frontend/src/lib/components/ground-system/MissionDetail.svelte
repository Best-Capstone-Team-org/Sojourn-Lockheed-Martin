<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import Led from "./Led.svelte";
	import Screen from "../Screen/Screen.svelte";
	import type { ObjectiveDetail } from "$lib/api";

	let { objective }: { objective: ObjectiveDetail } = $props();
	let hintsOpen = $state(false);
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

			{#if objective.diagnostic}
				<Screen class="mb-2 p-3">
					<p class="text-sm">{objective.diagnostic}</p>
				</Screen>
			{/if}

			{#if objective.hints?.length}
				<button
					class="mb-2 cursor-pointer text-sm font-semibold underline"
					type="button"
					aria-expanded={hintsOpen}
					onclick={() => (hintsOpen = !hintsOpen)}
				>
					{hintsOpen ? "Hide hints" : "Show hints"}
				</button>
				{#if hintsOpen}
					<div class="border-l-4 border-sys-shadow-bottom bg-black/5 p-3">
						<p class="text-sm">{objective.hints.join("\n")}</p>
					</div>
				{/if}
			{/if}
		</div>
	{/if}
</Box>
