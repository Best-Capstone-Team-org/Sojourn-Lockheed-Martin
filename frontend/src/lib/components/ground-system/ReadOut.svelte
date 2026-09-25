<script lang="ts">
	import Box from "$lib/components/Box/Box.svelte";
	import ReadOutSection from "$lib/components/ground-system/ReadOutSection.svelte";
	import type { Telemetry } from "$lib/api";

	let { telemetry }: { telemetry: Telemetry | undefined } = $props();

	const frame = $derived(telemetry);
	const sensors = $derived(frame?.sensors);
	const hk = $derived(frame?.hk);
</script>

<div class="flex h-full flex-col">
	<Box class="w-full p-2">
		<span class="font-semibold">Read Out</span>
		<!-- {JSON.stringify(telemetry)} -->
	</Box>
	<ReadOutSection
		title="Frame Header"
		items={[
			{ label: "Mode", value: frame?.mode },
			{ label: "Uptime", value: frame?.uptime, unit: "s" },
			{ label: "Bus", value: frame?.bus, unit: "V" },
			{ label: "Load", value: frame?.load, unit: "mW" },
			{ label: "Reboot", value: frame?.reboots },
			{ label: "Last Fault", value: frame?.lastFault },
		]}
	/>
	<ReadOutSection
		title="Instruments"
		items={[
			{ label: "Magnetometer", value: sensors?.[0], unit: "nT" },
			{ label: "Inertial", value: sensors?.[1], unit: "°/s" },
			{ label: "Thermal", value: sensors?.[2], unit: "°C" },
			{ label: "Bus Monitor", value: sensors?.[3], unit: "mV" },
			{ label: "Radiation", value: sensors?.[4], unit: "ct" },
			{ label: "Star Tracker", value: sensors?.[5], unit: "q" },
		]}
	/>
	<ReadOutSection
		title="House Keeping"
		items={[
			{
				label: "Heater",
				value:
					hk?.heaterOn == null
						? undefined
						: hk.heaterOn
							? "On"
							: "Off",
			},
			{ label: "Propellant", value: hk?.prop, unit: "mg" },
			{ label: "Momentum", value: hk?.mom },
			{ label: "Recorder", value: hk?.rec, unit: "%" },
			{ label: "Shed Events", value: hk?.shed },
		]}
	/>
</div>
