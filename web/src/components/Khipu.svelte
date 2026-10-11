<script lang="ts">
  import type { ProjectRow } from "../lib/api/types";
  import { count, tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { link, projectPath } from "../lib/router.svelte";

  // A khipu drawn the way the Incas recorded numbers (Locke, 1912). Each pendant cord is a project
  // and records its cache writes in decimal knots: clusters of single knots for thousands, hundreds
  // and tens, a long knot with 2–9 turns or a figure-eight (1) for units, nothing for 0. Places sit
  // at fixed heights, so knots line up in rows across the khipu. Colour depth and length also follow
  // cache writes. Findings hang as a red subsidiary cord whose knot counts them. The first, darker
  // pendant records the sum, as summary cords do on real khipus.
  let { projects }: { projects: ProjectRow[] } = $props();

  const limit = 24;
  const top = 30; // the primary cord
  const x0 = 64;
  const places = [36, 80, 124, 168]; // y of thousands, hundreds, tens and units below the attachment
  const clusterStep = 5.4;
  const turnStep = 3.4;

  let available = $state(900);

  const shown = $derived([...projects].sort((a, b) => b.totals.cacheWrite - a.totals.cacheWrite).slice(0, limit));
  const total = $derived(projects.reduce((s, p) => s + p.totals.cacheWrite, 0));
  // One unit for the whole khipu, so that the sum fits in four places (at most 9999).
  const unit = $derived(Math.pow(10, Math.max(0, Math.ceil(Math.log10(Math.max(total, 1) / 9999)))));
  const max = $derived(Math.max(1, ...shown.map((p) => p.totals.cacheWrite)));

  const gap = $derived(Math.min(80, Math.max(44, (available - x0 - 70) / (shown.length + 1))));
  const end = $derived(x0 + shown.length * gap + 56);
  const width = $derived(Math.max(available, end + 16));

  function hash(s: string) {
    let h = 7;
    for (const c of s) h = (h * 31 + c.charCodeAt(0)) >>> 0;
    return h;
  }

  // y of the sagging primary cord at x
  function primaryY(x: number) {
    const u = (x - 12) / (end - 12);
    return (1 - u) * (1 - u) * top + 2 * (1 - u) * u * (top + 16) + u * u * (top - 2);
  }

  type Knot = { kind: "single" | "long" | "eight"; y: number; n: number };
  type Cord = {
    key: string;
    href?: string;
    label: string;
    title: string;
    x: number;
    y: number;
    len: number;
    path: string;
    depth: number;
    sum: boolean;
    knots: Knot[];
    at: (y: number) => number;
    findings: number;
  };

  function cord(name: string, writes: number, findings: number, i: number, sum: boolean, dir?: string): Cord {
    const h = hash(name);
    const x = x0 + i * gap;
    const share = sum ? 1 : Math.sqrt(writes / max);
    const len = sum ? 260 : 196 + share * 70 + ((h >> 3) % 9);
    const sway = sum ? 0 : (((h >> 7) % 5) - 2) * 0.8;
    // x offset of the cord at height y (the curve's control points are evenly spaced in y)
    const at = (y: number) => {
      const q = y / len;
      const u = 1 - q;
      return sway * (3 * u * u * q + 1.8 * u * q * q + 0.3 * q * q * q);
    };
    const v = Math.round(writes / unit);
    const digits = [Math.floor(v / 1000) % 10, Math.floor(v / 100) % 10, Math.floor(v / 10) % 10, v % 10];
    const knots: Knot[] = [];
    digits.forEach((n, place) => {
      if (!n) return;
      const y = places[place]! + 10;
      if (place < 3) {
        for (let k = 0; k < n; k++) knots.push({ kind: "single", y: y - ((n - 1) * clusterStep) / 2 + k * clusterStep, n: 1 });
      } else if (n === 1) knots.push({ kind: "eight", y: y - 6, n: 1 });
      else knots.push({ kind: "long", y: y - (n * turnStep) / 2, n });
    });
    const label = sum ? t("khipu.total") : name;
    return {
      key: dir ?? "total",
      href: dir ? projectPath(dir) : undefined,
      label: label.length > 16 ? label.slice(0, 15) + "…" : label,
      title: sum
        ? t("khipu.totalTitle", { writes: tokens(writes) })
        : `${name}: ${t("khipu.cord", { writes: tokens(writes), count: findings })}`,
      x,
      y: primaryY(x),
      len,
      path: `M0 0 C ${sway} ${len / 3}, ${sway * 0.6} ${(2 * len) / 3}, ${sway * 0.3} ${len}`,
      depth: share,
      sum,
      knots,
      at,
      findings: Math.min(findings, 9),
    };
  }

  const cords = $derived([
    cord("total", total, 0, 0, true),
    ...shown.map((p, i) => cord(p.name, p.totals.cacheWrite, p.findings, i + 1, false, p.dir)),
  ]);
  // Labels share one baseline under the longest cord, so slanted labels never overlap.
  const labelY = $derived(top + 40 + Math.max(...cords.map((c) => c.len)));
  const height = $derived(labelY + 52);
  const primary = $derived(`M12 ${top} Q ${(12 + end) / 2} ${top + 16} ${end} ${top - 2}`);
</script>

{#snippet single(x: number, y: number)}
  <ellipse cx={x} cy={y} rx="5.6" ry="3.6" class="fibre" />
  <ellipse cx={x} cy={y} rx="5.6" ry="3.6" fill="url(#k-volume)" />
  <path d="M{x - 4.6} {y + 2} Q {x} {y - 4.4} {x + 4.8} {y - 0.6}" class="strand-shadow" />
  <path d="M{x - 4.6} {y + 2} Q {x} {y - 4.4} {x + 4.8} {y - 0.6}" class="strand" />
  <path d="M{x - 3} {y - 0.4} Q {x} {y - 3.4} {x + 3} {y - 1.6}" class="gloss" />
{/snippet}

{#snippet long(x: number, y: number, n: number)}
  {#each { length: n } as _, k (k)}
    {@const yy = y + k * turnStep + 2}
    <g transform="rotate(-14 {x} {yy})">
      <ellipse cx={x} cy={yy} rx="5.4" ry="2.6" class="fibre turn" />
      <ellipse cx={x} cy={yy} rx="5.4" ry="2.6" fill="url(#k-volume)" />
      <path d="M{x - 3.4} {yy - 1.1} Q {x} {yy - 2.2} {x + 3.4} {yy - 1.1}" class="gloss" />
    </g>
  {/each}
{/snippet}

{#snippet eight(x: number, y: number)}
  <ellipse cx={x} cy={y + 3.4} rx="5" ry="3.8" class="fibre" />
  <ellipse cx={x} cy={y + 3.4} rx="5" ry="3.8" fill="url(#k-volume)" />
  <ellipse cx={x} cy={y + 9.8} rx="5" ry="3.8" class="fibre" />
  <ellipse cx={x} cy={y + 9.8} rx="5" ry="3.8" fill="url(#k-volume)" />
  <path d="M{x - 4} {y + 1.4} C {x + 6} {y + 4.4}, {x - 6} {y + 8.8}, {x + 4} {y + 11.8}" class="strand-shadow" />
  <path d="M{x - 4} {y + 1.4} C {x + 6} {y + 4.4}, {x - 6} {y + 8.8}, {x + 4} {y + 11.8}" class="strand" />
{/snippet}

<figure class="khipu">
  <div class="overflow-x-auto pb-1" bind:clientWidth={available}>
    <svg viewBox="0 0 {width} {height}" {width} {height} role="group" aria-label={t("khipu.label")}>
      <defs>
        <!-- the twist of spun cotton, laid over any fibre colour -->
        <pattern id="k-twist" width="4" height="4" patternUnits="userSpaceOnUse" patternTransform="rotate(32)">
          <rect width="1.6" height="4" class="ply-dark" /><rect x="2.2" width="0.7" height="4" class="ply-light" />
        </pattern>
        <pattern id="k-twist-primary" width="4.5" height="4.5" patternUnits="userSpaceOnUse" patternTransform="rotate(-36)">
          <rect width="2" height="4.5" class="ply-dark soft" /><rect x="2.8" width="0.8" height="4.5" class="ply-light" />
        </pattern>
        <!-- light from the upper left makes any knot look round -->
        <radialGradient id="k-volume" cx="35%" cy="30%" r="75%">
          <stop offset="0" stop-color="#fff" stop-opacity="0.5" />
          <stop offset="0.45" stop-color="#fff" stop-opacity="0" />
          <stop offset="1" stop-color="#000" stop-opacity="0.55" />
        </radialGradient>
        <radialGradient id="k-glow" cx="45%" cy="40%" r="50%" gradientTransform="translate(0 0.1) scale(1 0.8)">
          <stop offset="0" class="glow-in" />
          <stop offset="1" class="glow-out" />
        </radialGradient>
        <filter id="k-shadow" x="-50%" y="-20%" width="200%" height="140%">
          <feDropShadow dx="0.8" dy="1.8" stdDeviation="1.2" class="drop" />
        </filter>
      </defs>

      <rect {width} {height} fill="url(#k-glow)" />
      <g filter="url(#k-shadow)">
        <path d={primary} class="primary" />
        <path d={primary} class="primary twist" />
      </g>

      {#each cords as c (c.key)}
        <svelte:element this={c.href ? "a" : "g"} href={c.href} use:link class="cord" class:sum={c.sum}
          style="--depth: {Math.round(c.depth * 100)}%" aria-label={c.title} role={c.href ? undefined : "img"}>
          <title>{c.title}</title>
          <g transform="translate({c.x} {c.y})" filter="url(#k-shadow)">
            <rect x={-gap / 2} y="-6" width={gap} height={c.len + 16} fill="transparent" />
            <path d={c.path} class="pendant" />
            <path d={c.path} class="pendant twist" />
            {#each c.knots as k, j (j)}
              {#if k.kind === "single"}{@render single(c.at(k.y), k.y)}
              {:else if k.kind === "eight"}{@render eight(c.at(k.y), k.y)}
              {:else}{@render long(c.at(k.y), k.y, k.n)}{/if}
            {/each}
            {#if c.findings}
              {@const ax = c.at(16)}
              {@const tail = 68 + c.findings * 2}
              {@const sub = `M${ax} 16 C ${ax + 7} 24, ${ax + 12} 36, ${ax + 13} ${tail}`}
              <g class="finding">
                <path d={sub} class="subsidiary" />
                <path d={sub} class="subsidiary twist" />
                <circle cx={ax + 0.6} cy="16" r="2.8" class="fibre" />
                {#if c.findings === 1}{@render eight(ax + 12.2, 36)}{:else}{@render long(ax + 12.2, 32, c.findings)}{/if}
                <path d="M{ax + 13} {tail} q -1 4 -2.6 6 M{ax + 13} {tail} q 0 4 0.3 7 M{ax + 13} {tail} q 1 4 2.4 5.5" class="fray thin" />
              </g>
            {/if}
            <path d="M{c.at(c.len)} {c.len} q -1.6 5 -3.8 9 M{c.at(c.len)} {c.len} q -0.2 6 0.4 11 M{c.at(c.len)} {c.len} q 1.5 5 3.4 8" class="fray" />
            <!-- the half hitch that ties the pendant over the primary cord -->
            <path d="M-4 -3 Q 0 -7 4 -3 L 3.4 4 Q 0 6.4 -3.4 4 Z" class="fibre" />
            <path d="M-4 -3 Q 0 -7 4 -3 L 3.4 4 Q 0 6.4 -3.4 4 Z" fill="url(#k-volume)" />
          </g>
          <text x={c.x} y={labelY} class="name" text-anchor="end" transform="rotate(-35 {c.x} {labelY})">{c.label}</text>
        </svelte:element>
      {/each}
    </svg>
  </div>
  <figcaption class="mt-1 max-w-[80ch] text-xs text-fleece">
    {t("khipu.legend", { unit: count(unit) })}
    {#if projects.length > shown.length}{t("khipu.more", { shown: shown.length, total: projects.length })}{/if}
  </figcaption>
</figure>

<style>
  /* Fibre colours: undyed cotton to deep qolle orange by cache writes; the sum cord is brown.
     The khipu is the one bold element of the UI, so it has its own dyes next to the tokens. */
  .khipu {
    --pale: #e6dcc4;
    --deep: #f08a24;
    --sum: #7a5a3a;
    --primary: #e3d6b8;
    --shade: rgb(0 0 0 / 0.42);
    --light: rgb(255 255 255 / 0.3);
    --glow: #f2b45a;
    --glow-strength: 0.13;
    --drop: 0.6;
  }
  :global(:root[data-theme="light"]) .khipu {
    --pale: #cdb994;
    --deep: #c2620e;
    --sum: #5e4128;
    --primary: #bba67c;
    --shade: rgb(0 0 0 / 0.3);
    --light: rgb(255 255 255 / 0.45);
    --glow: #ffffff;
    --glow-strength: 0.75;
    --drop: 0.22;
  }
  .cord {
    --fibre: color-mix(in oklab, var(--deep) var(--depth), var(--pale));
    transition: opacity 150ms ease;
  }
  .cord.sum {
    --fibre: var(--sum);
  }
  .finding {
    --fibre: var(--color-cochineal);
  }
  .primary {
    fill: none;
    stroke: var(--primary);
    stroke-width: 7;
    stroke-linecap: round;
  }
  .pendant,
  .subsidiary {
    fill: none;
    stroke: var(--fibre);
    stroke-width: 4;
    stroke-linecap: round;
  }
  .sum .pendant {
    stroke-width: 5;
  }
  .subsidiary {
    stroke-width: 2.8;
  }
  .pendant.twist,
  .subsidiary.twist {
    stroke: url(#k-twist);
  }
  .primary.twist {
    stroke: url(#k-twist-primary);
  }
  .fibre {
    fill: var(--fibre);
  }
  .turn {
    stroke: var(--shade);
    stroke-width: 0.6;
  }
  .strand {
    fill: none;
    stroke: var(--fibre);
    stroke-width: 1.8;
    stroke-linecap: round;
  }
  .strand-shadow {
    fill: none;
    stroke: var(--shade);
    stroke-width: 3;
    stroke-linecap: round;
  }
  .gloss {
    fill: none;
    stroke: var(--light);
    stroke-width: 0.9;
    stroke-linecap: round;
  }
  .fray {
    fill: none;
    stroke: var(--fibre);
    stroke-width: 1.2;
    stroke-linecap: round;
  }
  .fray.thin {
    stroke-width: 1;
  }
  .ply-dark {
    fill: var(--shade);
  }
  .ply-dark.soft {
    opacity: 0.4;
  }
  .ply-light {
    fill: var(--light);
  }
  .glow-in {
    stop-color: var(--glow);
    stop-opacity: var(--glow-strength);
  }
  .glow-out {
    stop-color: var(--glow);
    stop-opacity: 0;
  }
  .drop {
    flood-color: #000;
    flood-opacity: var(--drop);
  }
  .name {
    fill: var(--color-fleece);
    font-size: 11.5px;
  }
  .sum .name {
    fill: var(--color-cotton);
    font-weight: 600;
  }
  /* Pointing at one cord dims the others. */
  svg:has(a.cord:hover, a.cord:focus-visible) .cord:not(:hover, :focus-visible) {
    opacity: 0.45;
  }
  a.cord:hover .name,
  a.cord:focus-visible .name {
    fill: var(--color-cotton);
  }
  a.cord:focus-visible {
    outline: none;
  }
  a.cord:focus-visible .name {
    text-decoration: underline;
  }
  @media (prefers-reduced-motion: reduce) {
    .cord {
      transition: none;
    }
  }
</style>
