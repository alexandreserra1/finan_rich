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

  // Lançamento proposto pela IA: só entra se a pessoa clicar em "Adicionar".
  function addProposal(list, p) {
    const card = document.createElement("div");
    card.className = "proposal";
    const text = document.createElement("span");
    text.textContent = window.finanProposal ? window.finanProposal.describe(p) : p.description;
    const b = document.createElement("button");
    b.type = "button";
    b.className = "small-btn";
    b.textContent = "Adicionar";
    b.addEventListener("click", async () => {
      b.disabled = true;
      try {
        await window.finanProposal.save(p);
        b.textContent = "Adicionado ✓";
      } catch (err) {
        b.disabled = false;
        b.textContent = "Tentar de novo";
        text.textContent += " — erro: " + err.message;
      }
    });
    card.append(text, b);
    list.appendChild(card);
    card.scrollIntoView({ block: "end" });
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
    let proposals = null;
    try {
      await window.finanSync?.();
      await Finan.postSSE("/chat/enviar", new URLSearchParams({ message: text }), (ev) => {
        if (ev.proposal) {
          if (!proposals) {
            proposals = document.createElement("div");
            proposals.className = "proposals";
            messages.appendChild(proposals);
          }
          addProposal(proposals, ev.proposal);
          return;
        }
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
