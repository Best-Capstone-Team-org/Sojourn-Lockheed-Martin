<script lang="ts">
	import { onMount } from "svelte";

	type Line = {
		text: string;
		class?: string;
	};

	let {
		lines,
		onLineComplete = () => {},
	}: {
		lines: Line[];
		onLineComplete?: () => void;
	} = $props();

	const START_MS = 900;
	const CHAR_MS = 25;
	const LINE_MS = 300;

	let line = $state(0);
	let typed = $state(0);
	const cursorAt = $derived(Math.min(line, lines.length - 1));

	function shown(i: number) {
		return i === line ? lines[i].text.slice(0, typed) : lines[i].text;
	}

	onMount(() => {
		if (matchMedia("(prefers-reduced-motion: reduce)").matches) {
			line = lines.length;
			return;
		}

		let timer: ReturnType<typeof setTimeout>;
		const type = () => {
			if (typed < lines[line].text.length) {
				typed++;
				timer = setTimeout(type, CHAR_MS);
				return;
			}
			onLineComplete();
			line++;
			typed = 0;
			if (line < lines.length) timer = setTimeout(type, LINE_MS);
		};
		timer = setTimeout(type, START_MS);

		return () => clearTimeout(timer);
	});
</script>

<div
	class="glow text-[length:min(1.2vw,2.1vh)] leading-relaxed whitespace-pre-wrap"
>
	{#each lines as l, i (i)}
		<p class={[l.class, i > line && "invisible"]}>
			{shown(i)}{#if i === cursorAt}<span class="cursor">█</span>{/if}
		</p>
	{/each}
</div>

<style>
	.cursor {
		animation: blink 1s steps(1) infinite;
	}

	@keyframes blink {
		50% {
			opacity: 0;
		}
	}

	.glow {
		text-shadow: 0 0 0.3em rgb(255 255 255 / 0.45);
	}
</style>
