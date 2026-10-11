<script lang="ts">
  import type { Snippet } from "svelte";
  import { fly, fade } from "svelte/transition";
  import Icon from "./Icon.svelte";
  import { t } from "../lib/i18n.svelte";

  // A sheet that slides in from the right, like Linear's detail view. Esc or the backdrop closes it.
  let { title, onclose, children }: { title: string; onclose: () => void; children: Snippet } = $props();

  let sheet: HTMLElement | undefined = $state();
  $effect(() => {
    sheet?.querySelector<HTMLElement>("[data-autofocus], button, a, input, select")?.focus();
  });
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && onclose()} />

<div class="fixed inset-0 z-30 bg-night/60" transition:fade={{ duration: 120 }} onclick={onclose} aria-hidden="true"></div>
<div
  bind:this={sheet}
  class="fixed inset-y-0 right-0 z-40 flex w-full max-w-[560px] flex-col border-l border-warp bg-loom shadow-2xl"
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
