<script lang="ts">
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import titleScreen from "$lib/assets/title_screen.png";
	import Button from "$lib/components/Button/Button.svelte";
</script>

<main class="relative h-screen w-screen">
	<img
		src={titleScreen}
		alt="Sojourn"
		class="h-full w-full object-cover"
	/>
	<Button
		onclick={() => goto(resolve("/level-select"))}
		class="absolute top-[50%] left-[20%] w-[20vw] p-[1vw] text-center text-[1.5vw] leading-[1.5vw] text-white"
	>
		Start
	</Button>
</main>
