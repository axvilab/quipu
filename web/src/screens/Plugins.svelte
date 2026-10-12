<script lang="ts">
  import { api } from "../lib/api/client";
  import type { PluginRow, Result } from "../lib/api/types";
  import { notifyChanged, onChanged } from "../lib/changes.svelte";
  import { dateTime, tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { report } from "../lib/toast.svelte";
  import Header from "../components/Header.svelte";
  import Loading from "../components/Loading.svelte";

  // Every plugin installed in every profile: where it is on, what it costs each session and
  // whether it was used. Plugins are switched in the profile's settings (undoable) or removed with
  // Claude Code's own command.
  let rows = $state<PluginRow[] | null>(null);
  let error = $state<unknown>(null);
  let removing = $state<string | null>(null);
  let busy = $state(false);

  async function load() {
    try {
      rows = await api.plugins();
      error = null;
    } catch (e) {
      error = e;
    }
  }
  load();
  onChanged(load);

  const key = (r: PluginRow) => `${r.profile}/${r.id}`;
  const profiles = $derived([...new Set(rows?.map((r) => r.profile) ?? [])]);

  async function run(p: Promise<Result>, undoable: boolean) {
    busy = true;
    try {
      const res = await p;
      const id = res.params?.change;
      report(res, undoable && id ? { label: t("action.undo"), run: () => api.undo(id).then(report).then(() => notifyChanged()) } : undefined);
      removing = null;
    } catch (e) {
      report(e);
    } finally {
      busy = false;
      notifyChanged();
    }
  }
</script>

<Header title={t("plugins.title")} subtitle={t("plugins.subtitle")} />

{#if !rows}
  <Loading {error} />
{:else if !rows.length}
  <p class="border-t border-warp py-6 text-sm text-fleece">{t("plugins.empty")}</p>
{:else}
  {#each profiles as profile (profile)}
    <section class="border-t border-warp py-5">
      {#if profiles.length > 1}<h2 class="mb-2 text-sm font-semibold">{t("plugins.profile", { name: profile })}</h2>{/if}
      <ul class="divide-y divide-warp">
        {#each rows.filter((r) => r.profile === profile) as r (key(r))}
          <li class="py-3">
            <div class="flex flex-wrap items-start gap-x-4 gap-y-2">
              <span class="min-w-0 flex-1">
                <span class="block">
                  <span class="font-medium">{r.name}</span>
                  <span class="ml-1.5 text-xs text-fleece">{r.marketplace}{r.version ? ` · ${r.version}` : ""}</span>
                </span>
                <span class="block text-xs text-fleece">
                  {r.state === "everywhere" ? t("plugins.state.everywhere") : r.state === "projects" ? t("plugins.state.projects", { projects: r.projects.join(", ") }) : t("plugins.state.off")}
                </span>
                <span class="block text-xs {r.uses ? 'text-fleece' : r.state !== 'off' ? 'text-cochineal' : 'text-fleece'}">
                  {r.uses ? t("plugins.used", { count: r.uses, projects: r.usedIn.join(", "), time: dateTime(r.last ?? "") }) : t("plugins.unused")}
                </span>
              </span>
              <span class="shrink-0 text-right text-xs tabular-nums {r.state === 'off' ? 'text-fleece' : 'text-qolle'}" title={t("plugins.tokensHint")}>
                {r.tokens ? t("plugins.tokens", { tokens: tokens(r.tokens) }) : ""}
              </span>
              <span class="flex shrink-0 gap-2">
                {#if r.state === "everywhere"}
                  <button class="btn" disabled={busy} onclick={() => run(api.setPluginEnabled(r.profile, r.id, false), true)}>{t("plugins.turnOff")}</button>
                {:else}
                  <button class="btn" disabled={busy} onclick={() => run(api.setPluginEnabled(r.profile, r.id, true), true)}>{t("plugins.turnOn")}</button>
                {/if}
                {#if removing === key(r)}
                  <button class="btn border-cochineal text-cochineal" disabled={busy} onclick={() => run(api.uninstallPlugin(r.profile, r.id), false)}>{t("plugins.confirmRemove")}</button>
                  <button class="btn btn-ghost" onclick={() => (removing = null)}>{t("action.cancel")}</button>
                {:else}
                  <button class="btn btn-ghost" onclick={() => (removing = key(r))}>{t("action.remove")}</button>
                {/if}
              </span>
            </div>
            {#if removing === key(r)}
              <p class="mt-2 text-xs text-fleece">{t("plugins.removeNote", { id: r.id, profile: r.profile })}</p>
            {/if}
          </li>
        {/each}
      </ul>
    </section>
  {/each}
  <p class="border-t border-warp pt-4 text-xs text-fleece">{t("plugins.note")}</p>
{/if}
