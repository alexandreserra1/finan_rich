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

  document.querySelectorAll("[data-md]").forEach((el) => render(el, el.textContent));

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
