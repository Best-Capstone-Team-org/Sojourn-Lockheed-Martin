<script lang="ts">
	import Button from '$lib/components/Button/Button.svelte';
	import Screen from '$lib/components/Screen/Screen.svelte';

	function start() {
		console.log('Start');
	}

	function openOptions() {
		console.log('Options');
	}
</script>

<div class="flex justify-center items-center h-screen flex-col gap-0">
	<Screen class="w-50 h-50 flex items-center justify-center">
		<h1 class="text-center text-2xl">Sojourn</h1>
	</Screen>

	<Button class="w-96 mt-4" onclick={start}>
		<h1 class="text-center text-2xl">Start</h1>
	</Button>

	<Button class="w-96 mt-4" onclick={openOptions}>
		<h1 class="text-center text-2xl">Options</h1>
	</Button>
</div>


