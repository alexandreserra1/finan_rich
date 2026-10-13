// Chat com streaming: envia a pergunta e mostra a resposta conforme ela chega.
(function () {
  const messages = document.getElementById("messages");
  const form = document.getElementById("chat-form");
  const input = form.elements.message;
  const button = form.querySelector("button");

  function render(el, text) {
    el.dataset.raw = text;
    el.innerHTML = Finan.markdown(text);
  }

  function add(role, text) {
    document.querySelector(".suggestions")?.remove();
    const el = document.createElement("div");
    el.className = "msg " + role;
    render(el, text);
    messages.appendChild(el);
    el.scrollIntoView({ block: "end" });
    return el;
  }

  // "Salvar como nota" nas respostas: a pergunta vira o título.
  function addSave(el) {
    if (!window.finanSaveNote || el.querySelector(".save-note")) return;
    const b = document.createElement("button");
    b.type = "button";
    b.className = "link plain save-note";
    b.textContent = "Salvar como nota";
    b.addEventListener("click", async () => {
      const question = el.previousElementSibling?.classList.contains("user") ? el.previousElementSibling.dataset.raw : "";
      try {
        await window.finanSaveNote(question, el.dataset.raw);
        b.textContent = "Salvo em Notas ✓";
        b.disabled = true;
      } catch (err) {
        b.textContent = "Erro: " + err.message;
      }
    });
    el.appendChild(b);
  }

  document.querySelectorAll("[data-md]").forEach((el) => render(el, el.textContent));
  // main.js (módulo) carrega depois deste script: espera para ligar os botões.
  window.addEventListener("load", () => document.querySelectorAll(".msg.assistant").forEach(addSave));

  document.querySelectorAll(".suggestions button").forEach((b) =>
    b.addEventListener("click", () => { input.value = b.textContent; form.requestSubmit(); }));

  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); form.requestSubmit(); }
  });

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const text = input.value.trim();
    if (!text) return;
    button.disabled = true;
    input.value = "";
    add("user", text);
    const reply = add("assistant pending", "Pensando… Na primeira pergunta o modelo é carregado na memória, o que pode levar alguns minutos em computadores mais lentos.");
    let answer = "";
    try {
      await window.finanSync?.();
      await Finan.postSSE("/chat/enviar", new URLSearchParams({ message: text }), (ev) => {
        if (!ev.text) return;
        answer += ev.text;
        reply.classList.remove("pending");
        render(reply, answer);
        reply.scrollIntoView({ block: "end" });
      });
      addSave(reply);
    } catch (err) {
      reply.className = "msg error";
      render(reply, "Erro: " + err.message + "\n\nSua pergunta não foi salva. Tente de novo.");
      input.value = text;
    } finally {
      button.disabled = false;
      input.focus();
    }
  });
})();
