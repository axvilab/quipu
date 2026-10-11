// Colour theme: follow the system by default, or keep the one the person picked (per browser).
// index.html applies the saved choice before the first paint, so there is no flash.

export type Theme = "system" | "light" | "dark";
export const themes: Theme[] = ["system", "light", "dark"];

const key = "quipu.theme";
const systemLight = matchMedia("(prefers-color-scheme: light)");

function saved(): Theme {
  try {
    const v = localStorage.getItem(key);
    return v === "light" || v === "dark" ? v : "system";
  } catch {
    return "system";
  }
}

export const theme = $state({ choice: saved() });

function apply() {
  const light = theme.choice === "light" || (theme.choice === "system" && systemLight.matches);
  document.documentElement.dataset.theme = light ? "light" : "dark";
}

export function setTheme(t: Theme) {
  theme.choice = t;
  try {
    localStorage.setItem(key, t);
  } catch {
    // ignore
  }
  apply();
}

systemLight.addEventListener("change", apply);
apply();
