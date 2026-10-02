<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import titleScreen from "$lib/assets/title_screen.png";
	import Button from "$lib/components/Button/Button.svelte";
</script>

<main
	class="relative h-screen w-screen bg-cover bg-center bg-no-repeat"
	style:background-image="url({titleScreen})"
	role="img"
	aria-label="Sojourn"
>
	<Button
		onclick={() => goto(resolve("/level-select"))}
		wellClass="absolute top-[50%] left-[20%] w-[20vw] text-center text-[1.5vw] leading-[1.5vw]"
		buttonClass="px-6 py-3"
	>
		Start
	</Button>
</main>
