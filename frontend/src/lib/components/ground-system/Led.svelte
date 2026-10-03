<script lang="ts">
	type Color = "off" | "red" | "yellow" | "green";

	let { color = "off", fade = 2000 }: { color?: Color; fade?: number } =
		$props();

	const ledStyle: Record<Color, string> = {
		off: "bg-neutral-700 inset-shadow-recess [--hl:0.5]",
		red: "bg-sys-alert-red shadow-[0_0_6px_1px] shadow-sys-alert-red/60 [--hl:0.85]",
		yellow: "bg-sys-alert-yellow shadow-[0_0_6px_1px] shadow-sys-alert-yellow/60 [--hl:0.85]",
		green: "bg-sys-alert-green shadow-[0_0_6px_1px] shadow-sys-alert-green/60 [--hl:0.85]",
	};

	const lens =
		"bg-[radial-gradient(circle_at_30%_28%,rgb(255_255_255/var(--hl)),transparent_45%),radial-gradient(circle_at_70%_78%,transparent_45%,rgb(0_0_0/0.3))]";

	let led: HTMLDivElement;
	let flashColor = $state<Exclude<Color, "off">>("green");
	let animation: Animation | undefined;

	export function blink(blinkColor: Exclude<Color, "off"> = "green") {
		flashColor = blinkColor;
		animation?.cancel();
		animation = led.animate([{ opacity: 1 }, { opacity: 0 }], {
			duration: fade,
			easing: "cubic-bezier(0.5, 0, 0.75, 0)",
		});
	}
</script>

<div
	class="flex size-4 items-center justify-center rounded-full bg-sys-medium-grey shadow-[0_1px_0_rgb(255_255_255/0.4)] inset-shadow-recess"
>
	<div class={["relative size-3 rounded-full", lens, ledStyle[color]]}>
		<div
			bind:this={led}
			class={[
				"absolute inset-0 rounded-full opacity-0",
				lens,
				ledStyle[flashColor],
			]}
		></div>
	</div>
</div>
