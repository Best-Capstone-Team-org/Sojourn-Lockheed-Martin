<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import LevelDetails from "$lib/components/level-select/LevelDetails.svelte";
	import SelectionBrackets from "$lib/components/level-select/SelectionBrackets.svelte";
	import { onMount } from "svelte";
	import { apiFetch, type ScenarioResponse } from "$lib/api";
	import SelectedLevelPanel from "$lib/components/level-select/SelectedLevelPanel.svelte";

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

	// control custom scrolling
	let scroller: HTMLDivElement | undefined = $state();
	let viewportH = $state(0);
	let cardH = $state(0);
	let selectedIndex = $state(0);
	const selectedLevel = $derived(levels[selectedIndex]);

	function updateSelection() {
		if (!scroller || cardH === 0) return;
		const i = Math.round(scroller.scrollTop / cardH);
		selectedIndex = Math.max(0, Math.min(levels.length - 1, i));
	}

	function centerLevel(i: number) {
		scroller?.scrollTo({ top: i * cardH, behavior: "smooth" });
	}
</script>

<main class="h-screen overflow-hidden">
	<div class="flex h-full w-full min-w-0 flex-row">
		<div class="flex h-full min-h-0 w-3/5 min-w-0 flex-col">
			<Box class="w-full p-2">
				<h1 class="text-4xl font-bold">Level Select</h1>
			</Box>

			<div class="relative min-h-0 flex-1">
				<div
					bind:this={scroller}
					bind:clientHeight={viewportH}
					onscroll={updateSelection}
					class="h-full snap-y snap-mandatory scrollbar-none overflow-y-auto [&::-webkit-scrollbar]:hidden"
					style:padding-block="{Math.max(
						0,
						(viewportH - cardH) / 2
					)}px"
				>
					{#each levels as level, i (level.id)}
						<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
						<div
							class="snap-center"
							onclick={() => centerLevel(i)}
						>
							{#if i === 0}
								<div bind:clientHeight={cardH}>
									<LevelDetails scenario={level} />
								</div>
							{:else}
								<LevelDetails scenario={level} />
							{/if}
						</div>
					{/each}
				</div>

				<SelectionBrackets
					class="absolute -inset-x-1.5 top-1/2 z-30 -translate-y-1/2"
					style="height: calc({cardH}px + 1rem)"
				/>
			</div>
		</div>
		<Box
			class="flex h-screen w-2/5 min-w-0 flex-col justify-between gap-2 p-2"
		>
			<SelectedLevelPanel {selectedLevel} />
		</Box>
	</div>
</main>
