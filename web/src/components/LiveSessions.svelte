<script lang="ts">
  import { live } from "../lib/live.svelte";
  import { t } from "../lib/i18n.svelte";
  import { dateTime } from "../lib/format";
  import { link } from "../lib/router.svelte";

  // Claude Code sessions running right now, with what is known about Remote Control.
  const data = $derived(live.data);
  const unknown = $derived(data?.sessions.some((s) => s.remoteControl === "unknown") ?? false);
</script>

{#if data && (data.sessions.length || data.autoStart)}
  <section class="mt-10">
    <h2 class="mb-1 text-md font-semibold">{t("live.title")}</h2>
    <p class="mb-2 text-xs text-fleece">{t("live.note")}</p>
    {#if data.autoStart}
      <p class="mb-3 rounded-md border border-qolle/50 px-3 py-2 text-sm">{t("live.autoStart")}</p>
    {/if}
    <ul class="divide-y divide-warp">
      {#each data.sessions as s (s.pid)}
        <li class="flex flex-wrap items-center gap-x-4 gap-y-1 py-2.5 text-sm">
          <span class="min-w-0 flex-1">
            <span class="block truncate">{s.name}</span>
            <span class="block truncate font-mono text-xs text-fleece">{s.path}</span>
          </span>
          {#if s.started}<span class="text-xs text-fleece">{t("live.since", { time: dateTime(s.started) })}</span>{/if}
          <span class="w-44 text-right text-xs {s.remoteControl === 'on' ? 'font-semibold text-cochineal' : 'text-fleece'}">
            {t(`live.rc.${s.remoteControl}`)}
          </span>
        </li>
      {/each}
    </ul>
    {#if unknown && !data.watching}
      <p class="mt-2 text-xs text-fleece">
        {t("live.unknownHint")} <a href="/settings" use:link class="text-indigo hover:underline">{t("live.toSettings")}</a>
      </p>
    {/if}
  </section>
{/if}
