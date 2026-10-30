<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import Screen from "$lib/components/Screen/Screen.svelte";
	import type { ScenarioResponse } from "$lib/api";
	import easyProbe from "$lib/assets/easy-probe.png";
	import mediumProbe from "$lib/assets/med-probe.png";
	import hardProbe from "$lib/assets/hard-probe.png";

	let { scenario }: { scenario: ScenarioResponse } = $props();
	let objectivesOpen = $state(false);

	function probeForDifficulty(difficulty: string) {
		if (difficulty === "intermediate") {
			return hardProbe; // just wanted to show the hard probe
		} else if (difficulty === "tutorial") {
			return easyProbe;
		} else {
			return mediumProbe;
		}
	}
</script>

<Box class="relative w-full {objectivesOpen ? 'z-20' : 'z-0'}">
	<div class="flex min-h-[13vh] min-w-0 flex-row items-stretch gap-[1vw] p-2">
		<div
			class="flex min-h-0 min-w-0 flex-1 flex-row items-stretch gap-[1vw]"
		>
			<Screen
				class="flex flex-col items-center justify-center"
				bezelClass="aspect-square w-[14vh] shrink-0"
			>
				<img
					src={probeForDifficulty(scenario.difficulty)}
					alt={`${scenario.difficulty} probe`}
					class="h-full w-full object-contain"
				/>
			</Screen>

			<div class="flex min-h-0 min-w-0 flex-1 flex-col">
				<p class="text-xl font-bold">{scenario.name}</p>
				<p class="line-clamp-3 h-15 text-sm">
					{scenario.description}
				</p>
			</div>
		</div>
	</div>
</Box>
