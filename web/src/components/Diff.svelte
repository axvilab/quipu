<script lang="ts">
  import type { DiffLine } from "../lib/api/types";
  let { lines }: { lines: DiffLine[] } = $props();
</script>

<pre class="overflow-x-auto rounded-md border border-warp bg-night py-2 font-mono text-[12.5px] leading-6"><code
    >{#each lines as l, i (i)}<span
        class="block px-3 {l.kind === 'add' ? 'bg-chilca/10 text-chilca' : l.kind === 'del' ? 'bg-cochineal/10 text-cochineal' : 'text-fleece'}"
        >{l.text}</span
      >{/each}</code
  ></pre>
