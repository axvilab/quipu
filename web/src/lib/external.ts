// In the native window (Wails, ADR 0007) a link that opens a new window would do nothing, so it
// goes to the system browser through the Wails runtime instead. In a browser tab nothing changes.

type WailsRuntime = { BrowserOpenURL(url: string): void };

export function external(node: HTMLAnchorElement) {
  const onclick = (e: MouseEvent) => {
    const runtime = (window as unknown as { runtime?: WailsRuntime }).runtime;
    if (!runtime) return;
    e.preventDefault();
    runtime.BrowserOpenURL(node.href);
  };
  node.addEventListener("click", onclick);
  return { destroy: () => node.removeEventListener("click", onclick) };
}
