<script lang="ts">
	import Screen from "$lib/components/Screen/Screen.svelte";
	import satelliteDownlink from "$lib/assets/satelite_down.webp";
	import satelliteUplink from "$lib/assets/satelite_up.webp";
	import satelliteNoop from "$lib/assets/satelite_noop.webp";
	import { type CommandDownlink, type CommandUplink } from "$lib/api";

	let { commandDownlink, commandUplink }: { commandDownlink: CommandDownlink | undefined; commandUplink: CommandUplink | undefined } =
		$props();
</script>

<Screen
	class="flex h-full items-center justify-center p-6"
	bezelClass="h-full"
>
	<div class="flex w-full flex-col items-center justify-center">
		<img
			src={commandDownlink?.type === "commandResponse" ? satelliteDownlink : commandUplink !== undefined ? satelliteUplink : satelliteNoop}
			alt={
				commandDownlink?.type === "commandResponse"
					? "Satellite transmitting downlink"
					: commandUplink !== undefined
					? "Satellite receiving uplink"
					: "Satellite idle"
			}
			class="h-auto w-1/16 object-contain"
		/>
		<span class="font-semibold">Telemetry</span>
	</div>
</Screen>
