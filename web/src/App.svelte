<script lang="ts">
  import { api, ApiError } from "./lib/api/client";
  import type { Session } from "./lib/api/types";
  import { i18n, t } from "./lib/i18n.svelte";
  import { palette } from "./lib/palette.svelte";
  import { link, navigate, router } from "./lib/router.svelte";
  import Icon from "./components/Icon.svelte";
  import CommandPalette from "./components/CommandPalette.svelte";
  import Editor from "./components/Editor.svelte";
  import Panel from "./components/Panel.svelte";
  import { closeEditor, editor } from "./lib/editor.svelte";
  import { watchLive } from "./lib/live.svelte";
  import { collection, watchCollection } from "./lib/collection.svelte";
  import FirstCollection from "./components/FirstCollection.svelte";
  import RemoteControlBanner from "./components/RemoteControlBanner.svelte";
  import Help from "./components/Help.svelte";
  import Sidebar from "./components/Sidebar.svelte";
  import Toasts from "./components/Toasts.svelte";
  import Advice from "./screens/Advice.svelte";
  import Cache from "./screens/Cache.svelte";
  import NotFound from "./screens/NotFound.svelte";
  import Overview from "./screens/Overview.svelte";
  import Profiles from "./screens/Profiles.svelte";
  import Project from "./screens/Project.svelte";
  import Settings from "./screens/Settings.svelte";
  import Backup from "./screens/Backup.svelte";
  import NewProject from "./screens/NewProject.svelte";

  let session = $state<Session | null>(null);
  watchLive();
  watchCollection();
  let unauthorized = $state(false);

  $effect(() => {
    document.documentElement.lang = i18n.locale;
  });

  api
    .session()
    .then((s) => (session = s))
    .catch((e) => (unauthorized = e instanceof ApiError && e.status === 401));

  // Keyboard: Ctrl/⌘K opens the palette, "g" then a letter jumps to a page, "?" shows shortcuts.
  let pendingG = 0;
  function onkeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      e.preventDefault();
      palette.open = !palette.open;
      return;
    }
    const el = e.target instanceof Element ? e.target : null;
    if (e.metaKey || e.ctrlKey || e.altKey || el?.closest("input, textarea, select, [contenteditable]")) return;
    if (e.key === "?") {
      palette.help = true;
      return;
    }
    if (e.key === "g") {
      pendingG = Date.now();
      return;
    }
    if (Date.now() - pendingG < 1200) {
      const to = { o: "/", a: "/advice", c: "/cache", r: "/profiles", n: "/new", b: "/backup", s: "/settings" }[e.key];
      pendingG = 0;
      if (to) navigate(to);
    }
  }

  // Project pages set their own title (the project name).
  const titles: Record<string, string> = { overview: "nav.overview", advice: "nav.advice", cache: "nav.cache", profiles: "nav.profiles", new: "nav.new", backup: "nav.backup", settings: "nav.settings" };
  $effect(() => {
    const key = titles[router.route.name];
    if (key) document.title = `${t(key)} · Quipu`;
  });
</script>

<svelte:window {onkeydown} />

{#if unauthorized}
  <main class="grid min-h-screen place-items-center px-4">
    <div class="max-w-md text-center">
      <img src="/static/icon.svg" alt="" width="48" height="48" class="mx-auto mb-4 rounded-lg" />
      <h1 class="text-lg font-semibold">{t("auth.title")}</h1>
      <p class="mt-2 text-fleece">{t("auth.body")}</p>
      <code class="mt-4 inline-block rounded-md border border-warp bg-loom px-3 py-1.5 font-mono text-sm">quipu ui</code>
    </div>
  </main>
{:else}
  <div class="grid min-h-screen grid-cols-1 md:grid-cols-[232px_minmax(0,1fr)]">
    <div class="hidden md:block"><div class="sticky top-0 h-screen"><Sidebar {session} /></div></div>
    <header class="sticky top-0 z-20 flex h-12 items-center gap-1 border-b border-warp bg-night px-3 md:hidden">
      <a href="/" use:link class="mr-2 flex items-center gap-2 font-semibold"><img src="/static/icon.svg" alt="" width="22" height="22" class="rounded" />Quipu</a>
      {#each [["/", "overview"], ["/advice", "advice"], ["/cache", "cache"], ["/profiles", "profiles"], ["/backup", "backup"], ["/settings", "settings"]] as [href, name] (name)}
        <a {href} use:link class="grid h-9 w-9 place-items-center rounded-md text-fleece hover:bg-loom hover:text-cotton" aria-label={t(`nav.${name}`)}><Icon name={name!} /></a>
      {/each}
      <button class="ml-auto grid h-9 w-9 place-items-center rounded-md text-fleece hover:bg-loom" onclick={() => (palette.open = true)} aria-label={t("palette.search")}><Icon name="search" /></button>
    </header>
    <main class="min-w-0 px-4 py-6 md:px-10 md:py-8">
      <div class="mx-auto max-w-[1180px]">
        <RemoteControlBanner />
        {#if collection.data?.first && ["overview", "project", "advice", "cache", "new"].includes(router.route.name)}<FirstCollection state={collection.data} />
        {:else if router.route.name === "overview"}<Overview />
        {:else if router.route.name === "project"}{#key router.route.dir}<Project dir={router.route.dir} />{/key}
        {:else if router.route.name === "advice"}<Advice />
        {:else if router.route.name === "cache"}<Cache />
        {:else if router.route.name === "profiles"}<Profiles />
        {:else if router.route.name === "settings"}<Settings />
        {:else if router.route.name === "backup"}<Backup />
        {:else if router.route.name === "new"}<NewProject />
        {:else}<NotFound />{/if}
      </div>
    </main>
  </div>
  {#if editor.path}
    <Panel wide title={t("editor.title")} onclose={() => closeEditor()}>
      {#key editor.path}<Editor path={editor.path} />{/key}
    </Panel>
  {/if}
  <CommandPalette userFile={session?.userFile ?? ""} />
  <Help />
  <Toasts />
{/if}
