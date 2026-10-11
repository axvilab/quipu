<script lang="ts">
  import { fade, scale } from "svelte/transition";
  import { api } from "../lib/api/client";
  import type { ProjectRow } from "../lib/api/types";
  import { i18n, setLocale, t } from "../lib/i18n.svelte";
  import { setTheme, themes } from "../lib/theme.svelte";
  import { palette } from "../lib/palette.svelte";
  import { prefs, setDays } from "../lib/prefs.svelte";
  import { navigate, projectPath } from "../lib/router.svelte";
  import Icon from "./Icon.svelte";

  interface Command {
    id: string;
    label: string;
    hint?: string;
    icon: string;
    run: () => void;
  }

  let query = $state("");
  let active = $state(0);
  let projects = $state<ProjectRow[]>([]);
  let input: HTMLInputElement | undefined = $state();

  $effect(() => {
    if (!palette.open) return;
    query = "";
    active = 0;
    queueMicrotask(() => input?.focus());
    api.overview(prefs.days).then((o) => (projects = o.projects)).catch(() => {});
  });

  const go = (path: string) => () => navigate(path);

  const commands = $derived<Command[]>([
    { id: "overview", label: t("nav.overview"), hint: "G O", icon: "overview", run: go("/") },
    { id: "advice", label: t("nav.advice"), hint: "G A", icon: "advice", run: go("/advice") },
    { id: "cache", label: t("nav.cache"), hint: "G C", icon: "cache", run: go("/cache") },
    { id: "profiles", label: t("nav.profiles"), hint: "G R", icon: "profiles", run: go("/profiles") },
    { id: "settings", label: t("nav.settings"), hint: "G S", icon: "settings", run: go("/settings") },
    ...projects.map((p) => ({ id: "p:" + p.dir, label: p.name, hint: p.path, icon: "folder", run: go(projectPath(p.dir)) })),
    ...[7, 30, 90].map((d) => ({ id: "d" + d, label: t("palette.period", { count: d }), icon: "cache", run: () => setDays(d) })),
    {
      id: "lang",
      label: i18n.locale === "uk" ? "Switch to English" : "Перейти на українську",
      icon: "globe",
      run: () => setLocale(i18n.locale === "uk" ? "en" : "uk"),
    },
    ...themes.map((th) => ({
      id: "theme:" + th,
      label: t("palette.theme", { name: t(`theme.${th}`) }),
      icon: th === "system" ? "monitor" : th === "light" ? "sun" : "moon",
      run: () => setTheme(th),
    })),
    { id: "help", label: t("palette.shortcuts"), hint: "?", icon: "keyboard", run: () => (palette.help = true) },
  ]);

  const shown = $derived(
    commands.filter((c) => !query || (c.label + " " + (c.hint ?? "")).toLowerCase().includes(query.toLowerCase())).slice(0, 12),
  );

  function run(c: Command | undefined) {
    if (!c) return;
    palette.open = false;
    c.run();
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === "ArrowDown") active = Math.min(active + 1, shown.length - 1);
    else if (e.key === "ArrowUp") active = Math.max(active - 1, 0);
    else if (e.key === "Enter") run(shown[active]);
    else if (e.key === "Escape") palette.open = false;
    else return;
    e.preventDefault();
  }
</script>

{#if palette.open}
  <div class="fixed inset-0 z-50 bg-night/60" transition:fade={{ duration: 100 }} onclick={() => (palette.open = false)} aria-hidden="true"></div>
  <div
    class="fixed top-[14vh] left-1/2 z-50 w-[min(600px,calc(100vw-32px))] -translate-x-1/2 overflow-hidden rounded-xl border border-warp bg-loom shadow-2xl"
    transition:scale={{ start: 0.97, duration: 120 }}
    role="dialog"
    aria-modal="true"
    aria-label={t("palette.label")}
  >
    <div class="flex items-center gap-3 border-b border-warp px-4">
      <span class="text-fleece"><Icon name="search" /></span>
      <input
        bind:this={input}
        bind:value={query}
        oninput={() => (active = 0)}
        {onkeydown}
        class="h-12 flex-1 bg-transparent text-md outline-none placeholder:text-fleece"
        placeholder={t("palette.placeholder")}
        aria-label={t("palette.placeholder")}
        role="combobox"
        aria-expanded="true"
        aria-controls="palette-list"
        aria-activedescendant={shown[active] ? "cmd-" + shown[active]!.id : undefined}
      />
      <span class="kbd">Esc</span>
    </div>
    <ul id="palette-list" class="max-h-[50vh] overflow-y-auto p-2" role="listbox">
      {#each shown as c, i (c.id)}
        <li id={"cmd-" + c.id} role="option" aria-selected={i === active}>
          <button
            class="flex w-full items-center gap-3 rounded-md px-3 py-2 text-left {i === active ? 'bg-loom-2' : ''}"
            onmouseenter={() => (active = i)}
            onclick={() => run(c)}
            tabindex="-1"
          >
            <span class="text-fleece"><Icon name={c.icon} /></span>
            <span class="flex-1 truncate">{c.label}</span>
            {#if c.hint}<span class="max-w-[45%] truncate text-xs text-fleece">{c.hint}</span>{/if}
          </button>
        </li>
      {:else}
        <li class="px-3 py-6 text-center text-fleece">{t("palette.empty")}</li>
      {/each}
    </ul>
  </div>
{/if}
