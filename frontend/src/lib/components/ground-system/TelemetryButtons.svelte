<script lang="ts">
    import Box from "$lib/components/Box/Box.svelte";
    import Button from "$lib/components/Button/Button.svelte";
</script>

<Box class="w-full h-full p-2">
    <div class="flex flex-row gap-2 ">
        <div class="flex flex-col gap-2 w-1/2">
            <Button buttonClass="p-1 text-xs">
                ▲
            </Button>
            <Button buttonClass="p-1 text-xs">
                ▼
            </Button>
        </div>
        <Button buttonClass="p-2">
            Download Binary
        </Button>
    </div>
</Box>