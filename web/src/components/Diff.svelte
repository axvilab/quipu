<script lang="ts">
  import type { DiffLine } from "../lib/api/types";
  let { lines }: { lines: DiffLine[] } = $props();

  // Line numbers as in a unified diff: a hunk header gives the first line; removed lines count in
  // the old file, added lines in the new one, context lines in both.
  type Row = DiffLine & { n: number | null; showPath: boolean };
  const rows = $derived.by(() => {
    const out: Row[] = [];
    let oldN = 0;
    let newN = 0;
    let lastPath = "";
    for (const l of lines) {
      if (l.kind === "file") {
        oldN = newN = l.line ?? 0;
        out.push({ ...l, n: null, showPath: l.text !== lastPath });
        lastPath = l.text;
        continue;
      }
      let n: number | null = null;
      if (l.kind === "del") n = oldN ? oldN++ : null;
      else if (l.kind === "add") n = newN ? newN++ : null;
      else if (oldN) {
        n = newN;
        oldN++;
        newN++;
      }
      out.push({ ...l, n, showPath: false });
    }
    return out;
  });
  const numbered = $derived(rows.some((r) => r.n !== null));
</script>

<pre class="overflow-x-auto rounded-md border border-warp bg-night py-2 font-mono text-[12.5px] leading-6"><code
    >{#each rows as r, i (i)}{#if r.kind === "file"}<span
          class="block px-3 text-cotton {i ? 'mt-2 border-t border-warp pt-2' : ''} {r.showPath ? '' : 'h-0 overflow-hidden !p-0 !mt-1'}"
          >{r.showPath ? r.text : ""}</span
        >{:else}<span
          class="flex {r.kind === 'add' ? 'bg-chilca/10 text-chilca' : r.kind === 'del' ? 'bg-cochineal/10 text-cochineal' : 'text-fleece'}"
          >{#if numbered}<span class="w-12 shrink-0 select-none pr-3 text-right opacity-60" aria-hidden="true">{r.n ?? ""}</span
            >{/if}<span class="whitespace-pre {numbered ? 'pr-3' : 'px-3'}">{r.text}</span></span
        >{/if}{/each}</code
  ></pre>
