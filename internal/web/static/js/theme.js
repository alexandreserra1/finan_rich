// Tema da tela: "auto" (igual ao aparelho), "claro" ou "escuro". A escolha
// vale só para este aparelho (localStorage) e não vai na sincronização.
// Carregado no <head>, antes de desenhar, para a tela não piscar no tema errado.
(function () {
  const SUN = "M12 3v1M12 20v1M4.2 4.2l.7.7M19.1 19.1l.7.7M3 12h1M20 12h1M4.2 19.8l.7-.7M19.1 4.9l.7-.7M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8z";
  const MOON = "M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z";
  const media = matchMedia("(prefers-color-scheme: dark)");
  const get = () => {
    try {
      return localStorage.getItem("tema") || "auto";
    } catch {
      return "auto"; // navegador sem localStorage (janela anônima bloqueada)
    }
  };
  const apply = () => {
    const t = get();
    const dark = t === "escuro" || (t === "auto" && media.matches);
    document.documentElement.dataset.theme = dark ? "dark" : "light";
    document.querySelector('meta[name="theme-color"]')?.setAttribute("content", dark ? "#0a0d0c" : "#f4f6f5");
    // Ajustes: cartões de rádio (um por tema) ou um <select>.
    document.querySelectorAll("[data-theme-pick]").forEach((s) => { if (s.type === "radio") s.checked = s.value === t; else s.value = t; });
    // Botão do menu lateral: mostra para qual tema ele troca.
    document.querySelectorAll("[data-theme-label]").forEach((el) => { el.textContent = dark ? "Tema claro" : "Tema escuro"; });
    document.querySelectorAll("[data-theme-icon]").forEach((el) => el.setAttribute("d", dark ? SUN : MOON));
  };
  window.finanTheme = { get };
  media.addEventListener("change", apply);
  // Qualquer campo data-theme-pick da tela troca o tema (Ajustes no computador e no app).
  document.addEventListener("change", (e) => {
    if (!e.target.matches?.("[data-theme-pick]")) return;
    try {
      localStorage.setItem("tema", e.target.value);
    } catch {}
    apply();
  });
  // O botão do menu troca entre claro e escuro (sai do automático).
  document.addEventListener("click", (e) => {
    if (!e.target.closest?.("[data-theme-toggle]")) return;
    const dark = document.documentElement.dataset.theme === "dark";
    try {
      localStorage.setItem("tema", dark ? "claro" : "escuro");
    } catch {}
    apply();
  });
  document.addEventListener("DOMContentLoaded", apply);
  apply();
})();
