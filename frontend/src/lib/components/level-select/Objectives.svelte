<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import Led from "$lib/components/ground-system/Led.svelte";
	import type { Constraints, ObjectiveResponse } from "$lib/api";

	let {
		objectives,
		constraints,
		open = $bindable(false),
	}: {
		objectives: ObjectiveResponse[];
		constraints: Constraints;
		open?: boolean;
	} = $props();

	const objectiveStatusColors: Record<
		string,
		"off" | "red" | "yellow" | "green"
	> = {
		ready: "yellow",
		active: "yellow",
		complete: "green",
		failed: "red",
	};

	const objectiveCount = $derived(objectives.length);
	const objectiveLabel = $derived(
		`${objectiveCount} objective${objectiveCount === 1 ? "" : "s"}`
	);
	const constraintSummary = $derived(
		`writes ${constraints.writes ?? "–"} · reads ${constraints.reads ?? "–"}`
	);

	function toggleObjectives() {
		open = !open;
	}

	function closeObjectives() {
		open = false;
	}

	function ledColor(status: string) {
		return objectiveStatusColors[status] ?? "off";
	}
</script>

<div
	class="flex flex-row items-baseline justify-between gap-[1vw] px-2 py-1 text-sm"
>
	<button
		class="cursor-pointer font-semibold"
		onclick={toggleObjectives}
		aria-expanded={open}
	>
		{objectiveLabel}
		{open ? "▾" : "▸"}
	</button>
	<span>{constraintSummary}</span>
</div>

{#if open}
	<!-- click-away backdrop -->
	<button
		class="fixed inset-0 z-10 cursor-default"
		aria-label="Close objectives"
		onclick={closeObjectives}
	></button>

	<!-- objectives menu (drops below the card, never resizes it) -->
	<Box class="absolute top-full left-2 z-20 mt-1 p-2">
		<div class="flex w-max max-w-[70vw] flex-col gap-1">
			{#each objectives as objective (objective.id)}
				<div class="flex items-center gap-2 text-sm">
					<Led color={ledColor(objective.status)} />
					<span>{objective.name}</span>
				</div>
			{/each}
		</div>
	</Box>
{/if}
