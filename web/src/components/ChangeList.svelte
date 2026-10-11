<script lang="ts">
  import { api } from "../lib/api/client";
  import type { Change } from "../lib/api/types";
  import { notifyChanged } from "../lib/changes.svelte";
  import { changeTitle } from "../lib/findings";
  import { dateTime } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { report } from "../lib/toast.svelte";
  import Icon from "./Icon.svelte";

  // quipu's changes, one row per action, each with Undo (all files of the action at once).
  let { changes }: { changes: Change[] } = $props();

  async function undo(id: string) {
    try {
      report(await api.undo(id));
    } catch (e) {
      report(e);
    }
    notifyChanged();
  }
</script>

<ul class="divide-y divide-warp">
  {#each changes as c (c.id)}
    <li class="flex items-center gap-4 py-2.5 text-sm">
      <span class="h-2 w-2 shrink-0 rounded-full {c.undone ? 'bg-warp' : 'bg-chilca'}" aria-hidden="true"></span>
      <span class="min-w-0 flex-1">
        <span class="block truncate">{changeTitle(c)}</span>
        {#if c.files.length > 1}
          <span class="block truncate text-xs text-fleece" title={c.files.join("\n")}>{t("advice.files", { count: c.files.length })}</span>
        {:else}
          <span class="block truncate font-mono text-xs text-fleece">{c.path}</span>
        {/if}
      </span>
      <span class="shrink-0 text-xs text-fleece">{dateTime(c.time)}</span>
      {#if c.undone}
        <span class="w-24 shrink-0 text-right text-xs text-fleece">{t("advice.undone")}</span>
      {:else}
        <button class="btn w-24 shrink-0 justify-center" onclick={() => undo(c.id)}><Icon name="undo" size={14} />{t("action.undo")}</button>
      {/if}
    </li>
  {/each}
</ul>
