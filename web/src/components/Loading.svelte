<script lang="ts">
  import { t } from "../lib/i18n.svelte";
  import { ApiError } from "../lib/api/client";
  let { error = null }: { error?: unknown } = $props();
</script>

{#if error}
  <div class="rounded-lg border border-cochineal/40 bg-cochineal/10 px-4 py-3 text-sm" role="alert">
    {error instanceof ApiError ? t(`result.${error.code}`, error.params) : String(error)}
  </div>
{:else}
  <div class="space-y-3" aria-busy="true" aria-label={t("loading")}>
    {#each [0, 1, 2] as i (i)}<div class="h-10 animate-pulse rounded-md bg-loom"></div>{/each}
  </div>
{/if}
