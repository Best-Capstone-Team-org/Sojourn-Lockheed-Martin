<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import Screen from "$lib/components/Screen/Screen.svelte";
	import Led from "./Led.svelte";
	import satelliteDownlink from "$lib/assets/satelite_down.webp";
	import satelliteUplink from "$lib/assets/satelite_up.webp";
	import satelliteNoop from "$lib/assets/satelite_noop.webp";
	import satelliteFailure from "$lib/assets/satelite_failture.png";
	import { type CommandDownlink } from "$lib/api";

	let {
		commandDownlink,
		uplinkInFlight,
		downlinkInFlight,
		satelliteDown,
	}: {
		commandDownlink: CommandDownlink | undefined;
		uplinkInFlight: boolean;
		downlinkInFlight: boolean;
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

		if (downlinkInFlight) {
			return {
				src: satelliteDownlink,
				alt: "Satellite transmitting downlink",
			};
		}

		return { src: satelliteNoop, alt: "Satellite idle" };
	});

	const HGA = 0;
	const LGA = 1;

	let hgaLed: Led;
	let lgaLed: Led;

	$effect(() => {
		if (commandDownlink?.type !== "telemetry") return;
		const antenna = commandDownlink.telemetry.comms?.antenna;
		if (antenna === HGA) hgaLed?.blink("green");
		else if (antenna === LGA) lgaLed?.blink("green");
	});
</script>

<Box class="flex h-full w-full items-center p-2">
	<div
		class="grid w-2/5 grid-cols-[auto_auto] content-center items-center justify-start gap-2 px-1 font-bold"
	>
		<span>HGA</span>
		<Led bind:this={hgaLed} />
		<span>LGA</span>
		<Led bind:this={lgaLed} />
	</div>
	<Screen
		class="flex h-full items-center justify-center p-2"
		bezelClass="h-full w-3/5"
	>
		<div class="flex w-full items-center justify-center">
			<img
				src={satelliteImage.src}
				alt={satelliteImage.alt}
				class="h-auto w-16 object-contain"
			/>
		</div>
	</Screen>
</Box>
