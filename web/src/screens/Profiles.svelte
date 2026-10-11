<script lang="ts">
  import { api } from "../lib/api/client";
  import type { ProfileInfo, Profiles } from "../lib/api/types";
  import { t } from "../lib/i18n.svelte";
  import { report } from "../lib/toast.svelte";
  import Header from "../components/Header.svelte";
  import Icon from "../components/Icon.svelte";
  import Loading from "../components/Loading.svelte";
  import Panel from "../components/Panel.svelte";

  let data = $state<Profiles | null>(null);
  let error = $state<unknown>(null);
  let adding = $state(false);
  let signingIn = $state<string | null>(null);
  let removing = $state<string | null>(null);

  // add form
  let name = $state("");
  let share = $state<string[]>([]);
  // sign-in form
  let email = $state("");
  let browser = $state("");
  // rule form
  let folder = $state("");
  let ruleProfile = $state("");

  async function load() {
    try {
      data = await api.profiles();
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

  async function act(p: Promise<unknown>, after?: () => void) {
    try {
      report(await p);
      after?.();
    } catch (e) {
      report(e);
    }
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
          <span class="block font-medium">{p.name}</span>
          <span class="block truncate text-sm text-fleece">
            {#if p.signIn.state}{t("profiles.waiting")}{:else if p.signedIn}{p.account}{p.plan ? ` · ${p.plan}` : ""}{:else}{t("profile.signedOut")}{/if}
          </span>
        </span>
        <span class="hidden min-w-0 font-mono text-xs text-fleece md:block">{p.dir}</span>
        <span class="hidden text-xs text-fleece lg:block">{p.shares.length ? t("profiles.shares", { items: p.shares.join(", ") }) : ""}</span>
        <span class="flex gap-2">
          <button class="btn" onclick={() => ((signingIn = p.name), (email = ""))}>{p.signedIn ? t("profiles.signInAgain") : t("profiles.signIn")}</button>
          {#if p.name !== "default"}
            {#if removing === p.name}
              <button class="btn border-cochineal text-cochineal" onclick={() => act(api.removeProfile(p.name), () => (removing = null))}>{t("profiles.confirmRemove")}</button>
              <button class="btn btn-ghost" onclick={() => (removing = null)}>{t("action.cancel")}</button>
            {:else}
              <button class="btn btn-ghost" onclick={() => (removing = p.name)}>{t("action.remove")}</button>
            {/if}
          {/if}
        </span>
      </li>
    {/each}
  </ul>
  {#if removing}<p class="mt-2 text-xs text-fleece">{t("profiles.removeNote")}</p>{/if}

  <section class="mt-10 max-w-3xl">
    <h2 class="mb-1 text-md font-semibold">{t("rules.title")}</h2>
    <p class="mb-3 text-sm text-fleece">{t("rules.note")}</p>
    <ul class="divide-y divide-warp">
      {#each data.rules as r (r.path)}
        <li class="flex items-center gap-3 py-2 text-sm">
          <Icon name="folder" size={14} />
          <span class="min-w-0 flex-1 truncate font-mono text-[12.5px]">{r.path}</span>
          <span class="text-fleece">{t("rules.uses")}</span>
          <span class="font-medium">{r.profile}</span>
          <button class="btn btn-ghost" onclick={() => act(api.deleteRule(r.path))}>{t("action.remove")}</button>
        </li>
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
        <select class="field" bind:value={ruleProfile}>{#each data.profiles as p (p.name)}<option>{p.name}</option>{/each}</select>
      </label>
      <button class="btn btn-primary h-9">{t("rules.save")}</button>
    </form>
    <p class="mt-4 text-xs text-fleece">{t("rules.alias")} <code class="font-mono text-cotton">alias claude='quipu run'</code></p>
  </section>
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
  <Panel title={t("signin.title", { name: signing.name })} onclose={() => (signingIn = null)}>
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
          <a class="btn" href={signing.signIn.url} target="_blank" rel="noopener"><Icon name="external" size={14} />{t("signin.here")}</a>
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
