<script lang="ts">
  import { external } from "../lib/external";
  import { api } from "../lib/api/client";
  import type { ProfileInfo, Profiles, Result } from "../lib/api/types";
  import { t } from "../lib/i18n.svelte";
  import { notifyChanged } from "../lib/changes.svelte";
  import { link } from "../lib/router.svelte";
  import { report } from "../lib/toast.svelte";
  import Header from "../components/Header.svelte";
  import Icon from "../components/Icon.svelte";
  import Loading from "../components/Loading.svelte";
  import Panel from "../components/Panel.svelte";
  import Switch from "../components/Switch.svelte";

  let data = $state<Profiles | null>(null);
  let error = $state<unknown>(null);
  let adding = $state(false);
  let signingIn = $state<string | null>(null);
  let removing = $state<string | null>(null);
  let editing = $state<string | null>(null);
  let label = $state("");

  // add form
  let name = $state("");
  let share = $state<string[]>([]);
  // sign-in form
  let email = $state("");
  let browser = $state("");
  // rule form
  let folder = $state("");
  let ruleProfile = $state("");

  // The sign-in state of each profile, so the rest of the page (the sidebar) learns when it changes.
  const states = (d: Profiles | null) => d?.profiles.map((p) => `${p.name}:${p.signedIn}`).join(",") ?? "";

  async function load() {
    try {
      const before = states(data);
      data = await api.profiles();
      if (before && before !== states(data)) notifyChanged();
      error = null;
      if (!share.length) share = [...data.shareable];
      if (!browser) browser = data.browsers[0]?.id ?? "";
      if (!ruleProfile) ruleProfile = data.profiles[0]?.name ?? "";
    } catch (e) {
      error = e;
    }
  }
  load();

  // While a sign-in is running, poll until Claude finishes it.
  const pending = $derived(data?.profiles.some((p) => p.signIn.state) ?? false);
  $effect(() => {
    if (!pending) return;
    const id = setInterval(load, 2000);
    return () => clearInterval(id);
  });

  const signing = $derived<ProfileInfo | undefined>(data?.profiles.find((p) => p.name === signingIn));
  const edited = $derived<ProfileInfo | undefined>(data?.profiles.find((p) => p.name === editing));

  function openEdit(p: ProfileInfo) {
    editing = p.name;
    label = p.label ?? "";
    removing = null;
  }

  async function act(p: Promise<unknown>, after?: () => void) {
    try {
      report(await p);
      after?.();
    } catch (e) {
      report(e);
    }
    load();
  }

  // A change written through the journal: the toast offers to undo it.
  async function change(p: Promise<Result>) {
    try {
      const res = await p;
      const id = res.params?.change;
      report(res, id ? { label: t("action.undo"), run: () => act(api.undo(id)) } : undefined);
    } catch (e) {
      report(e);
    }
    notifyChanged();
    load();
  }

  const add = () => act(api.addProfile(name, share), () => ((adding = false), (signingIn = name), (name = "")));
  const startSignIn = (priv: boolean) => act(api.signIn(signingIn!, { email, browser, private: priv }));
</script>

