<script lang="ts">
  import { live } from "../lib/live.svelte";
  import { t } from "../lib/i18n.svelte";
  import { dateTime } from "../lib/format";

  // Shown on every screen while a session can be driven from claude.ai or the phone.
  const on = $derived(live.data?.sessions.filter((s) => s.remoteControl === "on") ?? []);
</script>

{#if on.length}
  <div class="mb-6 rounded-lg border border-cochineal/50 bg-cochineal/10 px-4 py-3 text-sm" role="alert">
    <p class="font-semibold">{t("live.banner.title", { count: on.length })}</p>
    <ul class="mt-1 space-y-0.5">
      {#each on as s (s.pid)}
        <li class="flex flex-wrap items-baseline gap-x-3">
          <span class="font-medium">{s.name}</span>
          <span class="font-mono text-xs text-fleece">{s.path}</span>
          {#if s.started}<span class="text-xs text-fleece">{t("live.since", { time: dateTime(s.started) })}</span>{/if}
        </li>
      {/each}
    </ul>
    <p class="mt-1.5 text-xs text-cotton/80">{t("live.banner.howOff")}</p>
  </div>
{/if}
