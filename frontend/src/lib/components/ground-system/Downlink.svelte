<script lang="ts">
	let { tlmHex }: { tlmHex: string[] } = $props();
</script>

<div class="flex h-full flex-col-reverse p-2">
	{#each tlmHex as hex}
		<p class="text-sm">{hex}</p>
	{/each}
</div>
