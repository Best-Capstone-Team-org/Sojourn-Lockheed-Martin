<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import Box from "$lib/components/Box/Box.svelte";
	import Button from "$lib/components/Button/Button.svelte";
	import Screen from "$lib/components/Screen/Screen.svelte";
	import { type ScenarioResponse } from "$lib/api";
	let { scenario }: { scenario: ScenarioResponse } = $props();
	import { gameState } from "$lib/components/ground-system/GameState.svelte";
</script>

<Box class="w-full">
	<div class="flex min-h-[12vh] min-w-0 flex-row items-stretch gap-[1vw] p-2">
		<div
			class="flex min-h-0 min-w-0 flex-1 flex-row items-stretch gap-[1vw]"
		>
			<Screen
				class="flex flex-col items-center justify-center gap-[1vw]"
				bezelClass="aspect-square h-full min-h-0 shrink-0 self-stretch"
			>
				<p>{scenario.difficulty}</p>
			</Screen>

			<div class="flex min-h-0 min-w-0 flex-1 flex-col">
				<p class="text-md font-bold">{scenario.name}</p>
				<p class="min-h-0 flex-1 text-sm">{scenario.description}</p>
			</div>
		</div>

		<Button
			wellClass="shrink-0 self-stretch"
			buttonClass="p-2"
			onclick={() => {
				goto(resolve("/ground-system"));
				gameState.id = scenario.id;
			}}
		>
			Play
		</Button>
	</div>
</Box>
