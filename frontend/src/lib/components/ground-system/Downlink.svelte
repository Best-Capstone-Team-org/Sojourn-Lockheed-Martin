<script lang="ts">
	import TerminalScroll from "../TerminalScroll/TerminalScroll.svelte";

	let { tlm }: { tlm: string[] } = $props();
</script>

<TerminalScroll class="p-2">
	{#each tlm as hex, index (index)}
		<p class="break-all pb-2 text-sm">{hex}</p>
	{/each}
</TerminalScroll>