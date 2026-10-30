<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import Box from "$lib/components/Box/Box.svelte";
	import Button from "$lib/components/Button/Button.svelte";
	import Screen from "$lib/components/Screen/Screen.svelte";
	import Objectives from "$lib/components/level-select/Objectives.svelte";
	import type { ScenarioResponse } from "$lib/api";
	import { gameState } from "$lib/components/ground-system/GameState.svelte";
	import easyProbe from "$lib/assets/easy-probe.png";
	import mediumProbe from "$lib/assets/med-probe.png";
	import hardProbe from "$lib/assets/hard-probe.png";

	let { scenario }: { scenario: ScenarioResponse } = $props();
	let objectivesOpen = $state(false);

	function startScenario() {
		gameState.scenario = scenario;
		goto(resolve("/ground-system"));
	}

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
				bezelClass="aspect-square w-[11vh] shrink-0"
			>
				<img
					src={probeForDifficulty(scenario.difficulty)}
					alt={`${scenario.difficulty} probe`}
					class="h-full w-full object-contain"
				/>
			</Screen>

			<div class="flex min-h-0 min-w-0 flex-1 flex-col">
				<p class="text-md font-bold">{scenario.name}</p>
				<p class="line-clamp-3 h-15 text-sm">
					{scenario.description}
				</p>
			</div>
		</div>

		<Button
			wellClass="shrink-0 self-stretch"
			buttonClass="p-2"
			onclick={startScenario}
		>
			Play
		</Button>
	</div>

	<hr
		class="mx-2 border-t border-black/35 shadow-[0_1px_0_rgba(255,255,255,0.4)]"
	/>
	<Objectives
		objectives={scenario.objectives}
		constraints={scenario.constraints}
		bind:open={objectivesOpen}
	/>
</Box>
