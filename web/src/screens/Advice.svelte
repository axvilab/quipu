<script lang="ts">
  import { api } from "../lib/api/client";
  import type { Advice, Finding } from "../lib/api/types";
  import { dateTime } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { report } from "../lib/toast.svelte";
  import FindingDetail from "../components/FindingDetail.svelte";
  import FindingList from "../components/FindingList.svelte";
  import Header from "../components/Header.svelte";
  import Icon from "../components/Icon.svelte";
  import Loading from "../components/Loading.svelte";
  import Panel from "../components/Panel.svelte";

  let data = $state<Advice | null>(null);
  let error = $state<unknown>(null);
  let open = $state<Finding | null>(null);

  async function load() {
    try {
      data = await api.advice();
      error = null;
    } catch (e) {
      error = e;
    }
  }
  load();

  async function undo(id: string) {
    try {
      report(await api.undo(id));
    } catch (e) {
      report(e);
    }
    load();
  }

  const user = $derived(data?.findings.filter((f) => f.scope === "user") ?? []);
  const projects = $derived(data?.findings.filter((f) => f.scope !== "user") ?? []);
</script>

<Header title={t("advice.title")} subtitle={t("advice.subtitle")} />

{#if !data}
  <Loading {error} />
{:else}
  {#if !data.findings.length}
    <p class="rounded-md border border-warp px-4 py-4 text-sm text-fleece">{t("overview.allGood")}</p>
  {/if}
  {#if user.length}
    <section class="mb-8">
      <h2 class="mb-1 text-md font-semibold">{t("advice.user")}</h2>
      <p class="mb-2 text-xs text-fleece">{t("advice.userNote")}</p>
      <FindingList findings={user} onopen={(f) => (open = f)} />
    </section>
  {/if}
  {#if projects.length}
    <section class="mb-8">
      <h2 class="mb-1 text-md font-semibold">{t("advice.projects")}</h2>
      <p class="mb-2 text-xs text-fleece">{t("advice.projectsNote")}</p>
      <FindingList findings={projects} onopen={(f) => (open = f)} />
    </section>
  {/if}

  {#if data.changes.length}
    <section class="mt-10">
      <h2 class="mb-2 text-md font-semibold">{t("advice.history")}</h2>
      <ul class="divide-y divide-warp">
        {#each data.changes as c (c.id)}
          <li class="flex items-center gap-4 py-2.5 text-sm">
            <span class="h-2 w-2 shrink-0 rounded-full {c.undone ? 'bg-warp' : 'bg-chilca'}" aria-hidden="true"></span>
            <span class="min-w-0 flex-1">
              <span class="block truncate">{c.title}</span>
              <span class="block truncate font-mono text-xs text-fleece">{c.path}</span>
            </span>
            <span class="shrink-0 text-xs text-fleece">{dateTime(c.time)}</span>
            {#if c.undone}
              <span class="w-24 shrink-0 text-right text-xs text-fleece">{t("advice.undone")}</span>
            {:else}
              <button class="btn w-24 shrink-0 justify-center" onclick={() => undo(c.id)}><Icon name="undo" size={14} />{t("action.undo")}</button>
            {/if}
          </li>
        {/each}
      </ul>
    </section>
  {/if}
{/if}

{#if open}
  <Panel title={t("finding.panelTitle")} onclose={() => (open = null)}>
    <FindingDetail finding={open} onapplied={() => { open = null; load(); }} />
  </Panel>
{/if}
