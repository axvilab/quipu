<script lang="ts">
  import type { Day } from "../lib/api/types";
  import { day, percent, tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";

  // Cache writes per day with axis values and a tooltip; colour = cost (qolle).
  let { days }: { days: Day[] } = $props();

  let width = $state(600);
  const height = 200;
  const top = 10; // room for the top axis label
  const bottom = 22;

  const max = $derived(Math.max(1, ...days.map((d) => d.totals.cacheWrite)));
  const ticks = $derived([0, max / 2, max]);
  // The axis is as wide as its longest label ("28,6 млн" is longer than "28.6M").
  const left = $derived(Math.max(32, ...ticks.map((v) => tokens(v).length * 6.6 + 12)));
  const bar = $derived(days.length ? (width - left) / days.length : 0);
  let hover = $state<number | null>(null);
  const y = (v: number) => top + (height - bottom - top) * (1 - v / max);
</script>

<div class="relative" bind:clientWidth={width}>
  <svg {width} {height} role="img" aria-label={t("chart.writesLabel")}>
    {#each ticks as tick (tick)}
      <line x1={left} x2={width} y1={y(tick)} y2={y(tick)} class="stroke-warp" stroke-dasharray={tick ? "2 4" : ""} />
      <text x={left - 8} y={y(tick) + 4} text-anchor="end" class="fill-fleece text-[11px]">{tokens(tick)}</text>
    {/each}
    {#each days as d, i (d.day)}
      <rect
        x={left + i * bar + Math.min(2, bar * 0.15)}
        y={y(d.totals.cacheWrite)}
        width={Math.max(bar - Math.min(4, bar * 0.3), 1)}
        height={Math.max(height - bottom - y(d.totals.cacheWrite), 1)}
        rx="2"
        class={hover === i ? "fill-qolle" : "fill-qolle/70"}
        onmouseenter={() => (hover = i)}
        onmouseleave={() => (hover = null)}
        role="presentation"
      />
    {/each}
    {#if days.length}
      <text x={left} y={height - 4} class="fill-fleece text-[11px]">{day(days[0]!.day)}</text>
      <text x={width} y={height - 4} text-anchor="end" class="fill-fleece text-[11px]">{day(days[days.length - 1]!.day)}</text>
    {/if}
  </svg>
  {#if hover !== null && days[hover]}
    {@const d = days[hover]!}
    <div
      class="pointer-events-none absolute top-0 z-10 rounded-md border border-warp bg-loom px-3 py-2 text-xs shadow-xl"
      style="left: {Math.min(left + hover * bar, width - 180)}px"
    >
      <p class="font-semibold">{day(d.day, { weekday: "short", day: "numeric", month: "long" })}</p>
      <p class="mt-1 text-qolle">{t("chart.writes", { tokens: tokens(d.totals.cacheWrite) })}</p>
      <p class="text-fleece">{t("chart.hit", { rate: percent(d.totals.hitRate) })}</p>
      <p class="text-fleece">{t("chart.requests", { count: d.totals.requests })}</p>
    </div>
  {/if}
  <table class="sr-only">
    <caption>{t("chart.writesLabel")}</caption>
    <tbody>
      {#each days as d (d.day)}<tr><th>{d.day}</th><td>{d.totals.cacheWrite}</td></tr>{/each}
    </tbody>
  </table>
</div>
