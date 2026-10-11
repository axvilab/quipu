<script lang="ts">
  import { slide } from "svelte/transition";
  import { api, ApiError } from "../lib/api/client";
  import type { NewExtension, NewFile, NewInfo, NewRequest, Result } from "../lib/api/types";
  import { notifyChanged } from "../lib/changes.svelte";
  import { tokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { report } from "../lib/toast.svelte";
  import Header from "../components/Header.svelte";
  import Icon from "../components/Icon.svelte";
  import Loading from "../components/Loading.svelte";

  // Set up a new or freshly cloned project from the user's own skills, rules and agents. Copies
  // that are already in several projects are offered as global instead, so the wizard never makes
  // the duplicates the advice warns about. The right side shows what Claude will get.
  let info = $state<NewInfo | null>(null);
  let error = $state<unknown>(null);

  let name = $state("");
  let parent = $state("");
  let summary = $state("");
  let profile = $state("");
  // How each picked extension is reused: copied into the project, or moved to the user's settings.
  let picked = $state<Record<string, "copy" | "global">>({});

  let files = $state<NewFile[]>([]);
  let planError = $state<ApiError | null>(null);
  let open = $state<string | null>(null);
  let busy = $state(false);
  let created = $state<Result | null>(null);

  async function load() {
    try {
      info = await api.newInfo();
      parent = info.parent;
      error = null;
    } catch (e) {
      error = e;
    }
  }
  load();

  const base = $derived(parent.replace(/\/+$/, ""));
  const folder = $derived(name.trim() ? `${base}/${name.trim()}` : "");
  const request = $derived<NewRequest>({
    folder,
    summary,
    profile,
    extensions: Object.keys(picked).filter((p) => picked[p] === "copy"),
    global: Object.keys(picked).filter((p) => picked[p] === "global"),
  });

  // Preview again shortly after the last change; without a name, under a placeholder one.
  let timer: ReturnType<typeof setTimeout> | undefined;
  $effect(() => {
    if (!info) return;
    const req = $state.snapshot(request);
    const placeholder = `${base}/${t("new.namePlaceholder")}`;
    clearTimeout(timer);
    timer = setTimeout(async () => {
      try {
        files = await api.planNew({ ...req, folder: req.folder || placeholder });
        planError = null;
      } catch (e) {
        planError = e instanceof ApiError ? e : new ApiError(500, "internal", { error: String(e) });
      }
    }, 200);
  });

  const always = $derived(files.filter((f) => f.always > 0).sort((a, b) => b.always - a.always));
  const onDemand = $derived(files.filter((f) => f.always === 0 && f.onDemand > 0));
  const unread = $derived(files.filter((f) => f.always === 0 && f.onDemand === 0));
  const start = $derived(always.reduce((n, f) => n + f.always, 0));
  const scale = $derived(Math.max(start, info?.average ?? 0, 1));
  const groups = $derived([
    { id: "always", files: always },
    { id: "demand", files: onDemand },
    { id: "unread", files: unread },
  ]);
  const existing = $derived(files.some((f) => f.existing));
  const nothingNew = $derived(files.length > 0 && files.every((f) => f.existing) && !profile);

  function toggle(e: NewExtension) {
    if (picked[e.path]) delete picked[e.path];
    else picked[e.path] = e.global ? "global" : "copy";
  }

  function cost(f: NewFile): string {
    if (f.always && f.onDemand) return t("new.costBoth", { always: tokens(f.always), demand: tokens(f.onDemand) });
    if (f.always || f.onDemand) return tokens(f.always || f.onDemand);
    return "";
  }

  async function create() {
    busy = true;
    try {
      const res = await api.createNew($state.snapshot(request));
      const change = res.params?.change;
      report(res, change ? { label: t("action.undo"), run: () => undo(change) } : undefined);
      created = res;
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
      created = null;
    } catch (e) {
      report(e);
    }
    notifyChanged();
  }

  function again() {
    created = null;
    name = "";
    summary = "";
    picked = {};
    load();
  }

  const command = $derived(created ? `cd ${created.params?.folder} && claude` : "");
  const chip = (on: boolean) =>
    `rounded-md border px-2.5 py-1 text-xs transition-colors ${on ? "border-indigo bg-loom text-cotton" : "border-warp text-fleece hover:border-fleece hover:text-cotton"}`;
</script>

{#snippet step(n: number, title: string, hint: string)}
  <div class="flex items-start gap-3">
    <span class="grid h-6 w-6 shrink-0 place-items-center rounded-full border border-warp text-xs tabular-nums text-fleece" aria-hidden="true">{n}</span>
    <div>
      <h2 class="text-md font-semibold leading-6">{title}</h2>
      <p class="mt-0.5 text-xs text-fleece">{hint}</p>
    </div>
  </div>
{/snippet}

<Header title={t("new.title")} subtitle={t("new.subtitle")} />

{#if !info}
  <Loading {error} />
{:else if created}
  <section class="max-w-2xl border-t border-warp py-6">
    <h2 class="text-md font-semibold">{t("new.done", { name: created.params?.name ?? "" })}</h2>
    {#if created.params?.ruleError}
      <p class="mt-2 text-sm text-cochineal">{t("new.ruleError")}</p>
    {/if}
    <p class="mt-3 text-sm text-fleece">{t("new.next")}</p>
    <div class="mt-2 flex flex-wrap items-center gap-2">
      <code class="rounded-md border border-warp bg-loom px-3 py-1.5 font-mono text-[13px]">{command}</code>
      <button class="btn" onclick={() => navigator.clipboard.writeText(command).then(() => report({ ok: true, code: "command-copied" }))}>
        <Icon name="copy" size={14} />{t("new.copy")}
      </button>
    </div>
    <p class="mt-4 text-sm text-fleece">{t("new.undoNote")}</p>
    <button class="btn btn-ghost mt-4" onclick={again}><Icon name="plus" />{t("new.another")}</button>
  </section>
{:else}
  <div class="grid gap-x-12 border-t border-warp lg:grid-cols-[minmax(0,27rem)_minmax(0,1fr)]">
    <ol class="flex flex-col gap-8 py-6">
      <li class="flex flex-col gap-3">
        {@render step(1, t("new.step1"), t("new.step1Hint"))}
        <div class="grid gap-3 pl-9 sm:grid-cols-[1fr_1.3fr]">
          <label class="block text-sm">
            {t("new.name")}
            <input class="field mt-1 w-full font-mono text-[13px]" placeholder={t("new.namePlaceholder")} bind:value={name} data-autofocus />
          </label>
          <label class="block text-sm">
            {t("new.parent")}
            <input class="field mt-1 w-full font-mono text-[13px]" bind:value={parent} />
          </label>
          {#if existing && folder}
            <p class="text-xs text-fleece sm:col-span-2">{t("new.existingFolder")}</p>
          {/if}
          <label class="block text-sm sm:col-span-2">
            {t("new.summary")}
            <textarea class="field mt-1 min-h-16 w-full py-2" rows="2" placeholder={t("new.summaryHint")} bind:value={summary}></textarea>
          </label>
          {#if info.profiles.length > 1}
            <label class="flex flex-col items-start text-sm sm:col-span-2">
              {t("new.profile")}
              <select class="field mt-1.5 w-full sm:w-auto sm:min-w-56" bind:value={profile}>
                <option value="">{t("new.profileNone")}</option>
                {#each info.profiles as p (p)}<option value={p}>{p}</option>{/each}
              </select>
            </label>
          {/if}
        </div>
      </li>

      <li class="flex flex-col gap-3">
        {@render step(2, t("new.step2"), t("new.step2Hint"))}
        <div class="pl-9">
          {#if info.extensions.length}
            <ul class="max-h-96 divide-y divide-warp overflow-y-auto rounded-lg border border-warp">
              {#each info.extensions as e (e.path)}
                {@const choice = picked[e.path]}
                <li class="px-3 py-2 text-sm {choice ? 'bg-loom/50' : ''}">
                  <label class="flex cursor-pointer items-start gap-3">
                    <input type="checkbox" class="mt-1 accent-indigo" checked={!!choice} onchange={() => toggle(e)} />
                    <span class="min-w-0 flex-1">
                      <span class="block truncate"><span class="text-fleece">{t(`kind.${e.kind}`)}</span> {e.name}</span>
                      <span class="block truncate text-xs text-fleece">{t("new.usedIn", { projects: e.projects.join(", ") })}</span>
                      {#if e.variants > 1}<span class="block text-xs text-fleece">{t("new.variants", { count: e.variants })}</span>{/if}
                    </span>
                    <span class="shrink-0 text-xs tabular-nums {e.tokens ? 'text-qolle' : 'text-fleece'}">{e.tokens ? t("new.plusTokens", { tokens: tokens(e.tokens) }) : t("new.onDemand")}</span>
                  </label>
                  {#if choice && e.global}
                    <div class="mt-2 flex flex-col gap-1.5 pl-7" transition:slide={{ duration: 120 }}>
                      <div class="flex flex-wrap gap-1.5" role="radiogroup" aria-label={e.name}>
                        <button role="radio" aria-checked={choice === "global"} class={chip(choice === "global")} onclick={() => (picked[e.path] = "global")}>{t("new.makeGlobal")}</button>
                        <button role="radio" aria-checked={choice === "copy"} class={chip(choice === "copy")} onclick={() => (picked[e.path] = "copy")}>{t("new.copyHere")}</button>
                      </div>
                      <p class="text-xs text-fleece">{t(choice === "global" ? "new.globalNote" : "new.copyNote", { count: e.projects.length + 1 })}</p>
                    </div>
                  {/if}
                </li>
              {/each}
            </ul>
          {:else}
            <p class="text-sm text-fleece">{t("new.noExtensions")}</p>
          {/if}
        </div>
      </li>

      <li class="flex flex-col gap-3">
        {@render step(3, t("new.step3"), t("new.step3Hint"))}
        <div class="flex flex-col gap-4 pl-9">
          {#if planError}
            <p class="text-sm text-cochineal" role="alert">{t(`result.${planError.code}`, planError.params)}</p>
          {:else if folder && nothingNew}
            <p class="text-sm text-fleece">{t("new.nothingNew")}</p>
          {/if}
          <div class="flex flex-wrap items-center gap-3">
            <button class="btn btn-primary" onclick={create} disabled={busy || !folder || !!planError || nothingNew}><Icon name="check" />{t(existing ? "new.setUp" : "new.create")}</button>
            <span class="min-w-0 truncate text-xs text-fleece {folder ? 'font-mono' : ''}">{folder || t("new.needName")}</span>
          </div>
        </div>
      </li>
    </ol>

    <aside class="min-w-0 border-t border-warp py-6 lg:sticky lg:top-0 lg:max-h-screen lg:self-start lg:overflow-y-auto lg:border-t-0 lg:border-l lg:pl-10" aria-label={t("new.gets")}>
      <h2 class="text-md font-semibold">{t("new.gets")}</h2>

      <p class="mt-4 text-sm text-fleece">{t("new.startLabel")}</p>
      <p class="mt-1 text-3xl font-semibold tabular-nums text-qolle">{t("new.startTokens", { tokens: tokens(start) })}</p>
      {#if info.average}
        <div class="mt-3 flex max-w-md flex-col gap-1.5 text-xs text-fleece">
          <div class="flex items-center gap-3">
            <span class="w-44 shrink-0 whitespace-nowrap">{t("new.thisOne")}</span>
            <span class="h-1.5 flex-1 rounded-full bg-loom"><span class="block h-full rounded-full bg-qolle transition-[width] duration-300 motion-reduce:transition-none" style="width: {(start / scale) * 100}%"></span></span>
            <span class="w-12 shrink-0 text-right tabular-nums">{tokens(start)}</span>
          </div>
          <div class="flex items-center gap-3">
            <span class="w-44 shrink-0 whitespace-nowrap">{t("new.yours")}</span>
            <span class="h-1.5 flex-1 rounded-full bg-loom"><span class="block h-full rounded-full bg-fleece/60" style="width: {(info.average / scale) * 100}%"></span></span>
            <span class="w-12 shrink-0 text-right tabular-nums">{tokens(info.average)}</span>
          </div>
        </div>
      {/if}

      {#each groups as g (g.id)}
        {#if g.files.length}
          <section class="mt-7">
            <h3 class="text-sm font-semibold">{t(`new.group.${g.id}`)}</h3>
            <p class="mt-0.5 text-xs text-fleece">{t(`new.group.${g.id}Hint`)}</p>
            <ul class="mt-2 divide-y divide-warp border-y border-warp">
              {#each g.files as f (f.rel)}
                <li>
                  <button class="flex w-full items-start gap-2.5 py-2 text-left hover:bg-loom/60" aria-expanded={open === f.rel} onclick={() => (open = open === f.rel ? null : f.rel)}>
                    <span class="mt-0.5 text-fleece"><Icon name={open === f.rel ? "down" : "right"} size={14} /></span>
                    <span class="min-w-0 flex-1">
                      <span class="block truncate font-mono text-[12.5px] {f.existing ? 'text-fleece' : ''}">{f.rel}</span>
                      {#if f.about}<span class="block text-xs text-fleece">{t(`new.about.${f.about}`)}</span>{/if}
                      {#if f.existing}<span class="block text-xs text-fleece">{t("new.kept")}</span>
                      {:else if f.global}<span class="block text-xs text-indigo">{t("new.movedGlobal")}</span>
                      {:else if f.from}<span class="block truncate text-xs text-indigo">{t("new.copiedFrom", { path: f.from })}</span>{/if}
                    </span>
                    <span class="shrink-0 text-xs tabular-nums {g.id === 'always' ? 'text-qolle' : 'text-fleece'}">{cost(f)}</span>
                  </button>
                  {#if open === f.rel}
                    <pre transition:slide={{ duration: 150 }} class="mb-2 max-h-80 overflow-auto rounded-md border border-warp bg-loom px-3 py-2 font-mono text-[12px] leading-relaxed whitespace-pre-wrap">{f.content}</pre>
                  {/if}
                </li>
              {/each}
            </ul>
          </section>
        {/if}
      {/each}
    </aside>
  </div>
{/if}
