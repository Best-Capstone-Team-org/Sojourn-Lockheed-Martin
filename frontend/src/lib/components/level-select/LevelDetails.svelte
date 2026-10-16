<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import Box from "$lib/components/Box/Box.svelte";
	import Button from "$lib/components/Button/Button.svelte";
	import Screen from "$lib/components/Screen/Screen.svelte";

	let {
		name,
		description,
		difficulty,
	}: {
		name: string;
		description: string;
		difficulty: number;
	} = $props();
</script>

<Box class="w-full">
	<div class="flex min-h-[12vh] min-w-0 flex-row items-stretch gap-[1vw] p-2">
		<div class="flex min-h-0 min-w-0 flex-1 flex-row items-stretch gap-[1vw]">
			<Screen
				class="flex flex-col items-center justify-center gap-[1vw]"
				bezelClass="aspect-square h-full min-h-0 shrink-0 self-stretch"
			>
				{#if difficulty === 1}
					<p>Easy</p>
				{:else if difficulty === 2}
					<p>Medium</p>
				{:else if difficulty === 3}
					<p>Hard</p>
				{/if}
			</Screen>

			<div class="flex min-h-0 min-w-0 flex-1 flex-col">
				<p class="text-md font-bold">{name}</p>
				<p class="min-h-0 flex-1 text-sm">{description}</p>
			</div>
		</div>

		<Button
			wellClass="shrink-0 self-stretch"
			buttonClass="p-2"
			onclick={() => goto(resolve("/ground-system"))}
		>
			Play
		</Button>
	</div>
</Box>
