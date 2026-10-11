<script lang="ts">
  import { api } from "../lib/api/client";
  import type { CacheReport } from "../lib/api/types";
  import { count, percent, tokens } from "../lib/format";
  import { has, t } from "../lib/i18n.svelte";
  import { prefs } from "../lib/prefs.svelte";
  import DailyChart from "../components/DailyChart.svelte";
  import Header from "../components/Header.svelte";
  import Loading from "../components/Loading.svelte";
  import PeriodSwitch from "../components/PeriodSwitch.svelte";
  import Stat from "../components/Stat.svelte";

  let data = $state<CacheReport | null>(null);
  let error = $state<unknown>(null);
  $effect(() => {
    void prefs.days;
    api.cache(prefs.days).then((d) => ((data = d), (error = null))).catch((e) => (error = e));
  });
  const maxMiss = $derived(Math.max(1, ...(data?.misses.map((m) => m.count) ?? [1])));
</script>

<Header title={t("cache.title")} subtitle={t("cache.subtitle")}>
  {#snippet actions()}<PeriodSwitch />{/snippet}
</Header>

{#if !data}
  <Loading {error} />
{:else}
  <dl class="grid grid-cols-2 gap-6 border-y border-warp py-5 md:grid-cols-4">
    <Stat label={t("stat.cacheWrites")} value={tokens(data.totals.cacheWrite)} current={data.totals.cacheWrite} previous={data.previous.cacheWrite} cost note={t("stat.cacheWritesNote")} />
    <Stat label={t("stat.hitRate")} value={percent(data.totals.hitRate)} current={data.totals.hitRate} previous={data.previous.hitRate} note={t("stat.hitRateNote")} />
    <Stat label={t("stat.output")} value={tokens(data.totals.output)} current={data.totals.output} previous={data.previous.output} />
    <Stat label={t("stat.requests")} value={count(data.totals.requests)} current={data.totals.requests} previous={data.previous.requests} />
  </dl>

  <section class="mt-8">
    <h2 class="mb-3 text-md font-semibold">{t("cache.chart")}</h2>
    {#if data.daily.length}<DailyChart days={data.daily} />{:else}<p class="text-sm text-fleece">{t("cache.noData")}</p>{/if}
  </section>

  <section class="mt-10 max-w-3xl">
    <h2 class="mb-1 text-md font-semibold">{t("cache.misses")}</h2>
    <p class="mb-3 text-xs text-fleece">{t("cache.missesNote")}</p>
    <ul class="divide-y divide-warp">
      {#each data.misses as m (m.reason)}
        <li class="grid grid-cols-[minmax(0,1fr)_120px_40px] items-center gap-4 py-3">
          <span>
            <span class="block">{has(`miss.${m.reason}`) ? t(`miss.${m.reason}`) : m.reason}</span>
            <span class="block text-xs text-fleece">{has(`miss.${m.reason}.tip`) ? t(`miss.${m.reason}.tip`) : ""}</span>
          </span>
          <span class="h-1.5 rounded-full bg-loom-2"><span class="block h-full rounded-full bg-cochineal" style="width: {(m.count / maxMiss) * 100}%"></span></span>
          <span class="text-right">{m.count}</span>
        </li>
      {:else}
        <li class="py-3 text-sm text-fleece">{t("cache.noMisses")}</li>
      {/each}
    </ul>
  </section>
{/if}
