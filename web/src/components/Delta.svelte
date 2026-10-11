<script lang="ts">
  import { change, count, percent } from "../lib/format";
  import { t } from "../lib/i18n.svelte";

  // Shows the change against the previous period. For costs, going up is bad (cost = true).
  // Growth of 10× and more reads better as a multiple than as "+7 075%".
  let { current, previous, cost = false }: { current: number; previous: number; cost?: boolean } = $props();

  const d = $derived(change(current, previous));
  const worse = $derived(d !== null && (cost ? d > 0.05 : d < -0.05));
  const better = $derived(d !== null && (cost ? d < -0.05 : d > 0.05));
</script>

{#if d !== null}
  <span
    class="text-xs {worse ? 'text-cochineal' : better ? 'text-chilca' : 'text-fleece'}"
    title={t("delta.title")}
  >
    {#if d >= 9}×{count(Math.round(d + 1))}{:else}{d > 0 ? "+" : ""}{percent(d, 0)}{/if}
  </span>
{/if}
