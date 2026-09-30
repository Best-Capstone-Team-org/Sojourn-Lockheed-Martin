<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import Button from "$lib/components/Button/Button.svelte";
</script>

<main class="flex h-screen w-screen items-center justify-center bg-[#293134]">
	<Button
		onclick={() => goto(resolve("/ground-system"))}
		class="p-[1vw] text-center text-[1.5vw] leading-[1.5vw] text-white"
	>
		ground system
	</Button>
</main>
