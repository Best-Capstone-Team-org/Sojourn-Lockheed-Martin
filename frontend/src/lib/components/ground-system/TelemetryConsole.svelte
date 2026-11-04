<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import Screen from "$lib/components/Screen/Screen.svelte";
	import satelliteDownlink from "$lib/assets/satelite_down.webp";
	import satelliteUplink from "$lib/assets/satelite_up.webp";
	import satelliteNoop from "$lib/assets/satelite_noop.webp";
	import satelliteFailure from "$lib/assets/satelite_failture.png";
	import { type CommandDownlink } from "$lib/api";

	let {
		commandDownlink,
		uplinkInFlight,
		satelliteDown,
	}: {
		commandDownlink: CommandDownlink | undefined;
		uplinkInFlight: boolean;
		satelliteDown: boolean;
	} = $props();

	const satelliteImage = $derived.by(() => {
		if (satelliteDown) {
			return { src: satelliteFailure, alt: "Satellite down" };
		}

		if (uplinkInFlight) {
			return {
				src: satelliteUplink,
				alt: "Satellite receiving uplink",
			};
		}

		if (commandDownlink?.type === "commandResponse") {
			return {
				src: satelliteDownlink,
				alt: "Satellite transmitting downlink",
			};
		}

		return { src: satelliteNoop, alt: "Satellite idle" };
	});
</script>

<Box class="h-full w-full p-2">
	<Screen
		class="flex h-full items-center justify-center p-2"
		bezelClass="h-full"
	>
		<div class="flex w-full flex-col items-center justify-center">
			<img
				src={satelliteImage.src}
				alt={satelliteImage.alt}
				class="h-auto w-16 object-contain"
			/>
		</div>
	</Screen>
</Box>
