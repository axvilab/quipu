<script lang="ts">
  import type { Snippet } from "svelte";
  import { fly, fade } from "svelte/transition";
  import Icon from "./Icon.svelte";
  import { t } from "../lib/i18n.svelte";

  // A sheet that slides in from the right, like Linear's detail view. Esc or the backdrop closes
  // it; with sheets stacked (the editor over a finding) Esc closes only the top one.
  let { title, onclose, children, wide = false }: { title: string; onclose: () => void; children: Snippet; wide?: boolean } = $props();

  function onkeydown(e: KeyboardEvent) {
    if (e.key !== "Escape" || !sheet) return;
    const sheets = document.querySelectorAll("[data-sheet]");
    if (sheets[sheets.length - 1] === sheet) onclose();
  }

  let sheet: HTMLElement | undefined = $state();
  $effect(() => {
    sheet?.querySelector<HTMLElement>("[data-autofocus], button, a, input, select")?.focus();
  });
</script>

<svelte:window {onkeydown} />

<div class="fixed inset-0 z-30 bg-night/60" transition:fade={{ duration: 120 }} onclick={onclose} aria-hidden="true"></div>
<div
  bind:this={sheet}
  data-sheet
  class="fixed inset-y-0 right-0 z-40 flex w-full flex-col border-l border-warp bg-loom shadow-2xl {wide ? 'max-w-[920px]' : 'max-w-[560px]'}"
  transition:fly={{ x: 40, duration: 180 }}
  role="dialog"
  aria-modal="true"
  aria-label={title}
>
  <header class="flex h-12 shrink-0 items-center justify-between border-b border-warp px-5">
    <h2 class="truncate text-md font-semibold">{title}</h2>
    <button class="btn btn-ghost h-8 w-8 justify-center px-0" onclick={onclose} aria-label={t("action.close")}>
      <Icon name="close" />
    </button>
  </header>
  <div class="flex-1 overflow-y-auto px-5 py-5">
    {@render children()}
  </div>
</div>
