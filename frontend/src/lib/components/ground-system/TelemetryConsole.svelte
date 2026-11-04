<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import Screen from "$lib/components/Screen/Screen.svelte";
	import satelliteDownlink from "$lib/assets/satelite_down.webp";
	import satelliteUplink from "$lib/assets/satelite_up.webp";
	import satelliteNoop from "$lib/assets/satelite_noop.webp";
	import { type CommandDownlink, type CommandUplink } from "$lib/api";

	let {
		commandDownlink,
		commandUplink,
	}: {
		commandDownlink: CommandDownlink | undefined;
		commandUplink: CommandUplink | undefined;
	} = $props();
</script>

<Box class="h-full w-full p-2">
	<Screen
		class="flex h-full items-center justify-center p-2"
		bezelClass="h-full"
	>
		<div class="flex w-full flex-col items-center justify-center">
			<img
				src={commandDownlink?.type === "commandResponse"
					? satelliteDownlink
					: commandUplink !== undefined
						? satelliteUplink
						: satelliteNoop}
				alt={commandDownlink?.type === "commandResponse"
					? "Satellite transmitting downlink"
					: commandUplink !== undefined
						? "Satellite receiving uplink"
						: "Satellite idle"}
				class="h-auto w-16 object-contain"
			/>
		</div>
	</Screen>
</Box>
