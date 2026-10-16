<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import LevelDetails from "$lib/components/level-select/LevelDetails.svelte";

	const levels = [1, 2, 3, 4, 5, 6, 7, 8];
</script>

<main class="box-border min-w-0 px-[10%]">
	<div class="flex w-full min-w-0 flex-col">
		<Box class="w-full p-2">
			<h1 class="text-4xl font-bold">Level Select</h1>
		</Box>
		{#each levels as level (level)}
			<LevelDetails
				name={`Level ${level}`}
				description={`This is the ${level} level. ... ... . .. . . . . . .. . . . .. . .. . . .. . . . .. . . .. . . .. . . . .. . . .. . . . . .. . .`}
				difficulty={(level % 3) + 1}
			/>
		{/each}
	</div>
</main>
