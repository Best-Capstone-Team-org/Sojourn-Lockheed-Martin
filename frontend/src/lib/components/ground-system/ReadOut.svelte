<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import ReadOutSection from "$lib/components/ground-system/ReadOutSection.svelte";
	import Toggle from "$lib/components/Toggle/Toggle.svelte";
	import type { Telemetry } from "$lib/api";
	import { options } from "./Options.svelte";
	import Led from "./Led.svelte";
	import Screen from "../Screen/Screen.svelte";
	import absent from "$lib/assets/absent-icon.png"

	let { readOutTlm }: { readOutTlm: Telemetry | undefined } = $props();

	const frame = $derived(readOutTlm);
	const sensors = $derived(frame?.sensors);
	const hk = $derived(frame?.hk);

	function toggleTelemetry(state: boolean) {
		options.telemetry = state;
	}

	let led: Led;

	$effect(() => {
		if (readOutTlm) {
			led.blink();
		}
	});

	// read out data
	const frameHeader = $derived([
		{ label: "Mode", value: frame?.mode },
		{ label: "Uptime", value: frame?.uptime, unit: "s" },
		{ label: "Bus", value: frame?.bus, unit: "V" },
		{ label: "Load", value: frame?.load, unit: "mW" },
		{ label: "Reboot", value: frame?.reboots },
		{ label: "Last Fault", value: frame?.lastFault },
	]);

	const instruments = $derived([
		{ label: "Magnetometer", value: sensors?.[0], unit: "nT" },
		{ label: "Inertial", value: sensors?.[1], unit: "°/s" },
		{ label: "Thermal", value: sensors?.[2], unit: "°C" },
		{ label: "Bus Monitor", value: sensors?.[3], unit: "mV" },
		{ label: "Radiation", value: sensors?.[4], unit: "ct" },
		{ label: "Star Tracker", value: sensors?.[5], unit: "q" },
	]);

	const houseKeeping = $derived([
		{
			label: "Heater",
			value:
				hk?.heaterOn == null ? undefined : hk.heaterOn ? "On" : "Off",
		},
		{ label: "Propellant", value: hk?.prop, unit: "mg" },
		{ label: "Momentum", value: hk?.mom },
		{ label: "Recorder", value: hk?.rec, unit: "%" },
		{ label: "Shed Events", value: hk?.shed },
	]);

	const readOutSections = $derived([
		{ title: "Frame Header", items: frameHeader },
		{ title: "Instruments", items: instruments },
		{ title: "House Keeping", items: houseKeeping },
	]);

	const isMissing = (value: string | number | null | undefined) =>
		value === undefined || value === null || value === "";
</script>

<div class="flex h-full min-h-0 flex-col overflow-hidden">
	<Box class="flex w-full shrink-0 items-center justify-between p-2">
		<span class="text-xl font-bold">Read Out</span>
		<Led bind:this={led} />
	</Box>
	<div class="min-h-0 flex-1 overflow-y-auto">
		{#each readOutSections as section (section.title)}
			<ReadOutSection title={section.title}>
				<Screen class="w-full p-2">
					{#each section.items as item (item.label)}
						<div class="flex justify-between">
							<span>{item.label}</span>
							{#if isMissing(item.value)}
								<img src={absent} alt="absent" class="w-4 h-4" />
							{:else}
								<span
									>{item.value}{item.unit
										? ` ${item.unit}`
										: ""}</span
								>
							{/if}
						</div>
					{/each}
				</Screen>
			</ReadOutSection>
		{/each}
		<ReadOutSection title="Options">
			<div class="flex justify-between">
				<span>Telemetry</span>
				<Toggle
					checked={options.telemetry}
					onchange={toggleTelemetry}
				/>
			</div>
		</ReadOutSection>
	</div>
</div>