<Header title={t("profiles.title")} subtitle={t("profiles.subtitle")}>
  {#snippet actions()}
    <button class="btn btn-primary" onclick={() => (adding = true)}><Icon name="plus" />{t("profiles.add")}</button>
  {/snippet}
</Header>

{#if !data}
  <Loading {error} />
{:else}
  <ul class="divide-y divide-warp border-y border-warp">
    {#each data.profiles as p (p.name)}
      <li class="flex flex-wrap items-center gap-4 py-3">
        <span class="h-2.5 w-2.5 shrink-0 rounded-full {p.signedIn ? 'bg-chilca' : 'bg-warp'}" aria-hidden="true"></span>
        <span class="min-w-0 flex-1">
          <span class="block">
            <span class="font-medium">{p.title}</span>
            {#if p.label}<span class="ml-1.5 text-xs text-fleece">{p.name}</span>{/if}
          </span>
          <span class="block truncate text-sm text-fleece">
            {#if p.signIn.state}{t("profiles.waiting")}{:else if p.signedIn}{p.account}{p.plan ? ` · ${p.plan}` : ""}{:else}{t("profile.signedOut")}{/if}
          </span>
          <span class="block truncate text-xs text-fleece">
            {[p.dir, p.shares.length ? t("profiles.shares", { items: p.shares.join(", ") }) : "", p.statusLine === "quipu" ? t("profiles.statusOn") : ""].filter(Boolean).join(" · ")}
          </span>
        </span>
        <span class="flex gap-2">
          {#if !p.signedIn}<button class="btn" onclick={() => ((signingIn = p.name), (email = ""))}>{t("profiles.signIn")}</button>{/if}
          <button class="btn btn-ghost btn-icon" onclick={() => openEdit(p)} aria-label={t("profiles.edit")} title={t("profiles.edit")}><Icon name="gear" size={16} /></button>
        </span>
      </li>
    {/each}
  </ul>

  <section class="mt-10 max-w-3xl">
    <h2 class="mb-1 text-md font-semibold">{t("rules.title")}</h2>
    <p class="mb-3 text-sm text-fleece">{t("rules.note")}</p>
    <ul class="divide-y divide-warp">
      {#each data.rules as r (r.path)}
        <li class="flex items-center gap-3 py-2 text-sm">
          <Icon name="folder" size={14} />
          <span class="min-w-0 flex-1 truncate font-mono text-[12.5px]">{r.path}</span>
          <span class="text-fleece">{t("rules.uses")}</span>
          <span class="font-medium">{data.profiles.find((p) => p.name === r.profile)?.title ?? r.profile}</span>
          <button class="btn btn-ghost btn-icon" onclick={() => act(api.deleteRule(r.path))} aria-label={t("rules.remove")} title={t("rules.remove")}><Icon name="trash" size={15} /></button>
        </li>
        {#if r.memory.shared || r.memory.sourceFiles}
          <li class="flex flex-wrap items-center gap-3 border-t-0 pb-2.5 pl-6 text-xs text-fleece">
            {#if r.memory.shared}
              <span class="flex-1">{t("rules.memoryShared", { from: r.memory.from || "?" })}</span>
              <button class="btn btn-ghost" onclick={() => change(api.shareMemory(r.path, false))}>{t("rules.memoryUnshare")}</button>
            {:else}
              <span class="flex-1">{t("rules.memoryOther", { from: r.memory.source ?? "", count: r.memory.sourceFiles, name: r.profile })}</span>
              <button class="btn" onclick={() => change(api.shareMemory(r.path, true))}>{t("rules.memoryShare")}</button>
            {/if}
          </li>
        {/if}
      {:else}
        <li class="py-2 text-sm text-fleece">{t("rules.empty")}</li>
      {/each}
    </ul>
    <form class="mt-4 flex flex-wrap items-end gap-3" onsubmit={(e) => { e.preventDefault(); act(api.setRule(folder, ruleProfile), () => (folder = "")); }}>
      <label class="flex min-w-60 flex-1 flex-col gap-1 text-xs text-fleece">{t("rules.folder")}
        <input class="field font-mono" list="folders" bind:value={folder} required placeholder="~/work" />
      </label>
      <datalist id="folders">{#each data.folders as f (f)}<option value={f}></option>{/each}</datalist>
      <label class="flex flex-col gap-1 text-xs text-fleece">{t("rules.profile")}
        <select class="field" bind:value={ruleProfile}>{#each data.profiles as p (p.name)}<option value={p.name}>{p.title}</option>{/each}</select>
      </label>
      <button class="btn btn-primary h-9">{t("rules.save")}</button>
    </form>
    <p class="mt-4 text-xs text-fleece">{t("rules.shim")} <a href="/settings" use:link class="text-indigo hover:underline">{t("rules.shimLink")}</a></p>
  </section>
{/if}

{#if editing && edited}
  {@const p = edited}
  <Panel title={t("profiles.editTitle", { name: p.title })} onclose={() => ((editing = null), (removing = null))}>
    <div class="flex flex-col gap-7 text-sm">
      <form class="flex flex-col gap-1" onsubmit={(e) => { e.preventDefault(); change(api.setProfileLabel(p.name, label)); }}>
        <label for="profile-label">{t("profiles.label")}</label>
        <div class="flex gap-2">
          <input id="profile-label" class="field flex-1" bind:value={label} maxlength="40" placeholder={p.name} data-autofocus />
          <button class="btn" disabled={label.trim() === (p.label ?? "")}>{t("action.save")}</button>
        </div>
        <span class="text-xs text-fleece">{t("profiles.labelNote", { name: p.name, dir: p.dir })}</span>
      </form>

      <section>
        <h3 class="font-semibold">{t("profiles.account")}</h3>
        <p class="mt-1 text-fleece">{p.signedIn ? `${p.account}${p.plan ? ` · ${p.plan}` : ""}` : t("profile.signedOut")}</p>
        <button class="btn mt-2" onclick={() => ((signingIn = p.name), (email = ""), (editing = null))}>{p.signedIn ? t("profiles.signInAgain") : t("profiles.signIn")}</button>
      </section>

      {#if p.name !== "default"}
        <section>
          <h3 class="font-semibold">{t("profiles.shareTitle")}</h3>
          <p class="mt-1 text-xs text-fleece">{t("profiles.shareEditNote")}</p>
          <ul class="mt-2 divide-y divide-warp border-y border-warp">
            {#each data?.shareable ?? [] as item (item)}
              <li class="flex items-center gap-3 py-2">
                <Switch on={p.shares.includes(item)} label={item} onchange={(on) => change(api.shareItem(p.name, item, on))} />
                <span class="font-mono text-[12.5px]">{item}</span>
              </li>
            {/each}
          </ul>
        </section>
      {/if}

      <section>
        <h3 class="font-semibold">{t("statusline.title")}</h3>
        <p class="mt-1 text-xs text-fleece">{t("statusline.note")}</p>
        <pre class="mt-2 whitespace-pre-wrap rounded-md border border-warp bg-loom px-3 py-2 font-mono text-[11.5px] leading-relaxed text-fleece"><span class="text-cotton">{p.title}</span> · Opus · shop ⎇ main · $0.42 · ctx <span class="text-chilca">▰▰▰▱▱▱▱▱ 34%</span> 68K/200K · 5h <span class="text-qolle">▰▰▰▱▱ 67%</span> ↻14:50 · 7d <span class="text-chilca">▰▰▱▱▱ 41%</span> · cache 91% <span class="text-chilca">⏳59m</span></pre>
        <div class="mt-2 flex items-center gap-3">
          <Switch on={p.statusLine === "quipu"} label={t("statusline.switch", { name: p.title })} onchange={(on) => change(api.setStatusLine(p.name, on))} />
          <span class="min-w-0 flex-1 truncate text-xs text-fleece" title={p.statusCommand ?? ""}>
            {p.statusLine === "quipu" ? t("statusline.on") : p.statusLine === "other" ? t("statusline.other", { command: p.statusCommand ?? "" }) : t("statusline.none")}
          </span>
        </div>
      </section>

      {#if p.name !== "default"}
        <section class="border-t border-warp pt-5">
          {#if removing === p.name}
            <p class="mb-2 text-xs text-fleece">{t("profiles.removeNote")}</p>
            <div class="flex gap-2">
              <button class="btn border-cochineal text-cochineal" onclick={() => act(api.removeProfile(p.name), () => ((removing = null), (editing = null)))}>{t("profiles.confirmRemove")}</button>
              <button class="btn btn-ghost" onclick={() => (removing = null)}>{t("action.cancel")}</button>
            </div>
          {:else}
            <button class="btn btn-ghost text-cochineal" onclick={() => (removing = p.name)}>{t("profiles.removeProfile")}</button>
          {/if}
        </section>
      {/if}
    </div>
  </Panel>
{/if}

{#if adding && data}
  <Panel title={t("profiles.add")} onclose={() => (adding = false)}>
    <form class="flex flex-col gap-5" onsubmit={(e) => { e.preventDefault(); add(); }}>
      <label class="flex flex-col gap-1 text-sm">{t("profiles.name")}
        <input class="field" bind:value={name} required pattern="[a-z0-9][a-z0-9-]{'{0,31}'}" placeholder="work" data-autofocus />
        <span class="text-xs text-fleece">{t("profiles.nameNote")}</span>
      </label>
      <fieldset class="flex flex-col gap-2">
        <legend class="mb-1 text-sm">{t("profiles.share")}</legend>
        {#each data.shareable as item (item)}
          <label class="flex items-center gap-2 text-sm"><input type="checkbox" value={item} bind:group={share} class="accent-indigo" /> <span class="font-mono text-[12.5px]">{item}</span></label>
        {/each}
        <p class="text-xs text-fleece">{t("profiles.shareNote")}</p>
      </fieldset>
      <div><button class="btn btn-primary">{t("profiles.create")}</button></div>
    </form>
  </Panel>
{/if}

{#if signingIn && signing}
  <Panel title={t("signin.title", { name: signing.title })} onclose={() => (signingIn = null)}>
    {#if signing.signIn.state === "open"}
      <ol class="flex flex-col gap-4 text-sm">
        <li>{t("signin.step.open")}</li>
        <li class="flex flex-wrap gap-2">
          {#if data?.browsers.length}
            <button class="btn btn-primary" onclick={() => act(api.openPrivate(signing.name, browser))}>{t("signin.private")}</button>
            <select class="field w-auto" bind:value={browser} aria-label={t("signin.browser")}>
              {#each data.browsers as b (b.id)}<option value={b.id}>{b.name}{b.default ? ` (${t("signin.default")})` : ""}</option>{/each}
            </select>
          {/if}
          <a class="btn" href={signing.signIn.url} target="_blank" rel="noopener" use:external><Icon name="external" size={14} />{t("signin.here")}</a>
          <button class="btn" onclick={() => navigator.clipboard.writeText(signing.signIn.url ?? "").then(() => report({ ok: true, code: "copied" }))}><Icon name="copy" size={14} />{t("signin.copy")}</button>
        </li>
        <li class="text-fleece">{t("signin.step.wait")}</li>
      </ol>
    {:else if signing.signIn.state === "starting"}
      <p class="text-sm text-fleece">{t("signin.starting")}</p>
    {:else if signing.signedIn && !email}
      <p class="mb-4 text-sm">{t("signin.signedAs", { account: signing.account })}</p>
      <button class="btn" onclick={() => (signingIn = null)}>{t("action.close")}</button>
    {/if}
    {#if !signing.signIn.state}
      <form class="mt-2 flex flex-col gap-5" onsubmit={(e) => { e.preventDefault(); startSignIn(true); }}>
        <p class="text-sm leading-relaxed text-fleece">{t("signin.why")}</p>
        <label class="flex flex-col gap-1 text-sm">{t("signin.email")}
          <input class="field" type="email" bind:value={email} placeholder="me@company.com" data-autofocus />
          <span class="text-xs text-fleece">{t("signin.emailNote")}</span>
        </label>
        {#if data?.browsers.length}
          <label class="flex flex-col gap-1 text-sm">{t("signin.browser")}
            <select class="field" bind:value={browser}>
              {#each data.browsers as b (b.id)}<option value={b.id}>{b.name}{b.default ? ` (${t("signin.default")})` : ""}</option>{/each}
            </select>
          </label>
        {/if}
        <div class="flex gap-2">
          <button class="btn btn-primary">{t("signin.start")}</button>
          <button type="button" class="btn" onclick={() => startSignIn(false)}>{t("signin.startHere")}</button>
        </div>
      </form>
    {/if}
  </Panel>
{/if}
