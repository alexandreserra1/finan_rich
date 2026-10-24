// Shared helpers for the finan_rich prototype: money format, theme tokens (dark + light),
// sidebar nav, debt plan (avalanche, mirrors finance.PayoffSchedule) and month labels.
(function () {
  const KEY = "finan-proto-tema";
  const R = (v, sign) => (sign && v > 0 ? "+" : v < 0 ? "−" : "") + "R$ " + Math.abs(v).toLocaleString("pt-BR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

  const DARK = {
    "--bg": "#0a0d0c", "--side": "#0c100f", "--surface": "#101614", "--surface-2": "#141c1a", "--hover": "#141c1a", "--inset": "#0c1110",
    "--border": "#1a2421", "--border-2": "#24302c", "--grid": "#18211e", "--track": "#1a2421",
    "--text": "#e8eeeb", "--text-2": "#b9c4bf", "--muted": "#8d9a95", "--faint": "#6c7a74",
    "--accent": "#34c08f", "--accent-hi": "#5fd6aa", "--accent-ink": "#05140f", "--accent-soft": "rgba(52,192,143,.13)", "--accent-line": "#1d3a30",
    "--pos": "#7fe0b8", "--neg": "#f07163", "--neg-soft": "rgba(240,113,99,.13)", "--warn": "#f0a845", "--warn-soft": "rgba(240,168,69,.13)",
    "--nec": "#5a93f0", "--des": "#f0a845", "--inv": "#34c08f", "--renda": "#a184f0",
    "--hero": "linear-gradient(160deg,#0f2a22 0%,#0e1714 60%)", "--hero-border": "#1d3a30", "--hero-muted": "#a9cbbf", "--hero-track": "#1a2a25",
    "--shadow": "0 1px 0 rgba(255,255,255,.02) inset", "--shadow-pop": "0 18px 40px -16px rgba(0,0,0,.7)", "--scrim": "rgba(3,6,5,.62)",
  };
  const LIGHT = {
    "--bg": "#f4f6f5", "--side": "#fbfcfb", "--surface": "#ffffff", "--surface-2": "#f3f6f4", "--hover": "#eef2f0", "--inset": "#f3f6f4",
    "--border": "#e2e7e4", "--border-2": "#d5ddd9", "--grid": "#edf1ef", "--track": "#e9eeeb",
    "--text": "#15201b", "--text-2": "#3d4843", "--muted": "#5f6b66", "--faint": "#77837d",
    "--accent": "#0f7a5c", "--accent-hi": "#0b5e47", "--accent-ink": "#ffffff", "--accent-soft": "#e3f2eb", "--accent-line": "#bfe0d2",
    "--pos": "#0f7a5c", "--neg": "#c2412f", "--neg-soft": "#fbecea", "--warn": "#a8650c", "--warn-soft": "#fdf3e2",
    "--nec": "#2f6fd6", "--des": "#bc7718", "--inv": "#0f9b6e", "--renda": "#7a5ad1",
    "--hero": "linear-gradient(160deg,#0f2e25 0%,#123a2e 100%)", "--hero-border": "#0f2e25", "--hero-muted": "#b9d6cb", "--hero-track": "#24574a",
    "--shadow": "0 1px 2px rgba(16,24,20,.04), 0 6px 20px -12px rgba(16,24,20,.12)", "--shadow-pop": "0 18px 40px -16px rgba(16,24,20,.28)", "--scrim": "rgba(16,24,20,.38)",
  };
  const pref = () => { try { return localStorage.getItem(KEY) || "escuro"; } catch { return "escuro"; } };
  const resolved = () => { const p = pref(); return p === "sistema" ? (matchMedia("(prefers-color-scheme: dark)").matches ? "escuro" : "claro") : p; };
  const setTheme = (t) => { try { localStorage.setItem(KEY, t); } catch {} window.dispatchEvent(new Event("fr-theme")); };
  const vars = (o) => Object.entries(o).map(([k, v]) => `${k}:${v}`).join(";");

  const ICON = {
    painel: "M3 11l9-7 9 7v9a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z",
    lanc: "M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01",
    contas: "M3 10l9-6 9 6M5 10v8M9.5 10v8M14.5 10v8M19 10v8M3 21h18",
    dividas: "M2 7a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2zM2 10h20",
    metas: "M4 22V4M4 4h12l-2 4 2 4H4",
    analise: "M3 3v18h18M7 15l4-4 3 3 5-6",
    chat: "M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z",
    ajustes: "M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 0 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 0 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 0 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 0 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z",
    sun: "M12 3v1M12 20v1M4.2 4.2l.7.7M19.1 19.1l.7.7M3 12h1M20 12h1M4.2 19.8l.7-.7M19.1 4.9l.7-.7M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8z",
    moon: "M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z",
  };
  const NAV = [
    ["Painel", "painel", "Painel.dc.html"], ["Lançamentos", "lanc", "Lancamentos.dc.html"], ["Contas", "contas", "Contas.dc.html"],
    ["Dívidas", "dividas", "Dividas.dc.html"], ["Metas", "metas", "Metas.dc.html"], ["Análise", "analise", "Analise.dc.html"],
    ["Consultor IA", "chat", "Consultor.dc.html"],
  ];
  const navStyle = (on) => `display:flex;align-items:center;gap:12px;padding:9px 10px;border-radius:9px;font-weight:500;text-decoration:none;transition:background-color .15s,color .15s;${on ? "background:var(--accent-soft);color:var(--text);box-shadow:inset 0 0 0 1px var(--accent-line)" : "color:var(--muted)"}`;
  function shell(active) {
    const dark = resolved() === "escuro";
    return {
      rootStyle: `${vars(dark ? DARK : LIGHT)};display:grid;grid-template-columns:240px minmax(0,1fr);min-height:100vh;background:var(--bg);color:var(--text);font-family:Geist,system-ui,sans-serif;font-size:14px;line-height:1.5;transition:background-color .3s ease,color .3s ease`,
      nav: NAV.map(([label, ic, href]) => ({ label, href, d: ICON[ic], style: navStyle(label === active) })),
      settingsStyle: navStyle(active === "Ajustes"), settingsIcon: ICON.ajustes,
      themeLabel: dark ? "Tema claro" : "Tema escuro", themeIcon: dark ? ICON.sun : ICON.moon,
      toggleTheme: () => setTheme(dark ? "claro" : "escuro"),
      isDark: dark,
    };
  }

  const DEBTS = [
    { id: "d1", name: "Rotativo cartão C6", creditor: "C6 Bank", balance: 2940, rate: 13.5, min: 420, due: 12 },
    { id: "d2", name: "Empréstimo pessoal", creditor: "Itaú", balance: 6820, rate: 3.2, min: 486, due: 28 },
    { id: "d3", name: "Financiamento moto", creditor: "Honda", balance: 2720, rate: 1.9, min: 310, due: 5 },
  ];
  function plan(debts, extra, minOnly) {
    const ds = debts.map((d) => ({ ...d }));
    const series = [ds.reduce((s, d) => s + d.balance, 0)];
    let interest = 0, months = 0; const paidOff = [];
    const base = debts.reduce((s, d) => s + d.min, 0) + (minOnly ? 0 : extra);
    while (ds.some((d) => d.balance > 0.005) && months < 120) {
      months++;
      ds.forEach((d) => { if (d.balance > 0) { const j = (d.balance * d.rate) / 100; interest += j; d.balance += j; } });
      let pool = base;
      ds.forEach((d) => { const p = Math.min(d.balance, d.min); d.balance -= p; pool -= p; });
      if (!minOnly) [...ds].sort((a, b) => b.rate - a.rate).forEach((d) => { const p = Math.min(d.balance, Math.max(0, pool)); d.balance -= p; pool -= p; });
      ds.forEach((d) => { if (d.balance <= 0.005 && !paidOff.find((p) => p.id === d.id)) { d.balance = 0; paidOff.push({ id: d.id, name: d.name, month: months }); } });
      series.push(ds.reduce((s, d) => s + d.balance, 0));
    }
    return { months, interest, series, paidOff, ok: series[series.length - 1] < 0.01 };
  }
  const MES = ["jan.", "fev.", "mar.", "abr.", "mai.", "jun.", "jul.", "ago.", "set.", "out.", "nov.", "dez."];
  const MESF = ["janeiro", "fevereiro", "março", "abril", "maio", "junho", "julho", "agosto", "setembro", "outubro", "novembro", "dezembro"];
  const monthAfter = (n, full) => { const t = 9 + n; const y = 2026 + Math.floor(t / 12); return full ? `${MESF[t % 12]} de ${y}` : `${MES[t % 12]} ${y}`; };
  const duration = (m) => { const y = Math.floor(m / 12), r = m % 12; const p = (n, a, b) => `${n} ${n === 1 ? a : b}`; return !y ? p(r, "mês", "meses") : r ? `${p(y, "ano", "anos")} e ${p(r, "mês", "meses")}` : p(y, "ano", "anos"); };

  window.FR = { R, shell, setTheme, pref, resolved, ICON, DEBTS, plan, monthAfter, duration, MES, MESF };
})();
