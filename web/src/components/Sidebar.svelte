<script lang="ts">
  import type { Session } from "../lib/api/types";
  import { t } from "../lib/i18n.svelte";
  import { palette } from "../lib/palette.svelte";
  import { link, router } from "../lib/router.svelte";
  import Icon from "./Icon.svelte";

  let { session }: { session: Session | null } = $props();

  const items = [
    { href: "/", name: "overview", icon: "overview", key: "O" },
    { href: "/advice", name: "advice", icon: "advice", key: "A" },
    { href: "/cache", name: "cache", icon: "cache", key: "C" },
    { href: "/profiles", name: "profiles", icon: "profiles", key: "R" },
    { href: "/settings", name: "settings", icon: "settings", key: "S" },
  ] as const;

  const current = $derived(router.route.name === "project" ? "overview" : router.route.name);
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
      title={t("nav.shortcut", { key: it.key })}
    >
      <Icon name={it.icon} />
      <span class="flex-1">{t(`nav.${it.name}`)}</span>
    </a>
  {/each}

  <div class="flex-1"></div>

  {#if session?.profiles.length}
    <p class="px-2.5 pb-1 text-xs text-fleece">{t("nav.accounts")}</p>
    {#each session.profiles as p (p.name)}
      <a href="/profiles" use:link class="flex items-center gap-2.5 rounded-md px-2.5 py-1.5 hover:bg-loom {p.current ? 'bg-loom' : ''}" title={p.account || t("profile.signedOut")}>
        <span class="h-2 w-2 shrink-0 rounded-full {p.signedIn ? 'bg-chilca' : 'bg-warp'}" aria-hidden="true"></span>
        <span class="min-w-0 flex-1">
          <span class="block truncate text-sm">{p.name}</span>
          <span class="block truncate text-xs text-fleece">{p.signedIn ? p.account : t("profile.signedOut")}</span>
        </span>
      </a>
    {/each}
  {/if}

  <div class="mt-3 flex items-center justify-between px-2.5">
    <button class="text-xs text-fleece hover:text-cotton" onclick={() => (palette.help = true)}>{t("help.open")}</button>
    {#if session?.version}<span class="text-xs text-fleece/60">{session.version}</span>{/if}
  </div>
</nav>
