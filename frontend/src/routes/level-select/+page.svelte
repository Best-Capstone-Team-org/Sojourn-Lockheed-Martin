<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import LevelDetails from "$lib/components/level-select/LevelDetails.svelte";
	import { onMount } from "svelte";
	import { apiFetch, type ScenarioResponse } from "$lib/api";

	let levels: ScenarioResponse[] = $state([]);
	async function getLevels() {
		try {
			const response = await apiFetch("scenarios");
			if (!response.ok) {
				throw new Error(`Failed to load scenarios: ${response.status}`);
			}

			const scenarios: ScenarioResponse[] = await response.json();
			levels = scenarios.sort((a, b) => a.name.localeCompare(b.name));
		} catch (error) {
			console.error("Failed to load scenarios", error);
		}
	}

	onMount(() => {
		getLevels();
	});
</script>

<main class="box-border min-w-0 px-[10%]">
	<div class="flex w-full min-w-0 flex-col">
		<Box class="w-full p-2">
			<h1 class="text-4xl font-bold">Level Select</h1>
		</Box>
		{#each levels as level (level.id)}
			<LevelDetails scenario={level} />
		{/each}
	</div>
</main>
