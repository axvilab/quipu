<script lang="ts">
  import { t } from "../lib/i18n.svelte";
  import { palette } from "../lib/palette.svelte";
  import Panel from "./Panel.svelte";

  const keys = [
    ["Ctrl K", "help.palette"],
    ["G O", "help.overview"],
    ["G A", "help.advice"],
    ["G C", "help.cache"],
    ["G R", "help.profiles"],
    ["G N", "help.new"],
    ["G B", "help.backup"],
    ["G S", "help.settings"],
    ["J / K", "help.move"],
    ["Enter", "help.openItem"],
    ["Esc", "help.close"],
    ["?", "help.this"],
  ];
</script>

{#if palette.help}
  <Panel title={t("help.title")} onclose={() => (palette.help = false)}>
    <dl class="grid grid-cols-[auto_1fr] items-center gap-x-6 gap-y-3 text-sm">
      {#each keys as [k, label] (k)}
        <dt><span class="kbd">{k}</span></dt>
        <dd>{t(label!)}</dd>
      {/each}
    </dl>
  </Panel>
{/if}
