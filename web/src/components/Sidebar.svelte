<script lang="ts">
  import type { Session } from "../lib/api/types";
  import { t } from "../lib/i18n.svelte";
  import { palette } from "../lib/palette.svelte";
  import { prefs, setAccountsOpen } from "../lib/prefs.svelte";
  import { link, router } from "../lib/router.svelte";
  import { external } from "../lib/external";
  import { checkUpdate, installUpdate, loadUpdate, updates } from "../lib/update.svelte";
  import Icon from "./Icon.svelte";

  let { session }: { session: Session | null } = $props();

  const items = [
    { href: "/", name: "overview", icon: "overview", key: "O" },
    { href: "/advice", name: "advice", icon: "advice", key: "A" },
    { href: "/cache", name: "cache", icon: "cache", key: "C" },
    { href: "/profiles", name: "profiles", icon: "profiles", key: "R" },
    { href: "/plugins", name: "plugins", icon: "plugins", key: "P" },
    { href: "/backup", name: "backup", icon: "backup", key: "B" },
    { href: "/settings", name: "settings", icon: "settings", key: "S" },
  ] as const;

  // What the server knows about a newer release; it checks GitHub only when asked or, if the user
  // turned that on, once a day, so reading this now and then is enough.
  $effect(() => {
    loadUpdate();
    const timer = setInterval(loadUpdate, 30 * 60_000);
    return () => clearInterval(timer);
  });

  // The project pages and the new project page belong to the overview.
  const current = $derived(router.route.name === "project" || router.route.name === "new" ? "overview" : router.route.name);
</script>

<nav class="flex h-full flex-col gap-1 border-r border-warp bg-night px-3 py-4" aria-label={t("nav.label")}>
  <a href="/" use:link class="mb-4 flex items-center gap-2.5 px-2 text-md font-semibold">
    <img src="/static/icon.svg" alt="" width="24" height="24" class="rounded-md" />
    Quipu
  </a>

  <button class="mb-3 flex h-8 items-center gap-2 rounded-md border border-warp bg-loom px-2.5 text-sm text-fleece hover:text-cotton"
    onclick={() => (palette.open = true)}>
    <Icon name="search" size={14} />
    <span class="flex-1 truncate text-left">{t("palette.search")}</span>
    <span class="kbd shrink-0">Ctrl K</span>
  </button>

  {#each items as it (it.name)}
    <a
      href={it.href}
      use:link
      class="group flex h-8 items-center gap-2.5 rounded-md px-2.5 text-sm transition-colors {current === it.name ? 'bg-loom-2 text-cotton' : 'text-fleece hover:bg-loom hover:text-cotton'}"
      aria-current={current === it.name ? "page" : undefined}
    >
      <Icon name={it.icon} />
      <span class="flex-1">{t(`nav.${it.name}`)}</span>
      <span class="hidden text-xs text-fleece/70 group-hover:inline" aria-hidden="true">G {it.key}</span>
    </a>
  {/each}

  <div class="flex-1"></div>

  {#if session?.profiles.length}
    <!-- Folded to one line by default: a dot per account; open shows each with its sign-in. -->
    <button class="flex items-center gap-2 rounded-md px-2.5 py-1.5 text-xs text-fleece hover:bg-loom hover:text-cotton"
      aria-expanded={prefs.accountsOpen} onclick={() => setAccountsOpen(!prefs.accountsOpen)}>
      <Icon name={prefs.accountsOpen ? "down" : "right"} size={12} />
      <span class="flex-1 text-left">{t("nav.accounts")}</span>
      {#if !prefs.accountsOpen}
        {#each session.profiles as p (p.name)}
          <span class="h-2 w-2 shrink-0 rounded-full {p.signedIn ? 'bg-chilca' : 'bg-warp'}" title="{p.title || p.name}: {p.signedIn ? p.account : t('profile.signedOut')}"></span>
        {/each}
      {/if}
    </button>
    {#if prefs.accountsOpen}
    {#each session.profiles as p (p.name)}
      <a href="/profiles" use:link class="flex items-center gap-2.5 rounded-md px-2.5 py-1.5 hover:bg-loom {p.current ? 'bg-loom' : ''}" title={p.account || t("profile.signedOut")}>
        <span class="h-2 w-2 shrink-0 rounded-full {p.signedIn ? 'bg-chilca' : 'bg-warp'}" aria-hidden="true"></span>
        <span class="min-w-0 flex-1">
          <span class="block truncate text-sm">{p.title || p.name}</span>
          <span class="block truncate text-xs text-fleece">{p.signedIn ? p.account : t("profile.signedOut")}</span>
        </span>
      </a>
    {/each}
    {/if}
  {/if}

  <div class="mt-3 flex items-center justify-between gap-3 px-2.5">
    <button class="shrink-0 text-xs text-fleece hover:text-cotton" onclick={() => (palette.help = true)}>{t("help.open")}</button>
    {#if updates.info?.available && updates.info.latest}
      {@const version = updates.info.latest}
      {#if updates.info.canInstall}
        <button class="flex min-w-0 items-center gap-1.5 text-xs text-indigo hover:text-cotton disabled:opacity-60" disabled={!!updates.busy}
          title={t("update.installHint", { version })} onclick={installUpdate}>
          <Icon name="download" size={12} />
          <span class="truncate">{updates.busy === "install" ? t("update.installing") : t("update.install", { version })}</span>
        </button>
      {:else}
        <a href={updates.info.notes} target="_blank" rel="noreferrer" use:external class="min-w-0 truncate text-xs text-indigo hover:text-cotton"
          title={t("update.manualHint")}>{t("update.available", { version })}</a>
      {/if}
    {:else if session?.version}
      <span class="flex min-w-0 items-center gap-1">
        <span class="truncate text-xs text-fleece/60" title={session.version}>{session.version}</span>
        {#if session.version.startsWith("v")}
          <button class="btn-icon flex h-6 w-6 items-center justify-center rounded-md text-fleece/60 hover:text-cotton" disabled={!!updates.busy}
            aria-label={updates.busy === "check" ? t("update.checking") : t("update.check")} title={t("update.check")} onclick={checkUpdate}>
            <span class={updates.busy === "check" ? "animate-spin motion-reduce:animate-none" : ""}><Icon name="refresh" size={12} /></span>
          </button>
        {/if}
      </span>
    {/if}
  </div>
</nav>
