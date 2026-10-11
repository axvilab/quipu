<script lang="ts">
  import type { Snippet } from "svelte";
  let { title, subtitle = "", actions }: { title: string; subtitle?: string; actions?: Snippet } = $props();
</script>

<header class="flex flex-wrap items-end justify-between gap-4 pb-6">
  <div class="min-w-0">
    <h1 class="truncate text-xl font-semibold tracking-tight">{title}</h1>
    {#if subtitle}<p class="mt-1 truncate text-sm text-fleece">{subtitle}</p>{/if}
  </div>
  {#if actions}<div class="flex items-center gap-2">{@render actions()}</div>{/if}
</header>
