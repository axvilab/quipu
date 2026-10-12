<script lang="ts">
  import { api } from "../lib/api/client";
  import type { BackupInfo, BackupResult, RestorePlan } from "../lib/api/types";
  import { notifyChanged } from "../lib/changes.svelte";
  import { dateTime } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { report } from "../lib/toast.svelte";
  import Header from "../components/Header.svelte";
  import Icon from "../components/Icon.svelte";
  import Loading from "../components/Loading.svelte";
  import Panel from "../components/Panel.svelte";
  import Switch from "../components/Switch.svelte";

  // Save the Claude Code setup of every profile, and get it back with a preview first.
  let info = $state<BackupInfo | null>(null);
  let error = $state<unknown>(null);
  let gitDir = $state("");
  let push = $state(false);
  let busy = $state(false);
  let blocked = $state<{ result: BackupResult; git: boolean } | null>(null);
  let plan = $state<RestorePlan | null>(null);
  let source = $state("");

  async function load() {
    try {
      info = await api.backup();
      if (!gitDir) gitDir = info.gitDir;
      error = null;
    } catch (e) {
      error = e;
    }
  }
  load();

  async function create(git: boolean, allowSecrets = false) {
    busy = true;
    try {
      const res = await api.createBackup({ gitDir: git ? gitDir : "", push: git && push, allowSecrets });
      if (!res.ok && res.secrets.length) {
        blocked = { result: res, git };
        return;
      }
      blocked = null;
      if (git && !res.changed) report({ ok: true, code: "backup-unchanged", params: { path: res.path ?? "" } });
      else report({ ok: true, code: git ? "backup-git" : "backup-archive", params: { path: res.path ?? "", count: String(res.files) } });
      load();
    } catch (e) {
      report(e);
    } finally {
      busy = false;
    }
  }

  async function preview(from: string) {
    source = from;
    try {
      plan = await api.planRestore(from);
    } catch (e) {
      report(e);
    }
  }

  async function restore() {
    if (!plan) return;
    busy = true;
    try {
      const res = await api.restore(source);
      const change = res.params?.change;
      report(res, change ? { label: t("action.undo"), run: () => undo(change) } : undefined);
      plan = null;
      notifyChanged();
    } catch (e) {
      report(e);
    } finally {
      busy = false;
    }
  }

  async function undo(change: string) {
    try {
      report(await api.undo(change));
    } catch (e) {
      report(e);
    }
    notifyChanged();
  }

  async function exclude(dir: string, out: boolean) {
    try {
      report(await api.excludeFromBackup(dir, out));
    } catch (e) {
      report(e);
    }
    load();
  }

  const size = (n: number) => (n < 1024 ? `${n} B` : `${Math.ceil(n / 1024)} KB`);
</script>

<Header title={t("backup.title")} subtitle={t("backup.subtitle")} />

