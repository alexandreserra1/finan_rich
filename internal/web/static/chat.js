// Chat com streaming: envia a pergunta e mostra a resposta conforme ela chega.
(function () {
  const messages = document.getElementById("messages");
  const form = document.getElementById("chat-form");
  const input = form.elements.message;
  const button = form.querySelector("button");
  const team = [...document.querySelectorAll("[data-agent]")];
  let agent = "jade"; // com quem a pessoa está falando (lembrado neste navegador)
  try { agent = localStorage.getItem("consultor-agente") || agent; } catch {}

  function pick(id) {
    if (!team.some((b) => b.dataset.agent === id)) id = "jade";
    agent = id;
    try { localStorage.setItem("consultor-agente", id); } catch {}
    let name = "";
    team.forEach((b) => {
      b.setAttribute("aria-pressed", String(b.dataset.agent === id));
      if (b.dataset.agent === id) name = b.dataset.name;
    });
    document.querySelectorAll(".suggestions [data-for]").forEach((b) => { b.hidden = b.dataset.for !== id; });
    input.placeholder = "Mensagem para " + name + "…";
    input.setAttribute("aria-label", input.placeholder);
    document.querySelector(".handoff")?.remove();
  }

  // whoTag: rosto e nome de quem respondeu, no alto da resposta.
  function whoTag(id) {
    const b = team.find((x) => x.dataset.agent === id) || team[0];
    const tag = document.createElement("span");
    tag.className = "msg-who";
    const img = document.createElement("img");
    Object.assign(img, { className: "face", src: "/static/img/avatar-" + b.dataset.animal + ".png", alt: "", width: 22, height: 22 });
    tag.append(img, b.dataset.name + " · " + b.dataset.role);
    return tag;
  }

  function render(el, text) {
    el.dataset.raw = text;
    (el.querySelector(".md") || el).innerHTML = Finan.markdown(text);
  }

  function add(role, text, who) {
    document.querySelector(".suggestions")?.remove();
    const el = document.createElement("div");
    el.className = "msg " + role;
    if (who) {
      const md = document.createElement("div");
      md.className = "md";
      el.append(whoTag(who), md);
    }
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

  // Mudança proposta pela IA (lançar, alterar, apagar, meta, dívida): só vale
  // se a pessoa tocar no botão.
  function addProposal(list, p) {
    const card = document.createElement("div");
    card.className = "proposal";
    const text = document.createElement("span");
    text.textContent = p.label;
    const b = document.createElement("button");
    b.type = "button";
    b.className = "small-btn" + (p.action === "apagar" ? " danger" : "");
    const [todo, done] = window.finanProposal ? window.finanProposal.button(p) : ["Confirmar", "Feito ✓"];
    b.textContent = p.applied ? done : todo;
    b.disabled = !!p.applied;
    b.addEventListener("click", async () => {
      b.disabled = true;
      try {
        await window.finanProposal.apply(p);
        b.textContent = done;
      } catch (err) {
        b.disabled = false;
        b.textContent = "Tentar de novo";
        text.textContent = p.label + " — erro: " + err.message;
      }
    });
    card.append(text, b);
    list.appendChild(card);
    return card;
  }

  // Resposta do app, sem IA (pergunta pronta ou a IA falhou): marca de onde veio e,
  // se acabou o grátis do finan_rich, mostra como usar a própria chave grátis.
  function markApp(el, ev) {
    const tag = document.createElement("p");
    tag.className = "small muted app-answer";
    tag.textContent = ev.reason ? "Resposta do app, sem IA. A IA falhou: " + ev.reason : "Resposta do app, sem IA.";
    el.appendChild(tag);
    if (!ev.limit) return;
    const box = document.createElement("p");
    box.className = "notice warn";
    box.append("O grátis do finan_rich chegou ao limite por agora. Para seguir com IA, use sua própria chave grátis (o Gemini dá uma de graça) em ");
    const a = document.createElement("a");
    a.href = "/configuracoes";
    a.textContent = "Ajustes";
    box.append(a, ".");
    el.appendChild(box);
  }

  document.querySelectorAll("[data-md]").forEach((md) => render(md.parentElement, md.textContent));
  team.forEach((b) => b.addEventListener("click", () => pick(b.dataset.agent)));
  pick(agent);

  // O assunto é de outro do time: oferece perguntar para ele a mesma coisa.
  function addHandoff(who, question) {
    const box = document.createElement("div");
    box.className = "handoff";
    const img = document.createElement("img");
    Object.assign(img, { className: "face", src: "/static/img/avatar-" + who.animal + ".png", alt: "", width: 28, height: 28 });
    const text = document.createElement("span");
    text.textContent = "Isso é com " + who.name + " (" + who.role + ").";
    const b = document.createElement("button");
    b.type = "button";
    b.className = "small-btn secondary";
    b.textContent = "Perguntar para " + who.name;
    b.addEventListener("click", () => { pick(who.id); input.value = question; form.requestSubmit(); });
    box.append(img, text, b);
    messages.appendChild(box);
  }
  // Cartões salvos na conversa (main.js carrega depois: espera para os botões).
  window.addEventListener("load", () => document.querySelectorAll("[data-proposals]").forEach((list) =>
    JSON.parse(list.dataset.proposals).forEach((p) => addProposal(list, p))));
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
    document.querySelector(".handoff")?.remove();
    add("user", text);
    const reply = add("assistant pending", "Pensando… Na primeira pergunta o modelo é carregado na memória, o que pode levar alguns minutos em computadores mais lentos.", agent);
    let suggest = null;
    let answer = "";
    let proposals = null;
    try {
      await window.finanSync?.();
      await Finan.postSSE("/chat/enviar", new URLSearchParams({ message: text, agent }), (ev) => {
        if (ev.suggest) { suggest = ev.suggest; return; }
        if (ev.app) {
          answer = ev.app.text;
          reply.classList.remove("pending");
          render(reply, answer);
          markApp(reply, ev.app);
          reply.scrollIntoView({ block: "end" });
          return;
        }
        if (ev.proposal) {
          if (!proposals) {
            proposals = document.createElement("div");
            proposals.className = "proposals";
            messages.appendChild(proposals);
          }
          addProposal(proposals, ev.proposal).scrollIntoView({ block: "end" });
          return;
        }
        if (!ev.text) return;
        answer += ev.text;
        reply.classList.remove("pending");
        render(reply, answer);
        reply.scrollIntoView({ block: "end" });
      });
      addSave(reply);
      if (suggest) addHandoff(suggest, text);
    } catch (err) {
      reply.className = "msg error";
      reply.querySelector(".msg-who")?.remove();
      render(reply, "Erro: " + err.message + "\n\nSua pergunta não foi salva. Tente de novo.");
      input.value = text;
    } finally {
      button.disabled = false;
      input.focus();
    }
  });
})();
