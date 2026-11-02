<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import Button from "$lib/components/Button/Button.svelte";
	import Screen from "$lib/components/Screen/Screen.svelte";

	let countries = $state(["USA", "Romania", "Philippines"]);
	let countryIndex = $state(0);
	const country = $derived(countries[countryIndex]);

	const inc = () => {
		countryIndex = (countryIndex + 1) % countries.length;
	};

	const dec = () => {
		countryIndex = (countryIndex - 1 + countries.length) % countries.length;
	};
</script>

<Box class="h-full w-full p-2">
	<div class="flex flex-row gap-2">
		<div class="flex w-1/2 flex-col gap-2">
			<Button
				buttonClass="p-1 text-xs"
				onclick={inc}
			>
				▲
			</Button>
			<Button
				buttonClass="p-1 text-xs"
				onclick={dec}
			>
				▼
			</Button>
		</div>
		<Screen
			bezelClass="w-1/2"
			class="flex items-center justify-center p-2 text-center"
		>
			{country}
		</Screen>
	</div>
</Box>
