// Funções compartilhadas entre as páginas.
window.Finan = (function () {
  // postSSE faz um POST e chama onEvent para cada evento "data: {json}" recebido.
  // Rejeita se o servidor enviar {"error": "..."}.
  async function postSSE(url, body, onEvent) {
    const res = await fetch(url, { method: "POST", body });
    if (!res.ok) throw new Error((await res.text()) || res.statusText);
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buf = "";
    for (;;) {
      const { value, done } = await reader.read();
      if (done) return;
      buf += decoder.decode(value, { stream: true });
      let i;
      while ((i = buf.indexOf("\n\n")) >= 0) {
        const chunk = buf.slice(0, i);
        buf = buf.slice(i + 2);
        if (!chunk.startsWith("data: ")) continue;
        const ev = JSON.parse(chunk.slice(6));
        if (ev.error) throw new Error(ev.error);
        onEvent(ev);
      }
    }
  }

  // Markdown mínimo e seguro: escapa o HTML antes de aplicar a formatação.
  function markdown(text) {
    const esc = text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
    const inline = (s) => s
      .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
      .replace(/`([^`]+)`/g, "<code>$1</code>");
    let html = "", list = null;
    for (const line of esc.split("\n")) {
      const ul = line.match(/^\s*[-*]\s+(.*)/), ol = line.match(/^\s*\d+[.)]\s+(.*)/);
      const kind = ul ? "ul" : ol ? "ol" : null;
      if (list && kind !== list) { html += `</${list}>`; list = null; }
      if (kind) {
        if (!list) { html += `<${kind}>`; list = kind; }
        html += `<li>${inline((ul || ol)[1])}</li>`;
      } else if (/^#{1,4}\s/.test(line)) {
        html += `<h4>${inline(line.replace(/^#+\s/, ""))}</h4>`;
      } else if (line.trim() !== "") {
        html += `<p>${inline(line)}</p>`;
      }
    }
    if (list) html += `</${list}>`;
    return html;
  }

  function formatBytes(n) {
    return n >= 1e9 ? (n / 1e9).toFixed(1) + " GB" : Math.round(n / 1e6) + " MB";
  }

  // Botão "Baixar modelo": mostra o progresso e recarrega a página ao terminar.
  document.querySelectorAll("[data-pull]").forEach((button) => {
    const box = button.closest(".notice");
    const status = box.querySelector(".pull-status");
    const bar = box.querySelector(".pull-bar");
    button.addEventListener("click", async () => {
      button.disabled = true;
      bar.hidden = false;
      try {
        await postSSE("/configuracoes/baixar-modelo", null, (ev) => {
          if (ev.done) return;
          if (ev.total) {
            bar.value = Math.floor((ev.completed / ev.total) * 100);
            status.textContent = `Baixando: ${formatBytes(ev.completed || 0)} de ${formatBytes(ev.total)}`;
          } else {
            status.textContent = ev.status;
          }
        });
        status.textContent = "Pronto! Recarregando…";
        location.reload();
      } catch (err) {
        status.textContent = "Erro: " + err.message;
        button.disabled = false;
      }
    });
  });

  // PWA: o service worker só funciona em https ou no próprio computador
  // (localhost); pelo Wi-Fi em http o navegador ignora, e o app segue normal.
  // No app do celular as telas já vêm dentro do app: não precisa.
  const inApp = document.documentElement.dataset.app === "mobile";
  if ("serviceWorker" in navigator && window.isSecureContext && !inApp && location.pathname !== "/entrar") {
    navigator.serviceWorker.register("/sw.js").catch(() => {});
  }

  // Botão "Instalar app" (Chrome, Edge e Android avisam quando dá para instalar).
  const installButton = document.querySelector("[data-install]");
  let installPrompt = null;
  window.addEventListener("beforeinstallprompt", (e) => {
    e.preventDefault();
    installPrompt = e;
    if (installButton) installButton.hidden = false;
  });
  installButton?.addEventListener("click", async () => {
    if (!installPrompt) return;
    installPrompt.prompt();
    await installPrompt.userChoice;
    installPrompt = null;
    installButton.hidden = true;
  });

  // Formulários com data-confirm pedem confirmação antes de enviar (no lugar
  // de onsubmit="", que a CSP não deixa rodar).
  document.querySelectorAll("form[data-confirm]").forEach((form) => form.addEventListener("submit", (e) => {
    if (!confirm(form.dataset.confirm)) e.preventDefault();
  }));

  return { postSSE, markdown };
})();
