<script lang="ts">
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { resolve } from "$app/paths";
	import endScreen from "$lib/assets/end_screen.webp";
	import Box from "$lib/components/Box/Box.svelte";
	import Button from "$lib/components/Button/Button.svelte";
	import Screen from "$lib/components/Screen/Screen.svelte";
	import TextWriter from "$lib/components/TextWriter/TextWriter.svelte";
	import Led from "$lib/components/ground-system/Led.svelte";
	import { gameState } from "$lib/components/ground-system/GameState.svelte";

	// README.md.
	const TEAM = [
		"Macsen Casaus",
		"Susan Hamilton",
		"Isaac Lagoy",
		"Owen Shadburne",
		"David Zhang",
	];

	const LED_MS = 300;

	const scenario = gameState.scenario;
	const objectives = scenario?.objectives ?? [];

	const label = "mt-[1.3em] text-[0.85em] text-white/60";
	const lines: { text: string; class?: string }[] = [
		{
			text: "> MISSION COMPLETE",
			class: "text-[2.5em] leading-tight font-bold",
		},
		...(scenario
			? [
					{
						text: `> ${scenario.name.toUpperCase()}`,
						class: "text-[1.5em]",
					},
				]
			: []),
		{ text: "> THANK YOU FOR PLAYING", class: "mt-[1.3em]" },
		...(scenario
			? [
					{ text: "> LEVEL AUTHOR", class: label },
					{ text: `>   ${scenario.author.toUpperCase()}` },
				]
			: []),
		{ text: "> TEAM", class: label },
		...TEAM.map((name) => ({ text: `>   ${name.toUpperCase()}` })),
		{ text: "> SPONSOR", class: label },
		{ text: ">   LOCKHEED MARTIN" },
		{ text: "> END OF TRANSMISSION", class: label },
	];

	// Completed Objectives
	let lit = $state(0);
	let led: Led;

	onMount(() => {
		if (matchMedia("(prefers-reduced-motion: reduce)").matches) {
			lit = objectives.length;
			return;
		}

		const leds = setInterval(() => {
			lit++;
			if (lit >= objectives.length) clearInterval(leds);
		}, LED_MS);

		return () => {
			clearInterval(leds);
		};
	});
</script>

<svelte:head>
	<title>End Screen</title>
</svelte:head>

<main class="h-screen overflow-hidden">
	<Box class="flex h-full flex-col gap-2 p-2">
		<div class="flex items-center justify-between px-1">
			<span class="text-xl font-bold">Downlink</span>
			<Led
				bind:this={led}
				fade={600}
			/>
		</div>

		<Screen
			bezelClass="min-h-0 flex-1"
			class="grid grid-cols-[1.15fr_1fr] items-center gap-[3vw] p-[3vw]"
		>
			<div class="flex flex-col items-center gap-6">
				<div class="relative aspect-[167/82] w-full">
					<div class="absolute inset-0 overflow-hidden">
						<img
							src={endScreen}
							alt="The probe, spinning"
							class="absolute top-[-63.4%] left-[-44.3%] w-[191.6%] max-w-none mix-blend-lighten [image-rendering:pixelated]"
						/>
					</div>
					{#each ["-top-3 -left-3 border-t-2 border-l-2", "-top-3 -right-3 border-t-2 border-r-2", "-bottom-3 -left-3 border-b-2 border-l-2", "-right-3 -bottom-3 border-r-2 border-b-2"] as corner (corner)}
						<span class="absolute size-8 border-white/60 {corner}"
						></span>
					{/each}
				</div>
				<p
					class="glow text-[length:min(1vw,1.8vh)] tracking-[0.5em] text-white/70"
				>
					SOJOURN
				</p>
			</div>

			<TextWriter
				{lines}
				onLineComplete={() => led.blink()}
			/>
		</Screen>

		<div class="flex min-h-[8vh] items-center justify-between gap-4 px-2">
			<div class="flex flex-wrap gap-x-6 gap-y-1">
				{#each objectives as objective, i (objective.id)}
					<span class="flex items-center gap-2 font-semibold">
						<Led color={i < lit ? "green" : "off"} />
						{objective.name}
					</span>
				{/each}
			</div>
			<div class="flex h-[8vh] w-[30vw] shrink-0 gap-2">
				<Button
					wellClass="w-1/2"
					alert
					onclick={() => goto(resolve("/"))}
				>
					Main Menu
				</Button>
				<Button
					wellClass="w-1/2"
					onclick={() => goto(resolve("/level-select"))}
				>
					Level Select
				</Button>
			</div>
		</div>
	</Box>
</main>
