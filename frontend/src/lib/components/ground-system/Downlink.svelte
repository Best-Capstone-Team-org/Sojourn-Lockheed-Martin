<script lang="ts">
	import TerminalScroll from "../TerminalScroll/TerminalScroll.svelte";
	import { options } from "./Options.svelte";

	let { tlm }: { tlm: string[] } = $props();

	function visible(hex: string): boolean {
		if (hex.startsWith("TLM ")) {
			return options.telemetry;
		} else {
			return true;
		}
	}
</script>

<TerminalScroll class="border-b border-white p-3">
	{#each tlm as hex, index (index)}
		{#if visible(hex)}
			<p class="pb-1 text-sm break-all">
				{hex}
			</p>
		{/if}
	{/each}
</TerminalScroll>
