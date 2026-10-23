// Tema da tela: "auto" (igual ao aparelho), "claro" ou "escuro". A escolha
// vale só para este aparelho (localStorage) e não vai na sincronização.
// Carregado no <head>, antes de desenhar, para a tela não piscar no tema errado.
(function () {
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
    document.querySelector('meta[name="theme-color"]')?.setAttribute("content", dark ? "#0e1412" : "#0f7a5c");
    document.querySelectorAll("[data-theme-pick]").forEach((s) => { s.value = t; });
  };
  window.finanTheme = { get };
  media.addEventListener("change", apply);
  // Qualquer <select data-theme-pick> da tela troca o tema (Ajustes no computador e no app).
  document.addEventListener("change", (e) => {
    if (!e.target.matches?.("[data-theme-pick]")) return;
    try {
      localStorage.setItem("tema", e.target.value);
    } catch {}
    apply();
  });
  document.addEventListener("DOMContentLoaded", apply);
  apply();
})();
