<script lang="ts">
	import Screen from "$lib/components/Screen/Screen.svelte";
	import satelliteDownlink from "$lib/assets/satelite_down.webp";
	import satelliteUplink from "$lib/assets/satelite_up.webp";
	import satelliteNoop from "$lib/assets/satelite_noop.webp";
	import { type CommandDownlink } from "$lib/api";

	let { commandDownlink }: { commandDownlink: CommandDownlink | undefined } =
		$props();
</script>

<Screen
	class="flex h-full items-center justify-center p-6"
	bezelClass="h-full"
>
	<div class="flex h-full w-full flex-col items-center justify-center">
		<img
			src={commandDownlink?.type === "commandResponse" ? satelliteDownlink : satelliteNoop}
			alt={
				commandDownlink?.type === "commandResponse"
					? "Satellite transmitting downlink"
					: "Satellite idle"
			}
		/>
		<span class="font-semibold">Telemetry</span>
	</div>
</Screen>
