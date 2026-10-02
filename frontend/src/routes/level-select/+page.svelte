<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import Button from "$lib/components/Button/Button.svelte";
</script>

<main class="flex h-screen w-screen items-center justify-center">
	<Button
		onclick={() => goto(resolve("/ground-system"))}
		wellClass="text-center text-[1.5vw] leading-[1.5vw]"
		buttonClass="px-6 py-3"
	>
		ground system
	</Button>
</main>
