// Screens reload when quipu changes files from somewhere else on the page (the editor, an undo
// in a toast), so a fixed finding disappears without a manual refresh.

const event = "quipu:changed";

export function notifyChanged() {
  window.dispatchEvent(new Event(event));
}

/** onChanged calls reload after any change; use it in a component's script. */
export function onChanged(reload: () => void) {
  $effect(() => {
    window.addEventListener(event, reload);
    return () => window.removeEventListener(event, reload);
  });
}
