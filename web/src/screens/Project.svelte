<script lang="ts">
  import { api } from "../lib/api/client";
  import type { Finding, Project } from "../lib/api/types";
  import { contextGroups, type ContextGroup } from "../lib/context";
  import { findingTitle } from "../lib/findings";
  import { dateTime, percent, tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { prefs } from "../lib/prefs.svelte";
  import { onChanged } from "../lib/changes.svelte";
  import { openEditor } from "../lib/editor.svelte";
  import ChangeList from "../components/ChangeList.svelte";
  import ContextBar from "../components/ContextBar.svelte";
  import DailyChart from "../components/DailyChart.svelte";
  import FindingDetail from "../components/FindingDetail.svelte";
  import FindingGroups from "../components/FindingGroups.svelte";
  import Header from "../components/Header.svelte";
  import Icon from "../components/Icon.svelte";
  import Loading from "../components/Loading.svelte";
  import Panel from "../components/Panel.svelte";
  import PeriodSwitch from "../components/PeriodSwitch.svelte";
  import Stat from "../components/Stat.svelte";

  let { dir }: { dir: string } = $props();
  let data = $state<Project | null>(null);
  let error = $state<unknown>(null);
  let tab = $state<"context" | "advice" | "changes" | "activity">("context");
  let open = $state<Finding | null>(null);
  let toggled = $state<Record<string, boolean>>({});

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

  const groups = $derived(data ? contextGroups(data.items, data.plugins, data.findings) : []);
  const maxGroup = $derived(Math.max(1, ...groups.map((g) => g.always)));
  // The project's own files start open, and so does any group with something to fix.
  const isOpen = (g: ContextGroup) => toggled[g.id] ?? (g.id === "project" || g.findings.length > 0);
  const tabs = $derived([
    { id: "context" as const, label: t("project.tab.context") },
    { id: "advice" as const, label: t("project.tab.advice", { count: data?.findings.length ?? 0 }) },
    { id: "changes" as const, label: t("project.tab.changes", { count: data?.changes.length ?? 0 }) },
    { id: "activity" as const, label: t("project.tab.activity") },
  ]);
</script>

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
      {#if !groups.length}
        <p class="text-sm text-fleece">{t("context.empty")}</p>
      {/if}
      <div class="divide-y divide-warp border-y border-warp">
        {#each groups as g (g.id)}
          {@const expanded = isOpen(g)}
          <section>
            <button class="grid w-full grid-cols-[1rem_minmax(0,1fr)_minmax(4rem,10rem)_4.5rem] items-center gap-3 py-3 text-left" aria-expanded={expanded}
              onclick={() => (toggled[g.id] = !expanded)}>
              <span class="text-fleece"><Icon name={expanded ? "down" : "right"} size={14} /></span>
              <span class="min-w-0 truncate">
                <span class="font-medium">{t(`context.group.${g.id}`)}</span>
                <span class="ml-2 text-xs text-fleece">{t("context.items", { count: g.rows.length })}</span>
                {#if g.findings.length}<span class="ml-2 inline-block h-2 w-2 rounded-full bg-cochineal" title={t("context.finding")}></span>{/if}
              </span>
              <span class="h-1.5 rounded-full bg-loom-2"><span class="block h-full rounded-full bg-qolle" style="width: {(g.always / maxGroup) * 100}%"></span></span>
              <span class="text-right tabular-nums">{tokens(g.always)}</span>
            </button>
            {#if expanded}
              <ul class="mb-3 ml-7 divide-y divide-warp/60">
                {#each g.rows as r (r.key)}
                  <li class="grid grid-cols-[minmax(0,1fr)_auto_4.5rem] items-center gap-3 py-2 text-sm">
                    <span class="min-w-0">
                      <span class="flex min-w-0 items-center gap-2">
                        {#if r.editable}
                          <button class="truncate text-left hover:text-indigo" title={r.path} onclick={() => openEditor(r.path)}>{r.label}</button>
                        {:else}
                          <span class="truncate" title={r.path}>{r.label}</span>
                        {/if}
                        {#if r.kind !== "plugin" && r.kind !== "CLAUDE.md" && r.kind !== "memory"}<span class="shrink-0 text-xs text-fleece">{t(`kind.${r.kind}`)}</span>{/if}
                        {#if r.findings.length}
                          {@const f = r.findings[0]!}
                          <button class="flex min-w-0 items-center gap-1 text-xs text-cochineal hover:text-cotton" title={findingTitle(f)} onclick={() => (open = f)}>
                            <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-cochineal" aria-hidden="true"></span>
                            <span class="truncate">{t(`advice.group.${f.rule}`)}{r.findings.length > 1 ? ` +${r.findings.length - 1}` : ""}</span>
                          </button>
                        {/if}
                      </span>
                      {#if r.plugin}
                        <span class="block truncate text-xs {r.plugin.uses ? 'text-fleece' : 'text-cochineal'}">
                          {r.plugin.uses && r.plugin.last ? t("context.pluginUsed", { time: dateTime(r.plugin.last) }) : t("context.pluginUnused")}{#if r.plugin.skills.length}<span class="text-fleece">; {t("context.pluginSkills", { names: r.plugin.skills.join(", ") })}</span>{/if}
                        </span>
                      {/if}
                    </span>
                    <span class="text-right text-xs text-fleece">
                      {#if r.onUse}{r.always ? t("context.onUse", { tokens: tokens(r.onUse) }) : t("context.onUseOnly", { tokens: tokens(r.onUse) })}{/if}
                    </span>
                    <span class="text-right tabular-nums {r.always ? '' : 'text-fleece'}">{r.always ? tokens(r.always) : "–"}</span>
                  </li>
                {/each}
                {#each g.findings.filter((f) => !g.rows.some((r) => r.findings.includes(f))) as f (f.id)}
                  <li class="py-2 text-sm">
                    <button class="flex items-center gap-1.5 text-left text-xs text-cochineal hover:text-cotton" onclick={() => (open = f)}>
                      <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-cochineal" aria-hidden="true"></span>{findingTitle(f)}
                    </button>
                  </li>
                {/each}
              </ul>
            {/if}
          </section>
        {/each}
      </div>
    {:else if tab === "advice"}
      {#if data.findings.length}
        <FindingGroups findings={data.findings} onopen={(f) => (open = f)} onapplied={load} />
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
