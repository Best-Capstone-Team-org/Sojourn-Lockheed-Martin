<script lang="ts">
	type Color = "off" | "red" | "yellow" | "green";

	let { color = "off" }: { color?: Color } = $props();

	const led: Record<Color, string> = {
		off: "bg-neutral-700 inset-shadow-recess [--hl:0.5]",
		red: "bg-sys-alert-red shadow-[0_0_6px_1px] shadow-sys-alert-red/60 [--hl:0.85]",
		yellow: "bg-sys-alert-yellow shadow-[0_0_6px_1px] shadow-sys-alert-yellow/60 [--hl:0.85]",
		green: "bg-sys-alert-green shadow-[0_0_6px_1px] shadow-sys-alert-green/60 [--hl:0.85]",
	};
</script>

<div
	class="flex size-4 items-center justify-center rounded-full bg-sys-medium-grey shadow-[0_1px_0_rgb(255_255_255/0.4)] inset-shadow-recess"
>
	<div
		class={[
			"size-3 rounded-full",
			"bg-[radial-gradient(circle_at_30%_28%,rgb(255_255_255/var(--hl)),transparent_45%),radial-gradient(circle_at_70%_78%,transparent_45%,rgb(0_0_0/0.3))]",
			led[color],
		]}
	></div>
</div>
