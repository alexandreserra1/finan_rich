// Chat com streaming: envia a pergunta e mostra a resposta conforme ela chega.
// Os números do app chegam primeiro (evento "numbers") e aparecem na hora;
// a resposta da IA vem embaixo deles.
(function () {
  const messages = document.getElementById("messages");
  const form = document.getElementById("chat-form");
  const input = form.elements.message;
  const button = form.querySelector("button");
  const team = [...document.querySelectorAll("[data-agent]")];
  let agent = "jade"; // com quem a pessoa está falando (lembrado neste navegador)
  try { agent = localStorage.getItem("consultor-agente") || agent; } catch {}

  const face = (animal, size) => {
    const img = document.createElement("img");
    Object.assign(img, { className: "face", src: "/static/img/avatar-" + animal + ".png", alt: "", width: size, height: size });
    return img;
  };

  function pick(id) {
    if (!team.some((b) => b.dataset.agent === id)) id = "jade";
    agent = id;
    try { localStorage.setItem("consultor-agente", id); } catch {}
    let name = "";
    team.forEach((b) => {
      b.setAttribute("aria-pressed", String(b.dataset.agent === id));
      if (b.dataset.agent !== id) return;
      name = b.dataset.name;
      const head = document.querySelector("[data-agent-face]");
      if (head) head.src = "/static/img/avatar-" + b.dataset.animal + ".png";
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
    tag.append(face(b.dataset.animal, 28), b.dataset.name + " · " + b.dataset.role);
    return tag;
  }

  function render(el, text) {
    el.dataset.raw = text;
    (el.querySelector(".md") || el).innerHTML = Finan.markdown(text);
  }

  function add(role, text, who) {
    document.querySelector(".chat-empty")?.remove();
    const el = document.createElement("div");
    el.className = "msg " + role;
    const md = document.createElement("div");
    md.className = "md";
    if (who) el.append(whoTag(who));
    el.append(md);
    render(el, text);
    messages.appendChild(el);
    el.scrollIntoView({ block: "end" });
    return el;
  }

  // "Pensando": três pontinhos e o aviso de que a primeira resposta demora.
  function thinking(text) {
    const box = document.createElement("div");
    box.className = "thinking-box";
    const line = document.createElement("div");
    line.className = "thinking";
    const dots = document.createElement("span");
    dots.className = "dots";
    dots.setAttribute("aria-hidden", "true");
    dots.append(document.createElement("i"), document.createElement("i"), document.createElement("i"));
    line.append(dots, text);
    const note = document.createElement("p");
    note.className = "small faint slow-note";
    note.textContent = "No computador, a primeira resposta pode demorar enquanto o modelo carrega.";
    box.append(line, note);
    return box;
  }

  // Cartão com os números do app: aberto enquanto a IA escreve, recolhível depois.
  function numbersCard(text) {
    const card = document.createElement("details");
    card.className = "numbers";
    card.open = true;
    const sum = document.createElement("summary");
    sum.textContent = "Os números do app";
    const md = document.createElement("div");
    md.className = "md";
    md.innerHTML = Finan.markdown(text);
    card.append(sum, md);
    return card;
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
    const kind = document.createElement("small");
    kind.textContent = window.finanProposal?.kind ? window.finanProposal.kind(p) : "Mudança";
    text.append(kind, p.label);
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
    const text = document.createElement("span");
    text.textContent = "Isso é com " + who.name + " (" + who.role + ").";
    const b = document.createElement("button");
    b.type = "button";
    b.className = "small-btn secondary";
    b.textContent = "Perguntar para " + who.name;
    b.addEventListener("click", () => { pick(who.id); input.value = question; form.requestSubmit(); });
    box.append(face(who.animal, 28), text, b);
    messages.appendChild(box);
  }
  // main.js (módulo) carrega depois deste script: espera para ligar os botões dos
  // cartões salvos na conversa e o "Salvar como nota".
  window.addEventListener("load", () => {
    document.querySelectorAll("[data-proposals]").forEach((list) => JSON.parse(list.dataset.proposals).forEach((p) => addProposal(list, p)));
    document.querySelectorAll(".msg.assistant").forEach(addSave);
  });

  document.querySelectorAll(".suggestions button").forEach((b) =>
    b.addEventListener("click", () => { input.value = b.textContent.trim(); form.requestSubmit(); }));

  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); form.requestSubmit(); }
  });

  // setState: o estado da IA no cabeçalho (pronta, sem IA, sem conexão).
  function setState(state, text) {
    const el = document.querySelector(".ai-state");
    if (!el) return;
    el.dataset.state = state;
    el.lastChild.textContent = text;
  }

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const text = input.value.trim();
    if (!text) return;
    button.disabled = true;
    input.value = "";
    document.querySelector(".handoff")?.remove();
    add("user", text);
    const reply = add("assistant pending", "", agent);
    const md = reply.querySelector(".md");
    let wait = thinking("Lendo seus lançamentos e fazendo as contas…");
    reply.append(wait);
    let card = null; // os números do app, antes da resposta da IA
    let numbers = "";
    let suggest = null;
    let answer = "";
    let proposals = null;
    const done = () => { wait?.remove(); wait = null; reply.classList.remove("pending"); };
    try {
      await window.finanSync?.();
      await Finan.postSSE("/chat/enviar", new URLSearchParams({ message: text, agent }), (ev) => {
        if (ev.suggest) { suggest = ev.suggest; return; }
        if (ev.numbers) {
          numbers = ev.numbers;
          card = numbersCard(numbers);
          reply.insertBefore(card, md);
          wait.querySelector(".thinking").lastChild.textContent = "Os números já estão aí. A IA está escrevendo a explicação…";
          reply.scrollIntoView({ block: "end" });
          return;
        }
        if (ev.app) {
          answer = ev.app.text;
          done();
          // A IA falhou: o texto termina com os mesmos números; o cartão é a resposta.
          if (card && numbers && answer.endsWith(numbers)) {
            md.innerHTML = Finan.markdown(answer.slice(0, -numbers.length));
            reply.insertBefore(md, card);
          } else {
            card?.remove();
            md.innerHTML = Finan.markdown(answer);
          }
          reply.dataset.raw = answer;
          markApp(reply, ev.app);
          reply.scrollIntoView({ block: "end" });
          return;
        }
        if (ev.proposal) {
          if (!proposals) {
            proposals = document.createElement("div");
            proposals.className = "proposals";
            reply.appendChild(proposals);
          }
          addProposal(proposals, ev.proposal).scrollIntoView({ block: "end" });
          return;
        }
        if (!ev.text) return;
        if (!answer && card) card.open = false;
        answer += ev.text;
        done();
        reply.dataset.raw = answer;
        md.innerHTML = Finan.markdown(answer);
        reply.scrollIntoView({ block: "end" });
      });
      done();
      addSave(reply);
      if (suggest) addHandoff(suggest, text);
    } catch (err) {
      reply.className = "msg error";
      reply.setAttribute("role", "alert");
      reply.querySelectorAll(".msg-who, .numbers, .thinking-box, .proposals").forEach((x) => x.remove());
      render(reply, "Erro: " + err.message + "\n\nSua pergunta não foi salva. Tente de novo.");
      if (err instanceof TypeError) setState("off", "Sem conexão com o computador");
      input.value = text;
    } finally {
      button.disabled = false;
      input.focus();
    }
  });
})();
