// Per-browser view preferences. Shared data lives in quipu itself, not here.

const saved = (() => {
  try {
    return Number(localStorage.getItem("quipu.days")) || 30;
  } catch {
    return 30;
  }
})();

const accountsOpen = (() => {
  try {
    return localStorage.getItem("quipu.accountsOpen") === "1";
  } catch {
    return false;
  }
})();

export const prefs = $state({ days: [7, 30, 90].includes(saved) ? saved : 30, accountsOpen });

/** setAccountsOpen remembers whether the sidebar shows the accounts in full. */
export function setAccountsOpen(open: boolean) {
  prefs.accountsOpen = open;
  try {
    localStorage.setItem("quipu.accountsOpen", open ? "1" : "0");
  } catch {
    // ignore
  }
}

export function setDays(d: number) {
  prefs.days = d;
  try {
    localStorage.setItem("quipu.days", String(d));
  } catch {
    // ignore
  }
}
