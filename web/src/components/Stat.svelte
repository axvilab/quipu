<script lang="ts">
  import Delta from "./Delta.svelte";
  let {
    label,
    value,
    current,
    previous,
    cost = false,
    note = "",
  }: { label: string; value: string; current?: number; previous?: number; cost?: boolean; note?: string } = $props();
</script>

<div class="min-w-0">
  <dt class="text-xs text-fleece">{label}</dt>
  <dd class="mt-1 flex items-baseline gap-2">
    <span class="text-lg font-semibold">{value}</span>
    {#if current !== undefined && previous !== undefined}<Delta {current} {previous} {cost} />{/if}
  </dd>
  {#if note}<p class="mt-0.5 truncate text-xs text-fleece">{note}</p>{/if}
</div>
