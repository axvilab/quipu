<script lang="ts">
  import { tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";

  // What the first request of a session is made of: Claude Code itself (system prompt, tools,
  // MCP) versus the files you control. On-demand files are listed separately.
  let { latest, files, onDemand }: { latest: number; files: number; onDemand: number } = $props();

  const own = $derived(Math.max(latest - files, 0));
  const total = $derived(Math.max(latest, files, 1));
</script>

<div>
  <div class="flex h-3 overflow-hidden rounded-full bg-loom-2" role="img"
    aria-label={t("context.aria", { claude: tokens(own), files: tokens(files) })}>
    <div class="bg-fleece/45" style="width: {(own / total) * 100}%"></div>
    <div class="bg-qolle" style="width: {(files / total) * 100}%"></div>
  </div>
  <dl class="mt-3 flex flex-wrap gap-x-6 gap-y-1 text-sm">
    <div class="flex items-center gap-2"><span class="h-2 w-2 rounded-full bg-fleece/45"></span><dt class="text-fleece">{t("context.claude")}</dt><dd>{latest ? tokens(own) : "–"}</dd></div>
    <div class="flex items-center gap-2"><span class="h-2 w-2 rounded-full bg-qolle"></span><dt class="text-fleece">{t("context.files")}</dt><dd>≈ {tokens(files)}</dd></div>
    <div class="flex items-center gap-2"><span class="h-2 w-2 rounded-full border border-dashed border-fleece"></span><dt class="text-fleece">{t("context.onDemand")}</dt><dd>≈ {tokens(onDemand)}</dd></div>
  </dl>
</div>
