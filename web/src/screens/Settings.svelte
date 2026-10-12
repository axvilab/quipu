<script lang="ts">
  import { i18n, locales, setLocale, t } from "../lib/i18n.svelte";
  import { setTheme, theme, themes } from "../lib/theme.svelte";
  import { api } from "../lib/api/client";
  import { notifyChanged } from "../lib/changes.svelte";
  import { live, refreshLive } from "../lib/live.svelte";
  import { report } from "../lib/toast.svelte";
  import { checkUpdate, installUpdate, setAutoUpdate, updates } from "../lib/update.svelte";
  import { dateTime } from "../lib/format";
  import { external } from "../lib/external";
  import Switch from "../components/Switch.svelte";
  import type { Doctor } from "../lib/api/types";
  import Header from "../components/Header.svelte";
  import Icon from "../components/Icon.svelte";

  // View preferences of this window or browser. Data and changes live in quipu itself.
  const themeIcon = { system: "monitor", light: "sun", dark: "moon" } as const;
  const languageName = { uk: "Українська", en: "English" } as const;

  let busy = $state(false);

  let doctor = $state<Doctor | null>(null);
  async function check() {
    try {
      doctor = await api.doctor();
    } catch (e) {
      report(e);
    }
  }
  check();

  async function setShim(on: boolean) {
    busy = true;
    try {
      report(await api.setShim(on));
    } catch (e) {
      report(e);
    } finally {
      busy = false;
      notifyChanged();
      check();
    }
  }
  async function setShell(shell: string, on: boolean) {
    busy = true;
    try {
      report(await api.setShimRC(shell, on));
    } catch (e) {
      report(e);
    } finally {
      busy = false;
      notifyChanged();
      check();
    }
  }
  const mark = { ok: "✓", info: "–", warn: "!", fail: "✕" } as const;
  const tone = { ok: "text-chilca", info: "text-fleece", warn: "text-qolle", fail: "text-cochineal" } as const;
  async function setWatching(on: boolean) {
    busy = true;
    try {
      const res = await api.watchRemoteControl(on);
      const change = res.params?.change;
      report(res, change ? { label: t("action.undo"), run: () => undo(change) } : undefined);
    } catch (e) {
      report(e);
    } finally {
      busy = false;
      notifyChanged();
      refreshLive();
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
</script>

<Header title={t("settings.title")} subtitle={t("settings.subtitle")} />

<section class="max-w-xl border-t border-warp py-6">
  <h2 class="text-md font-semibold" id="theme-label">{t("theme.label")}</h2>
  <div class="mt-3 grid grid-cols-3 gap-2" role="radiogroup" aria-labelledby="theme-label">
    {#each themes as th (th)}
      <button
        role="radio"
        aria-checked={theme.choice === th}
        class="flex flex-col items-center gap-2 rounded-lg border px-3 py-4 text-sm transition-colors {theme.choice === th
          ? 'border-indigo bg-loom text-cotton'
          : 'border-warp text-fleece hover:border-fleece hover:text-cotton'}"
        onclick={() => setTheme(th)}
      >
        <Icon name={themeIcon[th]} size={20} />
        {t(`theme.${th}`)}
      </button>
    {/each}
  </div>
</section>

<section class="max-w-xl border-t border-warp py-6">
  <h2 class="text-md font-semibold" id="lang-label">{t("lang.label")}</h2>
  <div class="mt-3 grid grid-cols-2 gap-2" role="radiogroup" aria-labelledby="lang-label">
    {#each locales as l (l)}
      <button
        role="radio"
        aria-checked={i18n.locale === l}
        lang={l}
        class="rounded-lg border px-3 py-3 text-sm transition-colors {i18n.locale === l
          ? 'border-indigo bg-loom text-cotton'
          : 'border-warp text-fleece hover:border-fleece hover:text-cotton'}"
        onclick={() => setLocale(l)}
      >
        {languageName[l]}
      </button>
    {/each}
  </div>
</section>

<section class="max-w-xl border-t border-warp py-6">
  <h2 class="text-md font-semibold">{t("settings.update.title")}</h2>
  <p class="mt-1 text-sm leading-relaxed text-fleece">{t("settings.update.note")}</p>
  {#if updates.info}
    {@const u = updates.info}
    {#if u.current.startsWith("v")}
      <p class="mt-3 text-sm">
        {t("settings.update.current", { version: u.current })}
        <span class="text-fleece">{u.checked ? t("settings.update.checked", { time: dateTime(u.checked) }) : t("settings.update.never")}</span>
      </p>
      {#if u.available && u.latest}
        <p class="mt-3 flex flex-wrap items-center gap-3 text-sm">
          <span class="text-indigo">{t("update.available", { version: u.latest })}</span>
          {#if u.notes}<a href={u.notes} target="_blank" rel="noreferrer" use:external class="text-fleece underline hover:text-cotton">{t("update.notes")}</a>{/if}
        </p>
        {#if u.canInstall}
          <button class="btn btn-primary mt-3" disabled={!!updates.busy} title={t("update.installHint", { version: u.latest })} onclick={installUpdate}>
            <Icon name="download" size={14} />{updates.busy === "install" ? t("update.installing") : t("update.install", { version: u.latest })}
          </button>
        {:else}
          <p class="mt-2 text-xs text-fleece">{t("update.manualHint")}</p>
        {/if}
      {:else}
        <button class="btn mt-3" disabled={!!updates.busy} onclick={checkUpdate}>
          <Icon name="refresh" size={14} />{updates.busy === "check" ? t("update.checking") : t("settings.update.now")}
        </button>
      {/if}
      <div class="mt-4 flex items-center gap-3 text-sm">
        <Switch on={u.auto} label={t("settings.update.auto")} onchange={setAutoUpdate} />
        {t("settings.update.auto")}
      </div>
    {:else}
      <p class="mt-3 text-sm text-fleece">{t("settings.update.dev")}</p>
    {/if}
  {/if}
</section>

<section class="max-w-xl border-t border-warp py-6">
  <h2 class="text-md font-semibold">{t("settings.rc.title")}</h2>
  <p class="mt-1 text-sm leading-relaxed text-fleece">{t("settings.rc.note")}</p>
  {#if live.data}
    <label class="mt-3 flex items-center gap-3 text-sm">
      <input type="checkbox" class="h-4 w-4 accent-indigo" checked={live.data.watching} disabled={busy}
        onchange={(e) => setWatching((e.currentTarget as HTMLInputElement).checked)} />
      {t("settings.rc.toggle")}
    </label>
    <p class="mt-2 text-xs text-fleece">{live.data.watching ? t("settings.rc.on") : t("settings.rc.off")}</p>
  {/if}
</section>

<section class="max-w-xl border-t border-warp py-6">
  <h2 class="text-md font-semibold">{t("settings.shim.title")}</h2>
  <p class="mt-1 text-sm leading-relaxed text-fleece">{t("settings.shim.note")}</p>
  {#if doctor}
    {@const sh = doctor.shim}
    <p class="mt-3 text-sm {sh.installed && sh.inRc ? 'text-chilca' : sh.installed ? 'text-qolle' : 'text-fleece'}">
      {sh.installed && sh.inRc ? t("settings.shim.on", { file: sh.rcFile }) : sh.installed ? t("settings.shim.half", { file: sh.rcFile }) : t("settings.shim.off")}
    </p>
    <div class="mt-3 flex gap-2">
      {#if sh.installed}
        <button class="btn" onclick={() => setShim(false)} disabled={busy}>{t("settings.shim.turnOff")}</button>
      {/if}
      {#if !(sh.installed && sh.inRc)}
        <button class="btn btn-primary" onclick={() => setShim(true)} disabled={busy}>{t("settings.shim.turnOn", { file: sh.rcFile })}</button>
      {/if}
    </div>
    {#if sh.installed}
      <h3 class="mt-6 text-sm font-semibold">{t("settings.shim.shells")}</h3>
      <p class="mt-1 text-xs leading-relaxed text-fleece">{t("settings.shim.shellsNote")}</p>
      <ul class="mt-3 divide-y divide-warp border-y border-warp">
        {#each sh.shells as s (s.name)}
          <li class="py-2.5 text-sm">
            <div class="flex items-center gap-3">
              <span class="h-2 w-2 shrink-0 rounded-full {s.inRc ? 'bg-chilca' : 'bg-warp'}" aria-hidden="true"></span>
              <span class="min-w-0 flex-1">
                <span class="font-mono text-[13px]">{s.name}</span>
                {#if s.login}<span class="ml-1.5 text-xs text-fleece">{t("settings.shim.login")}</span>{/if}
                <span class="block text-xs text-fleece">{s.inRc ? t("settings.shim.connected", { file: s.rcFile }) : t("settings.shim.notConnected", { file: s.rcFile })}</span>
              </span>
              {#if s.inRc}
                <button class="btn btn-ghost" onclick={() => setShell(s.name, false)} disabled={busy}>{t("settings.shim.remove")}</button>
              {:else}
                <button class="btn" onclick={() => setShell(s.name, true)} disabled={busy}>{t("settings.shim.add")}</button>
              {/if}
            </div>
            {#if !s.inRc}
              <p class="mt-1.5 pl-5 text-xs text-fleece">{t("settings.shim.manual")} <code class="break-all font-mono">{s.line}</code></p>
            {/if}
          </li>
        {/each}
      </ul>
      <p class="mt-2 text-xs text-fleece">{t("settings.shim.newTerminal")}</p>
    {:else}
      <p class="mt-2 text-xs text-fleece">{t("settings.shim.manual")} <code class="font-mono">{sh.line.split("\n").pop()}</code></p>
    {/if}
  {/if}
</section>

<section class="max-w-xl border-t border-warp py-6">
  <div class="flex items-center justify-between">
    <h2 class="text-md font-semibold">{t("doctor.title")}</h2>
    <button class="btn btn-ghost" onclick={check}>{t("doctor.again")}</button>
  </div>
  {#if doctor}
    <ul class="mt-3 space-y-2">
      {#each doctor.checks as c, i (c.id + i)}
        <li class="flex gap-3 text-sm">
          <span class="w-4 shrink-0 text-center font-semibold {tone[c.state]}" aria-hidden="true">{mark[c.state]}</span>
          <span class="min-w-0">
            <span class="block">{t(`doctor.${c.id}.${c.state}`, c.params)}</span>
            {#if c.state === "warn" || c.state === "fail"}
              <span class="block text-xs text-fleece">{t(`doctor.${c.id}.fix`, c.params)}</span>
            {/if}
          </span>
        </li>
      {/each}
    </ul>
  {/if}
</section>
