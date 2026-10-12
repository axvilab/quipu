<script lang="ts">
  import { api } from "../lib/api/client";
  import type { PluginCell, PluginMatrix, PluginsMatrix, Result } from "../lib/api/types";
  import { notifyChanged, onChanged } from "../lib/changes.svelte";
  import { dateTime, tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { report } from "../lib/toast.svelte";
  import Header from "../components/Header.svelte";
  import Icon from "../components/Icon.svelte";
  import Loading from "../components/Loading.svelte";

  // Every plugin against every account: where it is on, where it is missing, what it costs and
  // whether it is used. Switching is undoable; installing and removing run Claude Code's own
  // commands under the account, so they ask first.
  let data = $state<PluginsMatrix | null>(null);
  let error = $state<unknown>(null);
  let confirm = $state<{ kind: "remove" | "install"; profile: string; id: string } | null>(null);
  let busy = $state<string | null>(null);

  async function load() {
    try {
      data = await api.plugins();
      error = null;
    } catch (e) {
      error = e;
    }
  }
  load();
  onChanged(load);

  const none: PluginCell = { installed: false, state: "", projects: [], uses: 0, usedIn: [], installable: false };

  // Plugins another account has that this one can still get.
  const missing = (profile: string) => data?.plugins.filter((p) => p.cells[profile]?.installable) ?? [];

  async function run(key: string, p: () => Promise<Result>, undoable = false) {
    busy = key;
    try {
      const res = await p();
      const id = res.params?.change;
      report(res, undoable && id ? { label: t("action.undo"), run: () => api.undo(id).then(report).then(() => notifyChanged()) } : undefined);
      confirm = null;
    } catch (e) {
      report(e);
    } finally {
      busy = null;
      notifyChanged();
    }
  }

  // Installs one by one: each runs claude plugin install under the account.
  async function installAll(profile: string) {
    for (const p of missing(profile)) {
      await run(`${profile}/${p.id}`, () => api.installPlugin(profile, p.id));
    }
  }

  function stateText(c: PluginCell): string {
    if (!c.installed) return t("plugins.state.missing");
    if (c.state === "everywhere") return t("plugins.state.everywhere");
    if (c.state === "projects") return t("plugins.state.projects", { projects: c.projects.join(", ") });
    return t("plugins.state.off");
  }

  const dot = (c: PluginCell) =>
    !c.installed ? "border border-warp" : c.state === "off" ? "bg-warp" : c.uses ? "bg-chilca" : "bg-cochineal";
  const isConfirm = (kind: "remove" | "install", profile: string, p: PluginMatrix) =>
    confirm?.kind === kind && confirm.profile === profile && confirm.id === p.id;
</script>

<Header title={t("plugins.title")} subtitle={t("plugins.subtitle")} />

{#if !data}
  <Loading {error} />
{:else if !data.plugins.length}
  <p class="border-t border-warp py-6 text-sm text-fleece">{t("plugins.empty")}</p>
{:else}
  {@const cols = `minmax(11rem, 1.1fr) repeat(${data.profiles.length}, minmax(13rem, 1fr))`}
  <div class="overflow-x-auto border-t border-warp">
    <div class="grid min-w-max gap-x-6 md:min-w-0" style="grid-template-columns: {cols}">
      <!-- Header: one column per account, with what it can still get from the others. -->
      <div class="py-3 text-xs text-fleece">{t("plugins.plugin")}</div>
      {#each data.profiles as profile (profile)}
        {@const n = missing(profile).length}
        <div class="flex flex-wrap items-center gap-2 py-3">
          <span class="text-sm font-semibold">{data.titles[profile] ?? profile}</span>
          {#if n}
            <button class="btn btn-ghost h-7 px-2 text-xs" disabled={!!busy} onclick={() => installAll(profile)} title={t("plugins.installAllHint", { profile })}>
              <Icon name="plus" size={12} />{t("plugins.installAll", { count: n })}
            </button>
          {/if}
        </div>
      {/each}

      {#each data.plugins as p (p.id)}
        <div class="border-t border-warp py-3">
          <span class="block font-medium">{p.name}</span>
          <span class="block text-xs text-fleece">{p.marketplace}</span>
          {#if p.tokens}<span class="block text-xs tabular-nums text-qolle" title={t("plugins.tokensHint")}>{t("plugins.tokens", { tokens: tokens(p.tokens) })}</span>{/if}
        </div>
        {#each data.profiles as profile (profile)}
          {@const c = p.cells[profile] ?? none}
          {@const key = `${profile}/${p.id}`}
          <div class="border-t border-warp py-3 text-xs">
            <span class="flex items-center gap-2 text-sm">
              <span class="h-2 w-2 shrink-0 rounded-full {dot(c)}" aria-hidden="true"></span>
              <span class={c.installed ? "" : "text-fleece"}>{stateText(c)}</span>
            </span>
            {#if c.installed}
              <span class="mt-0.5 block pl-4 {c.uses ? 'text-fleece' : c.state !== 'off' ? 'text-cochineal' : 'text-fleece'}">
                {c.uses ? t("plugins.used", { count: c.uses, projects: c.usedIn.join(", "), time: dateTime(c.last ?? "") }) : t("plugins.unused")}
              </span>
            {/if}
            <span class="mt-2 flex flex-wrap gap-1.5 pl-4">
              {#if !c.installed}
                {#if c.installable}
                  {#if isConfirm("install", profile, p)}
                    <button class="btn btn-primary h-7 px-2 text-xs" disabled={!!busy} onclick={() => run(key, () => api.installPlugin(profile, p.id))}>
                      {busy === key ? t("plugins.installing") : t("plugins.confirmInstall")}
                    </button>
                    <button class="btn btn-ghost h-7 px-2 text-xs" onclick={() => (confirm = null)}>{t("action.cancel")}</button>
                  {:else}
                    <button class="btn h-7 px-2 text-xs" disabled={!!busy} onclick={() => (confirm = { kind: "install", profile, id: p.id })}>{t("plugins.install")}</button>
                  {/if}
                {:else}
                  <span class="text-fleece">{t("plugins.notInstallable")}</span>
                {/if}
              {:else}
                {#if c.state === "everywhere"}
                  <button class="btn btn-icon h-7 w-7 text-chilca" disabled={!!busy} aria-label={t("plugins.turnOff")} title={t("plugins.turnOff")}
                    onclick={() => run(key, () => api.setPluginEnabled(profile, p.id, false), true)}><Icon name="power" size={14} /></button>
                {:else}
                  <button class="btn btn-icon h-7 w-7 text-fleece" disabled={!!busy} aria-label={t("plugins.turnOn")} title={t("plugins.turnOn")}
                    onclick={() => run(key, () => api.setPluginEnabled(profile, p.id, true), true)}><Icon name="power" size={14} /></button>
                {/if}
                {#if isConfirm("remove", profile, p)}
                  <button class="btn h-7 border-cochineal px-2 text-xs text-cochineal" disabled={!!busy} onclick={() => run(key, () => api.uninstallPlugin(profile, p.id))}>{t("plugins.confirmRemove")}</button>
                  <button class="btn btn-ghost h-7 px-2 text-xs" onclick={() => (confirm = null)}>{t("action.cancel")}</button>
                {:else}
                  <button class="btn btn-ghost btn-icon h-7 w-7" aria-label={t("action.remove")} title={t("action.remove")}
                    onclick={() => (confirm = { kind: "remove", profile, id: p.id })}><Icon name="trash" size={14} /></button>
                {/if}
              {/if}
            </span>
            {#if isConfirm("remove", profile, p)}
              <p class="mt-1.5 pl-4 text-fleece">{t("plugins.removeNote", { id: p.id, profile })}</p>
            {:else if isConfirm("install", profile, p)}
              <p class="mt-1.5 pl-4 text-fleece">{t("plugins.installNote", { id: p.id, profile })}</p>
            {/if}
          </div>
        {/each}
      {/each}
    </div>
  </div>
  <p class="border-t border-warp pt-4 text-xs text-fleece">{t("plugins.note")}</p>
{/if}
