<script lang="ts">
  // An on/off switch: a button with role="switch", reachable and toggled from the keyboard.
  let { on, label, disabled = false, onchange }: { on: boolean; label: string; disabled?: boolean; onchange: (on: boolean) => void } = $props();
</script>

<button
  type="button"
  role="switch"
  aria-checked={on}
  aria-label={label}
  {disabled}
  class="relative inline-flex h-5 w-9 shrink-0 items-center rounded-full border transition-colors motion-reduce:transition-none disabled:opacity-50 {on ? 'border-indigo bg-indigo' : 'border-warp bg-loom'}"
  onclick={() => onchange(!on)}
>
  <span class="inline-block h-3.5 w-3.5 rounded-full shadow transition-transform motion-reduce:transition-none {on ? 'translate-x-[1.1rem] bg-night' : 'translate-x-0.5 bg-fleece'}"></span>
</button>
