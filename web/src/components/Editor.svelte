<script lang="ts">
  import { api, ApiError } from "../lib/api/client";
  import type { EditorFile } from "../lib/api/types";
  import { notifyChanged } from "../lib/changes.svelte";
  import { closeEditor, editor } from "../lib/editor.svelte";
  import { tokens as fmtTokens } from "../lib/format";
  import { t } from "../lib/i18n.svelte";
  import { report } from "../lib/toast.svelte";
  import Icon from "./Icon.svelte";

  // Edits one instruction file. The token count follows the text as you type (estimated by quipu,
  // like everywhere else); saving goes through quipu's journal, so it can be undone.
  let { path }: { path: string } = $props();

  let file = $state<EditorFile | null>(null);
  let error = $state<unknown>(null);
  let text = $state("");
  let count = $state(0);
  let busy = $state(false);
  let conflict = $state(false);
  let area: HTMLTextAreaElement | undefined = $state();

  async function load() {
    try {
      file = await api.readFile(path);
      text = file.content;
      count = file.tokens;
      conflict = false;
      error = null;
    } catch (e) {
      error = e;
    }
  }
  $effect(() => {
    void path;
    load();
  });
  // Start at the top of the file with the cursor there.
  $effect(() => {
    if (!file || !area) return;
    area.focus();
    area.setSelectionRange(0, 0);
    area.scrollTop = 0;
  });

  const dirty = $derived(file !== null && text !== file.content);
  $effect(() => {
    editor.dirty = dirty;
  });

  // Count tokens shortly after typing stops.
  $effect(() => {
    const current = text;
    if (!file) return;
    const timer = setTimeout(async () => {
      try {
        count = (await api.tokens(current)).tokens;
      } catch {
        // keep the last count
      }
    }, 250);
    return () => clearTimeout(timer);
  });

  const delta = $derived(file ? count - file.tokens : 0);

  async function save(): Promise<boolean> {
    if (!file || !dirty || busy) return !dirty;
    busy = true;
    try {
      const res = await api.saveFile(file.path, text, file.version);
      const change = res.params?.change;
      file = { ...file, content: text, version: res.params?.version ?? file.version, tokens: count, exists: true };
      report(res, change ? { label: t("action.undo"), run: () => undo(change) } : undefined);
      notifyChanged();
      return true;
    } catch (e) {
      if (e instanceof ApiError && e.code === "file-changed") conflict = true;
      else report(e);
      return false;
    } finally {
      busy = false;
    }
  }

  async function undo(change: string) {
    try {
      report(await api.undo(change));
    } catch (e) {
      report(e);
    }
    notifyChanged();
    if (editor.path === path) load();
  }

  async function saveAndClose() {
    if (await save()) closeEditor(true);
  }

  function onkeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
      e.preventDefault();
      save();
    }
  }
</script>

{#if error}
  <p class="text-sm text-cochineal">{error instanceof ApiError ? t(`result.${error.code}`, error.params) : String(error)}</p>
{:else if file}
  <div class="flex h-full flex-col gap-3">
    <div class="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
      <p class="min-w-0 truncate font-mono text-[12.5px] text-fleece" title={file.path}>{file.path}</p>
      <p class="text-sm">
        <span class={file.always ? "text-qolle" : "text-fleece"}>{t("editor.tokens", { tokens: fmtTokens(count) })}</span>
        <span class="text-fleece">{file.always ? t("editor.always") : t("editor.onDemand")}</span>
        {#if delta !== 0}
          <span class="ml-1 {delta > 0 ? 'text-qolle' : 'text-chilca'}">{delta > 0 ? "+" : "−"}{fmtTokens(Math.abs(delta))}</span>
        {/if}
      </p>
    </div>
    {#if !file.exists}<p class="text-xs text-fleece">{t("editor.newFile")}</p>{/if}

    {#if conflict}
      <div class="flex flex-wrap items-center gap-3 rounded-md border border-cochineal/50 px-3 py-2 text-sm">
        <span class="flex-1">{t("editor.conflict")}</span>
        <button class="btn" onclick={load}>{t("editor.reload")}</button>
      </div>
    {/if}

    <textarea
      bind:this={area}
      bind:value={text}
      {onkeydown}
      spellcheck="false"
      aria-label={file.name}
      class="field min-h-[50vh] flex-1 resize-none font-mono text-[13px] leading-6"
    ></textarea>

    {#if editor.confirmClose}
      <div class="flex flex-wrap items-center gap-2 rounded-md border border-qolle/50 px-3 py-2 text-sm" role="alert">
        <span class="mr-auto">{t("editor.unsaved")}</span>
        <button class="btn btn-primary" onclick={saveAndClose} disabled={busy}>{t("editor.saveClose")}</button>
        <button class="btn" onclick={() => closeEditor(true)}>{t("editor.discard")}</button>
        <button class="btn btn-ghost" onclick={() => (editor.confirmClose = false)}>{t("editor.keepEditing")}</button>
      </div>
    {:else}
      <div class="flex items-center gap-3">
        <button class="btn btn-primary" onclick={save} disabled={!dirty || busy}><Icon name="check" />{t("editor.save")}</button>
        <button class="btn btn-ghost" onclick={() => (text = file?.content ?? text)} disabled={!dirty}>{t("editor.revert")}</button>
        <span class="ml-auto text-xs text-fleece">{t("editor.hint")}</span>
      </div>
    {/if}
  </div>
{/if}
