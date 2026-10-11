<script lang="ts">
  import { i18n, locales, setLocale, t } from "../lib/i18n.svelte";
  import { setTheme, theme, themes } from "../lib/theme.svelte";
  import { api } from "../lib/api/client";
  import { notifyChanged } from "../lib/changes.svelte";
  import { live, refreshLive } from "../lib/live.svelte";
  import { report } from "../lib/toast.svelte";
  import Header from "../components/Header.svelte";
  import Icon from "../components/Icon.svelte";

  // View preferences of this window or browser. Data and changes live in quipu itself.
  const themeIcon = { system: "monitor", light: "sun", dark: "moon" } as const;
  const languageName = { uk: "Українська", en: "English" } as const;

  let busy = $state(false);
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
