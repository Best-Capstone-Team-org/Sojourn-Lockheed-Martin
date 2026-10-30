<script lang="ts">
	import type { HTMLAttributes } from "svelte/elements";

	let { class: className = "", ...rest }: HTMLAttributes<HTMLDivElement> =
		$props();

	const uid = $props.id();

	// length of the horizontal arm in px
	const WIDTH = 48;

	// length of the vertical arm in px
	const HEIGHT = 24;

	// thickness of each arm in px
	const THICK = 12;

	type Point = [number, number];
	type Edge = { from: Point; to: Point; lit: boolean };

	// top-left L shape
	const baseL: Point[] = [
		[0, 0],
		[WIDTH, 0],
		[WIDTH, THICK],
		[THICK, THICK],
		[THICK, HEIGHT],
		[0, HEIGHT],
	];

	// shade edges
	function edgesOf(points: Point[]): Edge[] {
		let area = 0;
		points.forEach(([x1, y1], i) => {
			const [x2, y2] = points[(i + 1) % points.length];
			area += x1 * y2 - x2 * y1;
		});
		return points.map((from, i) => {
			const to = points[(i + 1) % points.length];
			const dx = to[0] - from[0];
			const dy = to[1] - from[1];
			const [nx, ny] = area > 0 ? [dy, -dx] : [-dy, dx];
			return { from, to, lit: nx < 0 || ny < 0 };
		});
	}

	// map corners
	const corners = [
		{ pos: "top-0 left-0", flipX: false, flipY: false },
		{ pos: "top-0 right-0", flipX: true, flipY: false },
		{ pos: "bottom-0 left-0", flipX: false, flipY: true },
		{ pos: "bottom-0 right-0", flipX: true, flipY: true },
	].map((c, i) => {
		const points = baseL.map(([x, y]): Point => [
			c.flipX ? WIDTH - x : x,
			c.flipY ? HEIGHT - y : y,
		]);
		return {
			...c,
			clipId: `${uid}-clip-${i}`,
			polygon: points.map((p) => p.join(",")).join(" "),
			cssPolygon: `polygon(${points.map(([x, y]) => `${x}px ${y}px`).join(", ")})`,
			edges: edgesOf(points),
		};
	});
</script>

<div
	class={["pointer-events-none", className]}
	aria-hidden="true"
	{...rest}
>
	{#each corners as corner (corner.pos)}
		<div class={["absolute", corner.pos]}>
			<div
				class="noise-bg bg-sys-dark-grey"
				style:clip-path={corner.cssPolygon}
			>
				<svg
					class="block"
					width={WIDTH}
					height={HEIGHT}
					viewBox="0 0 {WIDTH} {HEIGHT}"
					shape-rendering="crispEdges"
				>
					<defs>
						<clipPath id={corner.clipId}>
							<polygon points={corner.polygon} />
						</clipPath>
					</defs>

					<g clip-path="url(#{corner.clipId})">
						{#each corner.edges as edge, i (i)}
							<line
								x1={edge.from[0]}
								y1={edge.from[1]}
								x2={edge.to[0]}
								y2={edge.to[1]}
								stroke={edge.lit
									? "rgb(255 255 255 / 0.55)"
									: "rgb(0 0 0 / 0.4)"}
								stroke-width="4"
							/>
						{/each}
						<polygon
							points={corner.polygon}
							fill="none"
							stroke="var(--color-sys-dark-grey)"
							stroke-width="2"
						/>
					</g>
				</svg>
			</div>
		</div>
	{/each}
</div>
