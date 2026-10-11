<script lang="ts">
  import { prefs, setDays } from "../lib/prefs.svelte";
  import { t } from "../lib/i18n.svelte";
</script>

<div class="flex rounded-md border border-warp bg-loom p-0.5" role="group" aria-label={t("period.label")}>
  {#each [7, 30, 90] as d (d)}
    <button
      type="button"
      class="h-7 rounded px-2.5 text-sm transition-colors {prefs.days === d ? 'bg-loom-2 text-cotton' : 'text-fleece hover:text-cotton'}"
      aria-pressed={prefs.days === d}
      onclick={() => setDays(d)}>{t("period.days", { count: d })}</button
    >
  {/each}
</div>
