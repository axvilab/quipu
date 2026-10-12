<script lang="ts">
  import { api } from "../lib/api/client";
  import type { Finding } from "../lib/api/types";
  import { findingRow, findingWhere, groupFindings, type FindingGroup } from "../lib/findings";
  import { tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { report } from "../lib/toast.svelte";
  import FindingList from "./FindingList.svelte";
  import Icon from "./Icon.svelte";

  // Findings grouped by rule: what is wrong is said once in the group's title, the rows below
  // only name where. Small groups start open; big ones show their size and stay folded.
  let { findings, onopen, onapplied }: { findings: Finding[]; onopen: (f: Finding) => void; onapplied: () => void } = $props();

  const groups = $derived(groupFindings(findings));
  let toggled = $state<Record<string, boolean>>({});
  let fixing = $state<{ rule: string; done: number; count: number } | null>(null);

  const isOpen = (g: FindingGroup) => toggled[g.rule] ?? g.findings.length <= 3;

  // One by one, so each fix is computed against the files as the previous one left them.
  async function fixAll(g: FindingGroup) {
    fixing = { rule: g.rule, done: 0, count: g.findings.length };
    let fixed = 0;
    for (const f of g.findings) {
      try {
        const res = await api.apply(f.dir, f.id);
        if (res.ok) fixed++;
      } catch (e) {
        report(e);
      }
      fixing.done++;
    }
    fixing = null;
    report({ ok: true, code: "fixed-all", params: { count: String(fixed) } });
    onapplied();
  }
</script>

<div class="divide-y divide-warp border-y border-warp">
  {#each groups as g (g.rule)}
    {@const open = isOpen(g)}
    {#if g.findings.length === 1}
      {@const f = g.findings[0]!}
      <!-- One finding: the group is the finding, so one row opens it. -->
      <button class="flex w-full items-center gap-3 py-3 pl-6 text-left hover:bg-loom focus-visible:bg-loom" onclick={() => onopen(f)}>
        <span class="min-w-0 flex-1">
          <span class="block truncate">
            <span class="mr-2 font-medium">{g.manual ? t("advice.manual") + ": " : ""}{t(`advice.group.${g.rule}`)}</span>
            <span class="text-fleece">{findingRow(f)}</span>
          </span>
          <span class="mt-0.5 block truncate text-xs text-fleece">{findingWhere(f)}. {t(`advice.group.${g.rule}.note`)}</span>
        </span>
        {#if f.saves}<span class="shrink-0 text-sm tabular-nums text-qolle" title={t("finding.savesTitle")}>−{tokens(f.saves)}</span>{/if}
      </button>
    {:else}
    <section class="py-1">
      <div class="flex items-center gap-3 py-2">
        <button class="flex min-w-0 flex-1 items-center gap-2 text-left" aria-expanded={open} onclick={() => (toggled[g.rule] = !open)}>
          <span class="shrink-0 text-fleece"><Icon name={open ? "down" : "right"} size={14} /></span>
          <span class="min-w-0">
            <span class="block font-medium">
              {g.manual ? t("advice.manual") + ": " : ""}{t(`advice.group.${g.rule}`)}
              <span class="ml-1 text-sm font-normal text-fleece">{g.findings.length}</span>
            </span>
            {#if open}<span class="mt-0.5 block text-xs leading-relaxed text-fleece">{t(`advice.group.${g.rule}.note`)}</span>{/if}
          </span>
        </button>
        {#if g.saves}<span class="shrink-0 text-sm tabular-nums text-qolle" title={t("finding.savesTitle")}>−{tokens(g.saves)}</span>{/if}
        {#if g.fixAll}
          <button class="btn h-7 shrink-0 px-2 text-xs" disabled={!!fixing} title={t("advice.fixAllHint")} onclick={() => fixAll(g)}>
            {fixing?.rule === g.rule ? t("advice.fixingAll", { done: fixing.done, count: fixing.count }) : t("advice.fixAll", { count: g.findings.length })}
          </button>
        {/if}
      </div>
      {#if open}
        <div class="pb-2 pl-6"><FindingList findings={g.findings} {onopen} compact /></div>
      {/if}
    </section>
    {/if}
  {/each}
</div>
