<script lang="ts">
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

<Screen
	class="flex h-full items-center justify-center p-6"
	bezelClass="h-full"
>
	<div class="flex w-full flex-col items-center justify-center">
		<img
			src={satelliteImage.src}
			alt={satelliteImage.alt}
			class="h-auto w-1/16 object-contain"
		/>
		<span class="font-semibold">Telemetry</span>
	</div>
</Screen>
