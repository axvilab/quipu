<script lang="ts">
  import type { ProjectRow } from "../lib/api/types";
  import { tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { link, projectPath } from "../lib/router.svelte";

  // A khipu drawn from the data: a primary cord and one pendant cord per project. Length = cache
  // writes in the period, colour depth = share of the total, knots = findings to fix.
  let { projects }: { projects: ProjectRow[] } = $props();

  // Cords spread over the available width; labels hang to the left, so the first cord starts
  // far enough in for its name.
  let available = $state(900);
  const top = 22;
  const sag = 14;
  const maxLen = 128;

  const shown = $derived([...projects].sort((a, b) => b.totals.cacheWrite - a.totals.cacheWrite).slice(0, 24));
  const startX = 90;
  const gap = $derived(Math.min(120, Math.max(44, (available - startX - 40) / Math.max(shown.length, 1))));
  const width = $derived(Math.max(available, startX + shown.length * gap));
  // The primary cord ends a little after the last pendant, so a few projects do not leave it bare.
  const cordEnd = $derived(Math.min(width - 6, startX + (shown.length - 1) * gap + 90));
  const max = $derived(Math.max(1, ...shown.map((p) => p.totals.cacheWrite)));
  const total = $derived(shown.reduce((s, p) => s + p.totals.cacheWrite, 0) || 1);

  // y of the primary cord (a quadratic sag between x = 6 and cordEnd) at x.
  function cordY(x: number) {
    const u = (x - 6) / (cordEnd - 6);
    return top + 2 * (1 - u) * u * 2 * sag;
  }

  const cords = $derived(
    shown.map((p, i) => {
      const x = startX + i * gap;
      const y = cordY(x);
      const len = 22 + Math.sqrt(p.totals.cacheWrite / max) * maxLen;
      const share = p.totals.cacheWrite / total;
      const knots = Array.from({ length: Math.min(p.findings, 4) }, (_, k) => y + len * (0.3 + k * 0.15));
      return { p, x, y, len, share, knots };
    }),
  );
</script>

<figure class="khipu overflow-x-auto pb-1" bind:clientWidth={available}>
  <svg viewBox="0 0 {width} {top + sag + maxLen + 70}" width={width} height={top + sag + maxLen + 70} role="group" aria-label={t("khipu.label")}>
    <path d="M6 {top} Q {(6 + cordEnd) / 2} {top + 2 * sag} {cordEnd} {top}" class="primary" />
    {#each cords as c, i (c.p.dir)}
      <a href={projectPath(c.p.dir)} use:link
        aria-label={t("khipu.cord", { name: c.p.name, writes: tokens(c.p.totals.cacheWrite), count: c.p.findings })}>
        <g class="cord" style="--i: {i}">
          <title>{c.p.name}: {t("khipu.cord", { name: c.p.name, writes: tokens(c.p.totals.cacheWrite), count: c.p.findings })}</title>
          <rect x={c.x - gap / 2} y={c.y} width={gap} height={c.len + 64} fill="transparent" />
          <path
            d="M{c.x} {c.y} C {c.x + 3} {c.y + c.len / 3}, {c.x - 3} {c.y + (2 * c.len) / 3}, {c.x} {c.y + c.len}"
            pathLength="1"
            class="pendant"
            style="--depth: {0.35 + Math.min(c.share * 2.5, 0.65)}"
          />
          {#each c.knots as ky, k (k)}
            <circle cx={c.x} cy={ky} r="3.6" class="knot" />
          {/each}
          <circle cx={c.x} cy={c.y + c.len} r="2.4" class="end" />
          <text x={c.x} y={c.y + c.len + 16} class="name" transform="rotate(-35 {c.x} {c.y + c.len + 16})" text-anchor="end">{c.p.name}</text>
        </g>
      </a>
    {/each}
  </svg>
  <figcaption class="mt-1 text-xs text-fleece">{t("khipu.legend")}</figcaption>
</figure>

<style>
  .primary {
    fill: none;
    stroke: var(--color-cotton);
    stroke-width: 2.5;
    stroke-linecap: round;
  }
  .pendant {
    fill: none;
    stroke: color-mix(in oklab, var(--color-qolle) calc(var(--depth) * 100%), var(--color-fleece));
    stroke-width: 2.2;
    stroke-linecap: round;
    stroke-dasharray: 1;
    stroke-dashoffset: 1;
    animation: draw 700ms cubic-bezier(0.2, 0.7, 0.2, 1) forwards;
    animation-delay: calc(var(--i) * 35ms);
  }
  .knot {
    fill: var(--color-cochineal);
  }
  .end {
    fill: var(--color-cotton);
  }
  .name {
    fill: var(--color-fleece);
    font-size: 11.5px;
  }
  a:hover .name,
  a:focus-visible .name {
    fill: var(--color-cotton);
  }
  a:hover .pendant,
  a:focus-visible .pendant {
    stroke-width: 3.2;
  }
  a:focus-visible {
    outline: none;
  }
  @keyframes draw {
    to {
      stroke-dashoffset: 0;
    }
  }
</style>
