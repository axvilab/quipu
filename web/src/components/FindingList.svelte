<script lang="ts">
  import type { Finding } from "../lib/api/types";
  import { findingRow, findingTitle, findingWhere } from "../lib/findings";
  import { tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";

  // A list of findings; j/k or arrows move, Enter opens. Inside a rule's group (compact) a row
  // shows only what differs, since the group's title already says what is wrong.
  let { findings, onopen, compact = false }: { findings: Finding[]; onopen: (f: Finding) => void; compact?: boolean } = $props();

  function onkeydown(e: KeyboardEvent, i: number) {
    const rows = (e.currentTarget as HTMLElement).parentElement?.parentElement?.querySelectorAll<HTMLElement>("button[data-row]");
    if (!rows) return;
    if (e.key === "ArrowDown" || e.key === "j") rows[Math.min(i + 1, rows.length - 1)]?.focus();
    else if (e.key === "ArrowUp" || e.key === "k") rows[Math.max(i - 1, 0)]?.focus();
    else return;
    e.preventDefault();
  }
</script>

<ul class="divide-y divide-warp">
  {#each findings as f, i (f.id + f.dir)}
    <li>
      <button
        data-row
        class="group flex w-full items-center gap-4 px-3 py-3 text-left transition-colors hover:bg-loom focus-visible:bg-loom"
        onclick={() => onopen(f)}
        onkeydown={(e) => onkeydown(e, i)}
      >
        {#if !compact}<span class="h-2 w-2 shrink-0 rounded-full bg-cochineal" aria-hidden="true"></span>{/if}
        <span class="min-w-0 flex-1">
          <span class="block truncate">{compact ? findingRow(f) : findingTitle(f)}</span>
          <span class="mt-0.5 block truncate text-xs text-fleece">{findingWhere(f)}</span>
        </span>
        {#if f.saves}
          <span class="shrink-0 text-sm text-qolle" title={t("finding.savesTitle")}>−{tokens(f.saves)}</span>
        {/if}
        {#if !compact}<span class="shrink-0 text-xs text-fleece">{f.canApply ? t("finding.oneClick") : t("finding.manualShort")}</span>{/if}
      </button>
    </li>
  {/each}
</ul>
