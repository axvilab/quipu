<script lang="ts">
  import type { Collection } from "../lib/api/types";
  import { count } from "../lib/format";
  import { t } from "../lib/i18n.svelte";

  // Shown instead of the screens until the first collection has stored something.
  let { state }: { state: Collection } = $props();
  const share = $derived(state.total ? state.done / state.total : 0);
</script>

<section class="mx-auto mt-[12vh] max-w-md text-center" aria-live="polite">
  <img src="/static/icon.svg" alt="" width="48" height="48" class="mx-auto mb-5 rounded-lg" />
  <h1 class="text-lg font-semibold">{t("collect.title")}</h1>
  <p class="mt-2 text-sm leading-relaxed text-fleece">{t("collect.body")}</p>
  <div class="mx-auto mt-6 h-1.5 max-w-xs overflow-hidden rounded-full bg-loom" role="progressbar" aria-valuemin={0} aria-valuemax={state.total} aria-valuenow={state.done}>
    {#if state.total}
      <div class="h-full rounded-full bg-indigo transition-[width] duration-300 motion-reduce:transition-none" style="width: {share * 100}%"></div>
    {:else}
      <div class="h-full w-1/3 animate-pulse rounded-full bg-indigo motion-reduce:animate-none"></div>
    {/if}
  </div>
  <p class="mt-3 text-xs tabular-nums text-fleece">
    {state.total ? t("collect.progress", { done: count(state.done), total: count(state.total) }) : t("collect.finding")}
  </p>
</section>
