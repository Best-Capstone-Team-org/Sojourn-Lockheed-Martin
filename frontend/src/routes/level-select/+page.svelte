<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import levelSelectScreen from "$lib/assets/level-select-screen.png";

	const levels = [1, 2, 3, 4, 5, 6, 7, 8];
</script>

<main
	class="relative h-screen w-screen bg-cover bg-center bg-no-repeat"
	style:background-image="url({levelSelectScreen})"
	role="img"
	aria-label="Level Select"
>
	<p class="absolute top-[15%] text-white w-screen text-center text-[2vw]">Level Select</p>
	<div
		class="absolute top-[25%] left-[15vw] h-[50%] w-[70vw] overflow-y-auto"
	>
		<div class="flex flex-col pr-[1vw]">
			{#each levels as level (level)}
				<button
					class="w-full border border-transparent p-[1vw] text-center text-[1.5vw] leading-[1.5vw] text-white hover:border-white"
					onclick={() => goto(resolve("/ground-system"))}
				>
					Level {level}
				</button>
			{/each}
		</div>
	</div>
</main>
