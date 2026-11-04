<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import Screen from "$lib/components/Screen/Screen.svelte";

	let countries = $state(["USA", "Romania", "Philippines"]);
	let countryIndex = $state(0);
	const country = $derived(countries[countryIndex]);
</script>

<Box class="h-full w-full p-2">
	<Screen
		bezelClass="w-full"
		class="flex items-center justify-center p-2 text-center"
	>
		{country}
	</Screen>
</Box>
