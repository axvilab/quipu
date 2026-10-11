<script lang="ts">
  import { api } from "../lib/api/client";
  import type { Finding, Overview } from "../lib/api/types";
  import { percent, tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { onChanged } from "../lib/changes.svelte";
  import { prefs } from "../lib/prefs.svelte";
  import { link, projectPath } from "../lib/router.svelte";
  import FindingDetail from "../components/FindingDetail.svelte";
  import FindingList from "../components/FindingList.svelte";
  import Header from "../components/Header.svelte";
  import Khipu from "../components/Khipu.svelte";
  import Loading from "../components/Loading.svelte";
  import Panel from "../components/Panel.svelte";
  import PeriodSwitch from "../components/PeriodSwitch.svelte";
  import Stat from "../components/Stat.svelte";

  let data = $state<Overview | null>(null);
  let error = $state<unknown>(null);
  let open = $state<Finding | null>(null);

  async function load() {
    try {
      data = await api.overview(prefs.days);
      error = null;
    } catch (e) {
      error = e;
    }
  }
  $effect(() => {
    void prefs.days;
    load();
  });
  onChanged(load);

  const maxWrites = $derived(Math.max(1, ...(data?.projects.map((p) => p.totals.cacheWrite) ?? [1])));
</script>

<Header title={t("overview.title")} subtitle={t("overview.subtitle", { count: prefs.days })}>
  {#snippet actions()}<PeriodSwitch />{/snippet}
</Header>

{#if !data}
  <Loading {error} />
{:else if data.projects.length === 0}
  <div class="rounded-lg border border-warp px-6 py-10 text-center">
    <p class="text-md">{t("overview.empty.title")}</p>
    <p class="mt-2 text-sm text-fleece">{t("overview.empty.body")}</p>
  </div>
{:else}
  <section aria-labelledby="khipu-h">
    <h2 id="khipu-h" class="sr-only">{t("khipu.label")}</h2>
    <Khipu projects={data.projects} />
  </section>

  <dl class="mt-6 grid grid-cols-2 gap-6 border-y border-warp py-5 md:grid-cols-4">
    <Stat label={t("stat.cacheWrites")} value={tokens(data.totals.cacheWrite)} current={data.totals.cacheWrite} previous={data.previous.cacheWrite} cost note={t("stat.cacheWritesNote")} />
    <Stat label={t("stat.prompt")} value={tokens(data.totals.prompt)} current={data.totals.prompt} previous={data.previous.prompt} cost note={t("stat.hitNote", { rate: percent(data.totals.hitRate) })} />
    <Stat label={t("stat.sessions")} value={String(data.totals.sessions)} current={data.totals.sessions} previous={data.previous.sessions} />
    <Stat label={t("stat.misses")} value={String(data.totals.misses)} current={data.totals.misses} previous={data.previous.misses} cost note={t("stat.missesNote")} />
  </dl>

  <section class="mt-8" aria-labelledby="attention-h">
    <div class="mb-2 flex items-baseline justify-between">
      <h2 id="attention-h" class="text-md font-semibold">{t("overview.attention")}</h2>
      <a href="/advice" use:link class="text-sm text-indigo hover:underline">{t("overview.allAdvice")}</a>
    </div>
    {#if data.attention.length}
      <FindingList findings={data.attention.slice(0, 6)} onopen={(f) => (open = f)} />
    {:else}
      <p class="rounded-md border border-warp px-4 py-4 text-sm text-fleece">{t("overview.allGood")}</p>
    {/if}
  </section>

  <section class="mt-10" aria-labelledby="projects-h">
    <h2 id="projects-h" class="mb-2 text-md font-semibold">{t("overview.projects")}</h2>
    <div class="overflow-x-auto">
      <table class="w-full min-w-[640px] text-sm">
        <thead class="text-left text-xs text-fleece">
          <tr>
            <th class="py-2 pr-4 font-normal">{t("col.project")}</th>
            <th class="py-2 pr-4 font-normal">{t("col.cacheWrites")}</th>
            <th class="py-2 pr-4 text-right font-normal">{t("col.sessionStart")}</th>
            <th class="py-2 pr-4 text-right font-normal">{t("col.sessions")}</th>
            <th class="py-2 text-right font-normal">{t("col.findings")}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-warp">
          {#each data.projects as p (p.dir)}
            <tr class="group">
              <td class="py-2.5 pr-4">
                <a href={projectPath(p.dir)} use:link class="block font-medium group-hover:text-indigo">{p.name}</a>
                <span class="block truncate font-mono text-xs text-fleece">{p.path}</span>
              </td>
              <td class="w-[30%] py-2.5 pr-4">
                <div class="flex items-center gap-3">
                  <div class="h-1.5 flex-1 rounded-full bg-loom-2"><div class="h-full rounded-full bg-qolle" style="width: {(p.totals.cacheWrite / maxWrites) * 100}%"></div></div>
                  <span class="w-14 text-right">{tokens(p.totals.cacheWrite)}</span>
                </div>
              </td>
              <td class="py-2.5 pr-4 text-right">{p.start.sessions ? tokens(p.start.latest) : "–"}</td>
              <td class="py-2.5 pr-4 text-right">{p.totals.sessions}</td>
              <td class="py-2.5 text-right">{#if p.findings}<span class="text-cochineal">{p.findings}</span>{:else}<span class="text-fleece">0</span>{/if}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </section>

  <p class="mt-8 text-xs text-fleece">{t("overview.partial")}</p>
{/if}

{#if open}
  <Panel title={t("finding.panelTitle")} onclose={() => (open = null)}>
    <FindingDetail finding={open} onapplied={() => { open = null; load(); }} />
  </Panel>
{/if}
