<script lang="ts">
  import { api } from "../lib/api/client";
  import type { Finding, Item, Project } from "../lib/api/types";
  import { dateTime, percent, tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { prefs } from "../lib/prefs.svelte";
  import { onChanged } from "../lib/changes.svelte";
  import { openEditor } from "../lib/editor.svelte";
  import ChangeList from "../components/ChangeList.svelte";
  import ContextBar from "../components/ContextBar.svelte";
  import DailyChart from "../components/DailyChart.svelte";
  import FindingDetail from "../components/FindingDetail.svelte";
  import FindingList from "../components/FindingList.svelte";
  import Header from "../components/Header.svelte";
  import Loading from "../components/Loading.svelte";
  import Panel from "../components/Panel.svelte";
  import PeriodSwitch from "../components/PeriodSwitch.svelte";
  import Stat from "../components/Stat.svelte";

  let { dir }: { dir: string } = $props();
  let data = $state<Project | null>(null);
  let error = $state<unknown>(null);
  let tab = $state<"context" | "advice" | "changes" | "activity">("context");
  let open = $state<Finding | null>(null);
  let showAll = $state(false);

  async function load() {
    try {
      data = await api.project(dir, prefs.days);
      error = null;
    } catch (e) {
      error = e;
    }
  }
  $effect(() => {
    void prefs.days;
    void dir;
    load();
  });

  onChanged(load);

  $effect(() => {
    if (data) document.title = `${data.name} · Quipu`;
  });

  const always = $derived(data?.items.filter((i) => i.always) ?? []);
  const onDemand = $derived(data?.items.filter((i) => !i.always) ?? []);
  const maxTokens = $derived(Math.max(1, ...(data?.items.map((i) => i.tokens) ?? [1])));
  const tabs = $derived([
    { id: "context" as const, label: t("project.tab.context") },
    { id: "advice" as const, label: t("project.tab.advice", { count: data?.findings.length ?? 0 }) },
    { id: "changes" as const, label: t("project.tab.changes", { count: data?.changes.length ?? 0 }) },
    { id: "activity" as const, label: t("project.tab.activity") },
  ]);
</script>

{#snippet itemRows(items: Item[], limit: number)}
  <ul class="divide-y divide-warp">
    {#each items.slice(0, limit) as it (it.path + it.kind + it.always)}
      <li class="grid grid-cols-[minmax(0,1fr)_auto_100px_56px] items-center gap-4 py-2 text-sm">
        {#if it.editable}
          <button class="group min-w-0 text-left" onclick={() => openEditor(it.path)} title={t("editor.open")}>
            <span class="block truncate group-hover:text-indigo">{it.label}</span>
            <span class="block truncate font-mono text-xs text-fleece">{it.path}</span>
          </button>
        {:else}
          <span class="min-w-0">
            <span class="block truncate">{it.label}</span>
            <span class="block truncate font-mono text-xs text-fleece" title={it.path}>{it.path}</span>
          </span>
        {/if}
        <span class="text-xs text-fleece">{t(`scope.${it.scope.split(" ")[0]}`)} · {t(`kind.${it.kind}`)}</span>
        <span class="h-1.5 rounded-full bg-loom-2"><span class="block h-full rounded-full {it.always ? 'bg-qolle' : 'bg-fleece/50'}" style="width: {(it.tokens / maxTokens) * 100}%"></span></span>
        <span class="text-right">{tokens(it.tokens)}</span>
      </li>
    {/each}
  </ul>
{/snippet}

{#if !data}
  <Loading {error} />
{:else}
  <Header title={data.name} subtitle={data.path}>
    {#snippet actions()}<PeriodSwitch />{/snippet}
  </Header>

  <dl class="grid grid-cols-2 gap-6 border-y border-warp py-5 md:grid-cols-4">
    <Stat label={t("stat.sessionStart")} value={data.start.sessions ? tokens(data.start.latest) : "–"} note={data.start.sessions ? t("stat.startNote", { median: tokens(data.start.median), count: data.start.sessions }) : t("stat.noSessions")} />
    <Stat label={t("stat.cacheWrites")} value={tokens(data.totals.cacheWrite)} current={data.totals.cacheWrite} previous={data.previous.cacheWrite} cost />
    <Stat label={t("stat.prompt")} value={tokens(data.totals.prompt)} current={data.totals.prompt} previous={data.previous.prompt} cost note={t("stat.hitNote", { rate: percent(data.totals.hitRate) })} />
    <Stat label={t("stat.sessions")} value={String(data.totals.sessions)} current={data.totals.sessions} previous={data.previous.sessions} />
  </dl>

  <section class="mt-6">
    <h2 class="mb-3 text-md font-semibold">{t("project.startTitle")}</h2>
    <ContextBar latest={data.start.latest} files={data.alwaysTotal} onDemand={data.onDemandTotal} />
  </section>

  <div class="mt-8 flex gap-1 border-b border-warp" role="tablist">
    {#each tabs as tb (tb.id)}
      <button role="tab" aria-selected={tab === tb.id}
        class="-mb-px border-b-2 px-3 py-2 text-sm {tab === tb.id ? 'border-indigo text-cotton' : 'border-transparent text-fleece hover:text-cotton'}"
        onclick={() => (tab = tb.id)}>{tb.label}</button>
    {/each}
  </div>

  <div class="pt-5" role="tabpanel">
    {#if tab === "context"}
      {#if data.plugins.length}
        <h3 class="mb-1 text-sm font-semibold">{t("project.plugins")}</h3>
        <p class="mb-2 text-xs text-fleece">{t("project.pluginsNote")}</p>
        <ul class="mb-8 divide-y divide-warp">
          {#each data.plugins as p (p.name)}
            <li class="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-4 py-2 text-sm">
              <span class="min-w-0">
                <span class="block truncate">{p.name}</span>
                <span class="block text-xs text-fleece">{t(`project.pluginScope.${p.scope}`)}</span>
              </span>
              <span class="text-xs {p.uses ? 'text-chilca' : 'text-cochineal'}">
                {p.uses && p.last ? t("project.pluginUsed", { time: dateTime(p.last) }) : t("project.pluginUnused")}
              </span>
              <span class="w-20 text-right">{t("project.pluginTokens", { tokens: tokens(p.tokens) })}</span>
            </li>
          {/each}
        </ul>
      {/if}
      <h3 class="mb-1 text-sm font-semibold">{t("project.always", { tokens: tokens(data.alwaysTotal) })}</h3>
      <p class="mb-2 text-xs text-fleece">{t("project.alwaysNote")}</p>
      {@render itemRows(always, always.length)}
      <h3 class="mt-8 mb-1 text-sm font-semibold">{t("project.onDemand", { tokens: tokens(data.onDemandTotal) })}</h3>
      <p class="mb-2 text-xs text-fleece">{t("project.onDemandNote")}</p>
      {@render itemRows(onDemand, showAll ? onDemand.length : 12)}
      {#if onDemand.length > 12 && !showAll}
        <button class="btn btn-ghost mt-2" onclick={() => (showAll = true)}>{t("action.showAll", { count: onDemand.length })}</button>
      {/if}
    {:else if tab === "advice"}
      {#if data.findings.length}
        <FindingList findings={data.findings} onopen={(f) => (open = f)} />
      {:else}
        <p class="text-sm text-fleece">{t("project.noFindings")}</p>
      {/if}
    {:else if tab === "changes"}
      {#if data.changes.length}
        <ChangeList changes={data.changes} />
      {:else}
        <p class="text-sm text-fleece">{t("project.noChanges")}</p>
      {/if}
    {:else}
      <DailyChart days={data.daily} />
    {/if}
  </div>
{/if}

{#if open}
  <Panel title={t("finding.panelTitle")} onclose={() => (open = null)}>
    <FindingDetail finding={open} onapplied={() => { open = null; load(); }} />
  </Panel>
{/if}
