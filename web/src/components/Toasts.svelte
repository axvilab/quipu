<script lang="ts">
  import { fly } from "svelte/transition";
  import { dismiss, toasts } from "../lib/toast.svelte";
  import Icon from "./Icon.svelte";
  import { t } from "../lib/i18n.svelte";
</script>

<div class="fixed right-4 bottom-4 z-[60] flex w-[min(420px,calc(100vw-32px))] flex-col gap-2" role="status" aria-live="polite">
  {#each toasts.list as x (x.id)}
    <div transition:fly={{ y: 12, duration: 160 }}
      class="flex items-start gap-3 rounded-lg border bg-loom px-4 py-3 text-sm shadow-xl {x.kind === 'ok' ? 'border-chilca/40' : 'border-cochineal/50'}">
      <span class="mt-0.5 h-2 w-2 shrink-0 rounded-full {x.kind === 'ok' ? 'bg-chilca' : 'bg-cochineal'}" aria-hidden="true"></span>
      <p class="flex-1 leading-snug">{x.text}</p>
      <button class="text-fleece hover:text-cotton" onclick={() => dismiss(x.id)} aria-label={t("action.close")}><Icon name="close" size={14} /></button>
    </div>
  {/each}
</div>
