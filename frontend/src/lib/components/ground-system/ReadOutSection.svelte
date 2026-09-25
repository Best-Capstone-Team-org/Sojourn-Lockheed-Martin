<script lang="ts">
    import Box from "$lib/components/Box/Box.svelte";

    let { title, items }: { 
        title: string,
        items: { label: string, value: string | number | null | undefined, unit?: string }[]
    } = $props();

    const isMissing = (value: string | number | null | undefined) =>
        value === undefined || value === null || value === "";
</script>

<Box class="w-full p-2">
    <span class="font-semibold">{title}</span>
    <br />
    {#each items as item (item.label)}
        <div class="flex justify-between">
            <span>{item.label}</span>
            {#if isMissing(item.value)}
                <span class="text-sys-alert-red">absent</span>
            {:else}
                <span>{item.value}{item.unit ? ` ${item.unit}` : ""}</span>
            {/if}
        </div>
    {/each}
</Box>