{#if !info}
  <Loading {error} />
{:else}
  <section class="max-w-2xl border-t border-warp py-6">
    <h2 class="text-md font-semibold">{t("backup.create")}</h2>
    <p class="mt-1 text-sm leading-relaxed text-fleece">{t("backup.what")}</p>

    <div class="mt-4 flex flex-wrap items-center gap-3">
      <button class="btn btn-primary" onclick={() => create(false)} disabled={busy}><Icon name="backup" />{t("backup.toArchive")}</button>
      <span class="font-mono text-xs text-fleece">{info.folder}</span>
    </div>

    <div class="mt-5 rounded-lg border border-warp p-4">
      <label class="block text-sm" for="git-dir">{t("backup.gitLabel")}</label>
      <p class="mt-0.5 text-xs text-fleece">{t("backup.gitNote")}</p>
      <div class="mt-2 flex flex-wrap items-center gap-3">
        <input id="git-dir" class="field min-w-0 flex-1 font-mono text-[13px]" placeholder="~/claude-setup" bind:value={gitDir} />
        <button class="btn" onclick={() => create(true)} disabled={busy || !gitDir.trim()}>{t("backup.toGit")}</button>
      </div>
      <label class="mt-2 flex items-center gap-2 text-sm"><input type="checkbox" class="accent-indigo" bind:checked={push} />{t("backup.push")}</label>
    </div>

    {#if blocked}
      <div class="mt-5 rounded-lg border border-cochineal/50 bg-cochineal/10 p-4 text-sm" role="alert">
        <p class="font-semibold">{t("backup.secretsTitle")}</p>
        <ul class="mt-2 space-y-0.5 font-mono text-xs">
          {#each blocked.result.secrets as s (s.path + s.line)}<li>{s.path}:{s.line} <span class="text-fleece">{s.kind}</span></li>{/each}
        </ul>
        <p class="mt-2 text-xs text-cotton/80">{t("backup.secretsNote")}</p>
        <div class="mt-3 flex gap-2">
          <button class="btn" onclick={() => create(blocked!.git, true)} disabled={busy}>{t("backup.anyway")}</button>
          <button class="btn btn-ghost" onclick={() => (blocked = null)}>{t("action.close")}</button>
        </div>
      </div>
    {/if}
  </section>

  {#if info.projects.length}
    <section class="max-w-2xl border-t border-warp py-6">
      <h2 class="text-md font-semibold">{t("backup.projects")}</h2>
      <p class="mt-1 text-sm text-fleece">{t("backup.projectsNote")}</p>
      <ul class="mt-3 divide-y divide-warp border-y border-warp">
        {#each info.projects as p (p.dir)}
          <li class="flex items-center gap-3 py-2.5 text-sm">
            <Switch on={!p.excluded} label={p.name} onchange={(on) => exclude(p.dir, !on)} />
            <span class="min-w-0 flex-1">
              <span class="block truncate font-mono text-[12.5px] {p.excluded ? 'text-fleece line-through' : ''}">{p.name}</span>
              <span class="block text-xs text-fleece">{t("backup.projectFiles", { count: p.files, profiles: p.profiles.join(", ") })}</span>
            </span>
          </li>
        {/each}
      </ul>
    </section>
  {/if}

  <section class="max-w-2xl border-t border-warp py-6">
    <h2 class="text-md font-semibold">{t("backup.restore")}</h2>
    <p class="mt-1 text-sm text-fleece">{t("backup.restoreNote")}</p>
    {#if info.archives.length}
      <ul class="mt-3 divide-y divide-warp">
        {#each info.archives as a (a.path)}
          <li class="flex items-center gap-4 py-2.5 text-sm">
            <span class="min-w-0 flex-1">
              <span class="block truncate font-mono text-[12.5px]">{a.name}</span>
              <span class="block text-xs text-fleece">{dateTime(a.time)}</span>
            </span>
            <span class="text-xs text-fleece">{size(a.size)}</span>
            <button class="btn" onclick={() => preview(a.path)}>{t("backup.restoreThis")}</button>
          </li>
        {/each}
      </ul>
    {:else}
      <p class="mt-3 text-sm text-fleece">{t("backup.noArchives")}</p>
    {/if}
    {#if gitDir.trim()}
      <button class="btn btn-ghost mt-3" onclick={() => preview(gitDir)}>{t("backup.restoreGit")}</button>
    {/if}
  </section>
{/if}

{#if plan}
  {@const p = plan}
  <Panel title={t("backup.previewTitle")} onclose={() => (plan = null)}>
    <div class="flex flex-col gap-4 text-sm">
      <p class="text-fleece">{t("backup.previewFrom", { time: dateTime(p.created), home: p.from, version: p.version })}</p>
      <p>{t("backup.previewCounts", { new: p.new, changed: p.changed, same: p.same })}</p>
      {#if p.files.length}
        <ul class="space-y-1 font-mono text-[12.5px]">
          {#each p.files as f (f.path)}
            <li class="flex gap-3"><span class="w-16 shrink-0 {f.state === 'new' ? 'text-chilca' : 'text-qolle'}">{t(`backup.state.${f.state}`)}</span><span class="break-all">{f.path}</span></li>
          {/each}
        </ul>
      {/if}
      {#if p.links}<p class="text-xs text-fleece">{t("backup.previewLinks", { count: p.links })}</p>{/if}
      <div class="flex items-center gap-3 border-t border-warp pt-4">
        {#if p.files.length || p.links}
          <button class="btn btn-primary" onclick={restore} disabled={busy} data-autofocus><Icon name="check" />{t("backup.apply")}</button>
          <span class="text-xs text-fleece">{t("backup.applyNote")}</span>
        {:else}
          <span class="text-fleece">{t("backup.nothing")}</span>
        {/if}
      </div>
    </div>
  </Panel>
{/if}
