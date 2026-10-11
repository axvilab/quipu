// A small history router: the server answers every app path with index.html.

export type Route =
  | { name: "overview" }
  | { name: "project"; dir: string }
  | { name: "advice" }
  | { name: "cache" }
  | { name: "profiles" }
  | { name: "notFound" };

function parse(path: string): Route {
  const parts = path.split("/").filter(Boolean).map(decodeURIComponent);
  switch (parts[0]) {
    case undefined:
      return { name: "overview" };
    case "projects":
      return parts[1] ? { name: "project", dir: parts[1] } : { name: "overview" };
    case "advice":
      return { name: "advice" };
    case "cache":
      return { name: "cache" };
    case "profiles":
      return { name: "profiles" };
  }
  return { name: "notFound" };
}

export const router = $state({ route: parse(location.pathname) });

export function navigate(path: string) {
  if (path === location.pathname) return;
  history.pushState({}, "", path);
  router.route = parse(path);
  window.scrollTo(0, 0);
}

window.addEventListener("popstate", () => {
  router.route = parse(location.pathname);
});

export const projectPath = (dir: string) => `/projects/${encodeURIComponent(dir)}`;

/** link turns a plain <a href> into an in-app navigation (keeps middle-click and new tabs). */
export function link(node: Element) {
  const onclick = (e: Event) => {
    if (!(e instanceof MouseEvent)) return;
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    const href = node.getAttribute("href");
    if (!href || !href.startsWith("/")) return;
    e.preventDefault();
    navigate(href);
  };
  node.addEventListener("click", onclick);
  return { destroy: () => node.removeEventListener("click", onclick) };
}
