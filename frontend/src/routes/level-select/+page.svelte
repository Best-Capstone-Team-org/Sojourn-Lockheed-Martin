<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import levelSelectScreen from "$lib/assets/level-select-screen.png";
	import Button from "$lib/components/Button/Button.svelte";

	const levels = [1, 2, 3, 4, 5, 6, 7, 8];
</script>

<main class="relative h-screen w-screen">
	<img
		src={levelSelectScreen}
		alt="Level Select"
		class="h-full w-full object-cover"
	/>
	<div
		class="absolute top-[25%] left-1/2 flex h-[50%] w-[70vw] -translate-x-1/2 flex-col gap-[1vw] overflow-y-auto"
	>
		{#each levels as level (level)}
			<Button
				onclick={() => goto(resolve("/ground-system"))}
				class="w-full p-[1vw] text-center text-[1.5vw] leading-[1.5vw] text-white"
			>
				Level {level}
			</Button>
		{/each}
	</div>
</main>
