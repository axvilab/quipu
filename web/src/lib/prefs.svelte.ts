// Per-browser view preferences. Shared data lives in quipu itself, not here.

const saved = (() => {
  try {
    return Number(localStorage.getItem("quipu.days")) || 30;
  } catch {
    return 30;
  }
})();

export const prefs = $state({ days: [7, 30, 90].includes(saved) ? saved : 30 });

export function setDays(d: number) {
  prefs.days = d;
  try {
    localStorage.setItem("quipu.days", String(d));
  } catch {
    // ignore
  }
}
