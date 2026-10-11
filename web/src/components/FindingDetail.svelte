<script lang="ts">
  import type { Finding } from "../lib/api/types";
  import { api } from "../lib/api/client";
  import { report } from "../lib/toast.svelte";
  import { openEditor } from "../lib/editor.svelte";
  import { findingDetail, findingTitle, findingWhere } from "../lib/findings";
  import { tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import Diff from "./Diff.svelte";
  import Icon from "./Icon.svelte";

  let { finding, onapplied }: { finding: Finding; onapplied: () => void } = $props();
  let busy = $state(false);

  async function apply() {
    busy = true;
    try {
      const res = await api.apply(finding.dir, finding.id);
      const change = res.params?.change;
      report(res, change ? { label: t("action.undo"), run: () => undo(change) } : undefined);
      onapplied();
    } catch (e) {
      report(e);
    } finally {
      busy = false;
    }
  }

  // Undo from the toast: the panel is closed by then, so refresh through onapplied again.
  async function undo(change: string) {
    try {
      report(await api.undo(change));
    } catch (e) {
      report(e);
    }
    onapplied();
  }
</script>

<div class="flex flex-col gap-5">
  <div>
    <p class="text-sm text-fleece">{findingWhere(finding)}</p>
    <h3 class="mt-1 text-lg font-semibold leading-snug text-balance">{findingTitle(finding)}</h3>
    {#if finding.saves}
      <p class="mt-2 text-sm text-qolle">{t("finding.saves", { tokens: tokens(finding.saves) })}</p>
    {/if}
  </div>

  {#if findingDetail(finding)}
    <p class="max-w-prose leading-relaxed text-cotton/90">{findingDetail(finding)}</p>
  {/if}

  {#if finding.path}
    <div>
      <p class="mb-1 text-xs text-fleece">{t("finding.file")}</p>
      <p class="font-mono text-[12.5px] break-all">{finding.path}</p>
    </div>
  {/if}

  {#if finding.diff?.length}
    <div>
      <p class="mb-1 text-xs text-fleece">{t("finding.change")}</p>
      <Diff lines={finding.diff} />
    </div>
  {/if}

  {#if finding.projects.length > 1}
    <details class="text-sm">
      <summary class="cursor-pointer text-fleece">{t("finding.projectsList", { count: finding.projects.length })}</summary>
      <ul class="mt-2 space-y-1 font-mono text-[12.5px] text-fleece">
        {#each finding.projects as p (p)}<li>{p}</li>{/each}
      </ul>
    </details>
  {/if}

  <div class="flex items-center gap-3 border-t border-warp pt-4">
    {#if finding.canApply}
      <button class="btn btn-primary" onclick={apply} disabled={busy} data-autofocus>
        <Icon name="check" />{t("action.apply")}
      </button>
      <span class="text-xs text-fleece">{t("finding.applyNote")}</span>
    {:else}
      <span class="text-sm text-fleece">{t("finding.manual")}</span>
    {/if}
    {#if finding.path}
      {@const path = finding.path}
      <button class="btn {finding.canApply ? 'btn-ghost' : ''} ml-auto" onclick={() => openEditor(path)} data-autofocus={finding.canApply ? undefined : true}>
        <Icon name="edit" size={14} />{t("editor.edit")}
      </button>
    {/if}
  </div>
</div>
