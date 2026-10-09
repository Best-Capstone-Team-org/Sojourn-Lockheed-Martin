<script lang="ts">
	import type { HTMLButtonAttributes } from "svelte/elements";

	let {
		checked = $bindable(false),
		trackClass = "",
		wellClass = "",
		onclick,
		onchange,
		...rest
	}: Omit<HTMLButtonAttributes, "type" | "role" | "onchange"> & {
		checked?: boolean;
		onchange?: (checked: boolean) => void;
		trackClass?: string;
		wellClass?: string;
	} = $props();

	// constants - tune as needed
	const TEETH = 6;
	const TOOTH_H = 6; // base height in px
	const TOOTH_PEAK_H = 4; // peak height in px

	// compute tooth geometry
	const toothW = 100 / TEETH;

	const clip = (() => {
		const pts: string[] = [];
		for (let i = 0; i < TEETH; i++) {
			pts.push(`${(i * toothW).toFixed(0)}% ${TOOTH_H}px`);
			pts.push(`${((i + 0.5) * toothW).toFixed(3)}% ${TOOTH_PEAK_H}px`);
		}
		pts.push(`100% ${TOOTH_H}px`, "100% 100%", "0% 100%");
		return `polygon(${pts.join(",")})`;
	})();

	const teethShading = (() => {
		const stops: string[] = [];
		for (let i = 0; i < TEETH; i++) {
			const left = i * toothW;
			const crest = (i + 0.5) * toothW;
			const right = (i + 1) * toothW;
			stops.push(
				`rgb(255 255 255 / 0.2) ${(left + toothW * 0.08).toFixed(3)}%`, // right valley
				`rgb(255 255 255 / 0.5) ${(crest - toothW * 0.01).toFixed(3)}%`, // right peak
				`rgb(0 0 0 / 0.1) ${crest.toFixed(3)}%`, // left peak
				`rgb(0 0 0 / 0.5) ${right.toFixed(3)}%` // left valley
			);
		}
		return `linear-gradient(90deg, ${stops.join(", ")})`;
	})();
</script>

<div class={["relative h-7 w-14 rounded p-0", wellClass]}>
	<button
		type="button"
		role="switch"
		aria-checked={checked}
		class={[
			"relative h-full w-full cursor-pointer rounded",
			"noise-bg shadow-[inset_0_2px_4px_rgb(0_0_0/0.8),inset_0_0_6px_rgb(0_0_0/0.4)] noise-opacity-15",
			"transition-colors duration-150",
			checked ? "bg-sys-screen-dark-green" : "bg-sys-medium-grey",
			trackClass,
		]}
		onclick={(e) => {
			checked = !checked;
			onchange?.(checked);
			onclick?.(e);
		}}
		{...rest}
	>
		<span
			class={[
				"absolute -top-1.5 left-0 h-[calc(100%+6px)] w-1/2 overflow-hidden rounded-b",
				"transition-transform duration-150 ease-in-out backface-hidden",
				checked ? "translate-x-full" : "translate-x-0",
			]}
			aria-hidden="true"
		>
			<span
				class="noise-bg absolute inset-0 rounded-b bg-sys-switch noise-opacity-40"
				style:clip-path={clip}
				style:--teeth-shading={teethShading}
			>
				<!-- teeth -->
				<span
					aria-hidden="true"
					class={[
						"pointer-events-none absolute inset-0 bg-size-[100%_100%] bg-no-repeat",
						"bg-[linear-gradient(to_bottom,rgb(255_255_255/0.15),transparent_35%,rgb(0_0_0/0.3)),var(--teeth-shading)]",
					]}
				></span>

				<!-- bottom -->
				<span
					class="noise-bg absolute inset-0 translate-y-6.5 rounded-b bg-sys-switch-bottom noise-opacity-20"
					style:clip-path={clip}
					style:--teeth-shading={teethShading}
				>
				</span>
			</span>
		</span>
	</button>
</div>
