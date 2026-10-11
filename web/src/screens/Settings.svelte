<script lang="ts">
  import { i18n, locales, setLocale, t } from "../lib/i18n.svelte";
  import { setTheme, theme, themes } from "../lib/theme.svelte";
  import Header from "../components/Header.svelte";
  import Icon from "../components/Icon.svelte";

  // View preferences of this window or browser. Data and changes live in quipu itself.
  const themeIcon = { system: "monitor", light: "sun", dark: "moon" } as const;
  const languageName = { uk: "Українська", en: "English" } as const;
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
