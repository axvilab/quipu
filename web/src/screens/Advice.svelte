<script lang="ts">
  import { api } from "../lib/api/client";
  import type { Advice, Finding } from "../lib/api/types";
  import { onChanged } from "../lib/changes.svelte";
  import { t } from "../lib/i18n.svelte";
  import { tokens } from "../lib/format";
  import ChangeList from "../components/ChangeList.svelte";
  import FindingDetail from "../components/FindingDetail.svelte";
  import FindingGroups from "../components/FindingGroups.svelte";
  import Header from "../components/Header.svelte";
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

  onChanged(load);

  const saves = $derived(data?.findings.reduce((n, f) => n + f.saves, 0) ?? 0);
</script>

<Header title={t("advice.title")} subtitle={t("advice.subtitle")} />

{#if !data}
  <Loading {error} />
{:else}
  {#if !data.findings.length}
    <p class="rounded-md border border-warp px-4 py-4 text-sm text-fleece">{t("overview.allGood")}</p>
  {/if}
  {#if data.findings.length}
    <p class="mb-4 text-sm {saves ? 'text-cotton' : 'text-fleece'}">
      {saves ? t("advice.saves", { tokens: tokens(saves) }) : t("advice.savesNone")}
    </p>
    <FindingGroups findings={data.findings} onopen={(f) => (open = f)} onapplied={load} />
  {/if}

  {#if data.changes.length}
    <section class="mt-10">
      <h2 class="mb-2 text-md font-semibold">{t("advice.history")}</h2>
      <ChangeList changes={data.changes} />
    </section>
  {/if}
{/if}

{#if open}
  <Panel title={t("finding.panelTitle")} onclose={() => (open = null)}>
    <FindingDetail finding={open} onapplied={() => { open = null; load(); }} />
  </Panel>
{/if}
