<script lang="ts">
	import Screen from "$lib/components/Screen/Screen.svelte";
	import Button from "$lib/components/Button/Button.svelte";
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import type { ScenarioResponse } from "$lib/api";
	import { gameState } from "$lib/components/ground-system/GameState.svelte";

	let { selectedLevel }: { selectedLevel: ScenarioResponse | undefined } = $props();

	function startSelected() {
		if (!selectedLevel) return;
		gameState.scenario = selectedLevel;
		goto(resolve("/ground-system"));
	}

	let records = $derived<{
		id: string,
		name: string,
		elapsedMs: number,
		uplinks: number
	}[]>(
		selectedLevel?.record
		? selectedLevel.record.objectives.map((record) => {
			const objective = selectedLevel.objectives.find((objective) => objective.id === record.id);
			return {
				id: record.id,
				name: objective ? objective.name : "",
				elapsedMs: record.elapsed_ms,
				uplinks: record.uplinks
			};
		})
		: []
	);
</script>

<Screen
	bezelClass="w-full h-full"
>
	<div class="h-full overflow-y-auto p-2">
		<p class="text-3xl font-bold">{selectedLevel?.name ?? "—"}</p>
		<hr class="white mb-2 w-full" />
		<p class="text-sm">{selectedLevel?.description ?? "—"}</p>
		<hr class="white mt-2 mb-2 w-full" />
		<p class="text-lg font-bold">Objectives:</p>
		<ul class="list-inside list-disc">
			{#each selectedLevel?.objectives as objective (objective.id)}
				<li>{objective.name} - {objective.description}</li>
			{/each}
		</ul>
		<hr class="white mt-2 mb-2 w-full" />
		<p class="text-sm">
			Writes - {selectedLevel?.constraints.writes ?? "—"}
		</p>
		<p class="text-sm">
			Reads - {selectedLevel?.constraints.reads ?? "—"}
		</p>
		{#if records.length > 0}
			<hr class="white mt-2 mb-2 w-full" />
			<div class="flex flex-col gap-2">
				<p class="text-lg font-bold">Completion Record:</p>
				{#each records as record (record.id)}
					<div>
						<strong>{record.name}</strong>
						<p>Time: {(record.elapsedMs / 1000).toFixed(1)}s</p>
						<p>Uplinks: {record.uplinks}</p>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</Screen>
<div class="flex h-1/8 w-full flex-row justify-end gap-2">
	<Button
		wellClass="w-1/4"
		alert
		onclick={() => goto(resolve("/"))}
	>
		Quit
	</Button>
	<Button
		wellClass="w-1/4"
		onclick={startSelected}
	>
		Play
	</Button>
</div>
