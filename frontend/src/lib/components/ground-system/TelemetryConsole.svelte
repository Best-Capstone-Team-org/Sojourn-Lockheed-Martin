<script lang="ts">
	import Screen from "$lib/components/Screen/Screen.svelte";
	import satelliteDownlink from "$lib/assets/satelite_down.webp";
	import satelliteNoop from "$lib/assets/satelite_noop.webp";
	import satelliteUplink from "$lib/assets/satelite_up.webp";

	let {
		satelliteState,
	}: {
		satelliteState: "up" | "down" | "noop";
	} = $props();
</script>

<Screen
	class="flex h-full items-center justify-center p-6"
	bezelClass="h-full"
>
	<div class="flex h-full w-full flex-col items-center justify-center">
		<img
			src={
				satelliteState === "up"
					? satelliteUplink
					: satelliteState === "down"
						? satelliteDownlink
						: satelliteNoop
			}
			alt={
				satelliteState === "up"
					? "Satellite transmitting uplink"
					: satelliteState === "down"
						? "Satellite transmitting downlink"
						: "Satellite idle"
			}
			class="h-3/4 w-auto max-w-full object-contain"
		/>
		<span class="font-semibold">Telemetry</span>
	</div>
</Screen>
