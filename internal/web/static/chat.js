// Chat com streaming: envia o formulário e lê a resposta em Server-Sent Events.
(function () {
  const messages = document.getElementById("messages");
  const form = document.getElementById("chat-form");
  const input = form.elements.message;

  // Markdown mínimo e seguro: escapa o HTML antes de aplicar a formatação.
  function md(text) {
    const esc = text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
    const lines = esc.split("\n");
    let html = "", list = null;
    const inline = (s) => s
      .replace(/\*\*(.+?)\*\*/g, "<strong>$1</strong>")
      .replace(/`([^`]+)`/g, "<code>$1</code>");
    for (const line of lines) {
      const ul = line.match(/^\s*[-*]\s+(.*)/), ol = line.match(/^\s*\d+[.)]\s+(.*)/);
      const kind = ul ? "ul" : ol ? "ol" : null;
      if (list && kind !== list) { html += `</${list}>`; list = null; }
      if (kind) {
        if (!list) { html += `<${kind}>`; list = kind; }
        html += `<li>${inline((ul || ol)[1])}</li>`;
      } else if (/^#{1,4}\s/.test(line)) {
        html += `<h4>${inline(line.replace(/^#+\s/, ""))}</h4>`;
      } else if (line.trim() === "") {
        html += "<br>";
      } else {
        html += `<p>${inline(line)}</p>`;
      }
    }
    if (list) html += `</${list}>`;
    return html;
  }

  function render(el) { el.innerHTML = md(el.dataset.raw ?? el.textContent); }
  document.querySelectorAll("[data-md]").forEach((el) => { el.dataset.raw = el.textContent; render(el); });

  function add(role, text) {
    document.querySelector(".suggestions")?.remove();
    const el = document.createElement("div");
    el.className = "msg " + role;
    el.dataset.raw = text;
    render(el);
    messages.appendChild(el);
    el.scrollIntoView({ block: "end" });
    return el;
  }

  document.querySelectorAll(".suggestions button").forEach((b) =>
    b.addEventListener("click", () => { input.value = b.textContent; form.requestSubmit(); }));

  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); form.requestSubmit(); }
  });

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const text = input.value.trim();
    if (!text) return;
    const button = form.querySelector("button");
    button.disabled = true;
    input.value = "";
    add("user", text);
    const reply = add("assistant pending", "Pensando…");
    let answer = "";
    try {
      const res = await fetch("/chat/enviar", { method: "POST", body: new URLSearchParams({ message: text }) });
      if (!res.ok) throw new Error(await res.text());
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });
        let i;
        while ((i = buf.indexOf("\n\n")) >= 0) {
          const chunk = buf.slice(0, i); buf = buf.slice(i + 2);
          if (!chunk.startsWith("data: ")) continue;
          const ev = JSON.parse(chunk.slice(6));
          if (ev.error) throw new Error(ev.error);
          if (ev.text) {
            answer += ev.text;
            reply.classList.remove("pending");
            reply.dataset.raw = answer;
            render(reply);
            reply.scrollIntoView({ block: "end" });
          }
        }
      }
    } catch (err) {
      reply.className = "msg error";
      reply.dataset.raw = "Erro: " + err.message + "\n\n(Sua pergunta não foi salva. Tente de novo.)";
      render(reply);
    } finally {
      button.disabled = false;
      input.focus();
    }
  });
})();
