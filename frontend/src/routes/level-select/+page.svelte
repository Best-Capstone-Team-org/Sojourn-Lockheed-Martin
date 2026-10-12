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
	aria-label="Sojourn"
>
	<div
		class="absolute top-[25%] left-[15vw] h-[50%] w-[70vw] overflow-y-auto"
	>
		<div class="flex flex-col gap-[1vw] pr-[1vw]">
			{#each levels as level (level)}
				<button
					type="button"
					onclick={() => goto(resolve("/ground-system"))}
					class="w-full cursor-pointer border-2 border-white bg-transparent p-[1vw] text-center text-[1.5vw] leading-[1.5vw] text-white transition-colors duration-200 hover:bg-white hover:text-black focus-visible:bg-white focus-visible:text-black focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white"
				>
					Level {level}
				</button>
			{/each}
		</div>
	</div>
</main>
