// The editor sheet is opened from several screens, so its state lives here; App renders it.

export const editor = $state({ path: null as string | null, dirty: false, confirmClose: false });

export function openEditor(path: string) {
  editor.path = path;
  editor.dirty = false;
  editor.confirmClose = false;
}

/** closeEditor asks first when there are unsaved changes (inline, no browser dialog). */
export function closeEditor(force = false) {
  if (editor.dirty && !force) {
    editor.confirmClose = true;
    return;
  }
  editor.path = null;
  editor.dirty = false;
  editor.confirmClose = false;
}
