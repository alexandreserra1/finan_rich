// Consultor IA, a mesma tela no computador, no app do celular e no site.
// No computador (navegador), fala sempre com a IA dele pela /api/chat. No app:
// - pelo computador pareado (a IA roda lá; precisa estar ao alcance);
// - pela nuvem, direto do celular, com os nomes anonimizados (cloud-ai.js);
// - sem internet, a resposta pronta do app (regras em Go).
import * as cloud from "../cloud-ai.js";
import * as db from "../db.js";
import * as finance from "../finance.js";
import { endpoint, isApp } from "../device.js";
import { applyProposal, proposalButton, proposalKind, saveNote } from "../notes-save.js";
import { html, raw } from "../ui.js";

export const needsData = false;

let chat = null; // { history: [{role, content, proposals, agent}], suggest, model, ready, problem, busy, error, cloud, aiLocal, use }

// O time: cada personagem aconselha do seu jeito (internal/advisor/personas.go).
const SUGGESTIONS = {
  otavio: ["Como foi meu mês?", "Qual dívida devo pagar primeiro?", "O que vence esta semana?"],
  lia: ["Onde estou gastando demais este mês?", "Quanto ainda posso gastar?", "Como reduzir meus gastos?"],
  tito: ["Minhas metas estão no ritmo?", "Quanto guardar por mês para a reserva?", "Como chego na minha meta mais rápido?"],
};
const AGENT_KEY = "consultor-agente";
let team = [];
let agentId = "otavio"; // com quem a pessoa está falando (lembrado entre aberturas)
const member = (id) => team.find((a) => a.id === id) || team[0];
const face = (a, size) => html`<img class="face" src="/static/img/time/${a.id}-mini.webp" alt="" width="${size}" height="${size}">`;

const md = (text) => raw(window.Finan?.markdown ? window.Finan.markdown(text) : text.replace(/</g, "&lt;"));

const SEND_ICON = "M12 19V5M5 12l7-7 7 7";
const icon = (d, size = 18) => html`<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="${d}"></path></svg>`;

// numbersCard: as contas do app, que chegam na hora; depois da resposta da IA, ficam recolhidas.
// i: a mensagem da conversa, para lembrar se a pessoa abriu ou fechou o cartão (bind).
const numbersCard = (text, open, i) => html`<details class="numbers"${i === undefined ? "" : raw(` data-numbers="${i}"`)}${open ? " open" : ""}><summary>Os números do app</summary><div class="md">${md(text)}</div></details>`;

// answerBody: a resposta com o cartão dos números em cima. Se a IA falhou, o
// texto termina com os mesmos números: o cartão é a resposta (sem repetir).
function answerBody(m, i) {
  const n = m.numbers;
  if (n && m.app && m.content.endsWith(n)) return html`${md(m.content.slice(0, -n.length))}${numbersCard(n, m.numbersOpen ?? true, i)}`;
  return html`${n ? numbersCard(n, m.numbersOpen ?? false, i) : ""}${md(m.content)}`;
}

// aiState: o estado da IA no cabeçalho (pronta, sem IA ou sem conexão).
function aiState() {
  if (chat.offline) return ["off", "Sem conexão com o computador"];
  if (chat.problem) return ["none", "Sem IA agora"];
  if (!isApp) return ["ok", chat.model + (chat.aiLocal ? "" : " · na nuvem")];
  return ["ok", chat.model + (chat.cloud ? " · na nuvem" : " · no computador")];
}

const TRASH = "M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14M10 11v6M14 11v6";

function head() {
  return html`<div class="page-head consultor-head"><div class="title"><h1>Seu dinheiro tem com quem conversar.</h1>
    <p class="sub">Um time para ajudar a transformar números em próximos passos.</p></div></div>`;
}

// chatTop: quem está conversando, o estado da IA e o "Nova conversa".
function chatTop(me) {
  const [state, text] = chat.setup ? ["none", "Nenhuma IA configurada"] : aiState();
  return html`<div class="chat-top"><div class="consultor-id">${face(me, 58)}<div><h2>${me.name}</h2><span class="ai-state" data-state="${state}"><i></i>${me.role} · ${text}</span></div></div>
    ${chat.history?.length ? html`<button type="button" class="iconbtn" data-action="clear" aria-label="Nova conversa com ${me.name}" title="Nova conversa">${icon(TRASH)}</button>` : ""}</div>`;
}

function teamColumn(me) {
  return html`<aside class="panel specialists">
    <div class="team-pick" role="group" aria-label="Com quem do time falar">
      ${team.map((a) => html`<button type="button" class="team-member" data-agent="${a.id}" aria-pressed="${a.id === me.id}">${face(a, 42)}<span><strong>${a.name}</strong><small>${a.role}</small></span></button>`)}
    </div>
    <p class="faint tiny team-note">Personagens virtuais com retratos gerados por IA. As contas são do app; a IA explica e aconselha.</p>
  </aside>`;
}

export function render() {
  if (!chat) return html`<section class="panel"><h2>Consultor</h2><p class="muted loading">Abrindo o consultor…</p></section>`;
  const me = member(agentId);
  if (chat.setup) {
    return html`${head()}<div class="chat-layout">${teamColumn(me)}<section class="panel chat">${chatTop(me)}
      <div class="notice warn ai-setup"><strong>O consultor precisa de uma IA para funcionar.</strong>
      Para usar sem computador, escolha uma IA na nuvem (há opções grátis) em
      <a href="/configuracoes#ia-nuvem">Ajustes → Consultor sem computador</a>. Ou conecte este celular a um computador com a Rich.
      <span class="small muted">O resto do app funciona normalmente sem IA.</span></div></section></div>`;
  }
  return html`
    ${head()}
    <div class="chat-layout">
    ${teamColumn(me)}
    <section class="panel chat">
      ${chatTop(me)}
      ${!isApp ? deskNotes() : chat.problem ? html`<p class="notice warn">${chat.problem}</p>` : chat.cloud
        ? html`<p class="small muted chat-where">Usando ${chat.model} na nuvem, direto deste celular. Nomes de lojas, dívidas e metas vão com apelidos, e as notas não vão; a IA vê só valores, datas e categorias.</p>`
        : html`<p class="small muted chat-where">Usando ${chat.model} no computador. O consultor vê seus números, metas e notas a cada pergunta.</p>`}
      <div class="messages">
        ${!chat.history.length && !chat.busy ? html`<div class="chat-empty">
          <h2>Vamos dar um próximo passo?</h2>
          <p class="quote">“${me.quote}”</p>
          <p class="small muted">${me.name} vê seus lançamentos, dívidas, metas e notas. As contas são feitas pelo app; a IA interpreta e aconselha. Para lançar algo, vem um cartão e você confirma.</p>
        </div>` : ""}
        ${chat.history.map((m, i) => m.role === "user" ? html`<div class="msg user">${md(m.content)}</div>` : html`<div class="msg assistant">
          <span class="msg-who">${face(member(m.agent), 28)}${member(m.agent).name} · ${member(m.agent).role}</span>${answerBody(m, i)}
          ${m.app ? html`<p class="small muted app-answer">${m.aiError ? `Resposta do app, sem IA. A IA falhou: ${m.aiError}` : "Resposta do app, sem IA."}</p>` : ""}
          ${m.limit ? limitNotice() : ""}
          ${m.proposals?.length ? html`<div class="proposals">${m.proposals.map((p, j) => html`<div class="proposal"><span><small>${proposalKind(p)}</small>${p.label}</span>
            <button type="button" class="small-btn${p.action === "apagar" ? " danger" : ""}" data-propose="${i}:${j}" ${p.applied ? "disabled" : ""}>${proposalButton(p)[p.applied ? 1 : 0]}</button></div>`)}</div>` : ""}
          <button type="button" class="link plain save-note" data-save="${i}">Salvar como nota</button></div>`)}
        ${chat.busy ? html`<div class="msg assistant pending"><span class="msg-who">${face(me, 28)}${me.name} · ${me.role}</span>
          ${chat.numbers ? numbersCard(chat.numbers, true) : ""}
          <div class="partial">${chat.partial ? md(chat.partial) : ""}</div>
          ${chat.partial ? "" : html`<div class="thinking"><span class="dots" aria-hidden="true"><i></i><i></i><i></i></span>${chat.numbers ? "Os números já estão aí. A IA está escrevendo a explicação…" : "Lendo seus lançamentos e fazendo as contas…"}</div>
          <p class="small faint slow-note">${chat.cloud ? "Na nuvem, a resposta leva alguns segundos." : "No computador, a primeira resposta pode demorar enquanto o modelo carrega."}</p>`}</div>` : ""}
        ${chat.error ? html`<div class="msg error" role="alert">Erro: ${chat.error}</div>` : ""}
        ${chat.suggest && !chat.busy ? html`<div class="handoff">${face(member(chat.suggest.agent), 28)}<span>Isso é com ${member(chat.suggest.agent).name} (${member(chat.suggest.agent).role}).</span>
          <button type="button" class="small-btn secondary" data-handoff>Perguntar para ${member(chat.suggest.agent).name}</button></div>` : ""}
      </div>
      ${chat.busy ? "" : html`<div class="suggestions">${(SUGGESTIONS[me.id] || SUGGESTIONS.otavio).map((s) => html`<button type="button" data-suggest="${s}">${s}</button>`)}</div>`}
      <form data-form="ask" class="chat-form">
        <div class="composer">
          <textarea name="message" rows="1" placeholder="Qual é o seu próximo plano? Fale com ${me.name}…" aria-label="Mensagem para ${me.name}" required ${chat.busy ? "disabled" : ""}>${chat.draft || ""}</textarea>
          <button type="submit" class="send" aria-label="Enviar" ${chat.busy ? "disabled" : ""}>${icon(SEND_ICON, 16)}</button>
        </div>
        <p class="composer-hint"><span class="desk-only">Enter envia · Shift+Enter quebra a linha · </span>O cálculo aparece primeiro. Nada é lançado sem você confirmar.</p>
      </form>
    </section>
    </div>`;
}

// deskNotes: no computador, o aviso da IA que não está pronta (com o "Usar …" de um
// modelo já baixado; baixar e instalar ficam nos Ajustes) e onde a IA roda.
function deskNotes() {
  const use = chat.use;
  return html`${chat.problem ? html`<div class="notice warn"><strong>${chat.problem}</strong>
      ${use ? html`<p class="small">Ou use um que já está neste computador, sem baixar nada:</p>
      <div class="row-actions">${use.models.map((m) => html`<form method="post" action="/configuracoes"><input type="hidden" name="preset" value="${use.preset}"><input type="hidden" name="base_url" value="${use.baseURL}"><input type="hidden" name="model" value="${m}"><input type="hidden" name="volta" value="/chat"><button type="submit" class="secondary">Usar ${m}</button></form>`)}</div>` : ""}
      <p class="small">Para resolver, abra os <a href="/configuracoes#ia">Ajustes</a>.</p></div>` : ""}
    <p class="small muted chat-where">${chat.aiLocal ? "A IA roda neste computador: nada sai daqui." : "Na nuvem, nomes de lojas, dívidas e metas vão com apelidos."} O consultor vê seus números atuais a cada pergunta. <a href="/configuracoes">Trocar o modelo</a></p>`;
}

// limitNotice: acabou o grátis do finan_rich; o caminho para a chave grátis da própria pessoa.
function limitNotice() {
  return chat.cloud
    ? html`<p class="notice warn">O grátis da Rich chegou ao limite por agora. Para seguir com IA, use sua própria chave grátis (o Gemini dá uma de graça) em
      <a href="/configuracoes#ia-nuvem">Ajustes → Consultor sem computador</a>.</p>`
    : !isApp
    ? html`<p class="notice warn">O grátis da Rich chegou ao limite por agora. Para seguir com IA, use sua própria chave grátis (o Gemini dá uma de graça) em <a href="/configuracoes#ia">Ajustes</a>.</p>`
    : html`<p class="notice warn">O grátis da Rich chegou ao limite por agora. Para seguir com IA, use sua própria chave grátis nos Ajustes do computador.</p>`;
}

async function call(path, init = {}) {
  const api = await endpoint(path);
  const res = await fetch(api.url, { ...api.init, ...init, headers: { ...api.init.headers, ...(init.headers || {}) } });
  return res;
}

async function loadChat(ctx) {
  team = finance.team();
  agentId = await db.getMeta(AGENT_KEY, "otavio").catch(() => "otavio");
  await openChat(ctx);
}

async function openChat(ctx) {
  // No computador (isApp falso), a IA é sempre a dele: vai direto para a /api/chat.
  const cfg = isApp ? await cloud.config().catch(() => null) : null;
  if (cloud.ready(cfg) && (cfg.enabled || !ctx.server)) {
    const history = await cloud.loadChat();
    for (const m of history) {
      for (const p of m.proposals || []) p.applied = await cloud.applied(ctx.ledger, p).catch(() => false);
    }
    chat = { history, model: `${cfg.preset} · ${cfg.model}`, ready: true, problem: "", busy: false, error: "", cloud: true };
    ctx.rerender();
    return;
  }
  if (isApp && !ctx.server) {
    chat = { setup: true };
    ctx.rerender();
    return;
  }
  try {
    const res = await call("/api/chat");
    const body = await res.json();
    if (!res.ok) throw new Error(body.error || res.statusText);
    chat = { history: body.history, model: body.model, ready: body.ready, problem: body.problem || "", busy: false, error: "", aiLocal: body.local, use: body.use };
  } catch (err) {
    const offline = err instanceof TypeError;
    const noLink = isApp ? "Sem conexão com o computador. O consultor funciona quando o celular estiver no mesmo Wi-Fi e a Rich aberta lá."
      : "Sem conexão com a Rich. Confira se ela está aberta no computador e recarregue a página.";
    chat = { history: [], model: "", ready: false, offline, problem: offline ? noLink : err.message, busy: false, error: "" };
  }
  ctx.rerender();
}

// showPartial mostra a resposta enquanto ela é gerada, sem redesenhar a tela toda.
function showPartial(text) {
  const first = !chat.partial;
  chat.partial = text;
  const el = document.querySelector(".msg.pending .partial");
  if (el) el.innerHTML = window.Finan?.markdown ? window.Finan.markdown(text) : text.replace(/</g, "&lt;");
  if (first) document.querySelectorAll(".msg.pending .thinking, .msg.pending .slow-note").forEach((x) => x.remove());
}

async function ask(ctx, question) {
  const agent = agentId;
  chat.history.push({ role: "user", content: question });
  chat.busy = true;
  chat.error = "";
  chat.suggest = null;
  chat.offline = false;
  chat.numbers = "";
  chat.scroll = true; // cada redesenho desta pergunta mostra o fim da conversa
  ctx.rerender();
  // Os números do app aparecem na hora; a resposta da IA chega embaixo deles.
  // No computador, eles vêm do servidor, no primeiro evento da resposta.
  chat.numbers = isApp ? await cloud.numbers(ctx.ledger, question, agent).catch(() => "") : "";
  if (chat.numbers) ctx.rerender();
  try {
    if (chat.cloud) {
      const reply = await cloud.ask(ctx.ledger, chat.history.slice(0, -1), question, agent);
      chat.partial = "";
      chat.history.push(assistantMessage(reply, agent, chat.numbers));
      suggestHandoff(question, agent);
      await cloud.saveChat(chat.history);
      chat.busy = false;
      ctx.rerender();
      return;
    }
    await window.finanSync?.(); // a IA vê os lançamentos mais recentes deste aparelho
    const res = await call("/api/chat", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ message: question, agent }) });
    if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || res.statusText);
    let answer = "";
    let app = null; // resposta do app, sem IA: { text, reason, limit }
    const proposals = []; // lançamentos que a IA propôs: entram só com o "Adicionar"
    // A resposta vem em eventos "data: {...}", mostrados conforme chegam (no app, chega tudo de uma vez).
    await window.Finan.readSSE(res, (ev) => {
      if (ev.text) showPartial((answer += ev.text));
      if (ev.proposal) proposals.push(ev.proposal);
      if (ev.app) app = ev.app;
      if (ev.numbers && !chat.numbers) {
        chat.numbers = ev.numbers;
        ctx.rerender();
      }
    });
    chat.partial = "";
    chat.history.push(assistantMessage(app ? { answer: app.text, proposals, app: true, aiError: app.reason, limit: app.limit } : { answer, proposals }, agent, chat.numbers));
    suggestHandoff(question, agent);
  } catch (err) {
    chat.partial = "";
    // TypeError: fetch daqui; "network error": fetch do Go (IA na nuvem) que caiu no meio.
    const reason = err instanceof TypeError || /network error/.test(err.message) ? (chat.cloud ? "sem internet" : "sem conexão com o computador") : err.message;
    // Sem IA, o personagem responde com as contas do app (só onde há o advisor.wasm: app e site).
    const reply = isApp ? await cloud.offline(ctx.ledger, question, agent).catch(() => null) : null;
    if (!chat.cloud && err instanceof TypeError) chat.offline = true;
    if (reply) {
      chat.history.push(assistantMessage({ ...reply, aiError: reason }, agent, chat.numbers));
      if (chat.cloud) await cloud.saveChat(chat.history).catch(() => {});
    } else {
      chat.history.pop();
      chat.error = reason;
      chat.draft = question; // a pergunta não foi salva: volta para a caixa, para tentar de novo
    }
  }
  chat.busy = false;
  ctx.rerender();
}

// assistantMessage: a resposta para a conversa; app/aiError/limit marcam a resposta sem IA;
// numbers são as contas do app mostradas antes da resposta (ficam no cartão recolhível).
function assistantMessage({ answer, proposals = [], app, aiError, limit }, agent, numbers) {
  const m = { role: "assistant", content: answer, proposals, agent };
  if (numbers) m.numbers = numbers;
  if (app) Object.assign(m, { app: true, aiError: aiError || "", limit: !!limit });
  return m;
}

// suggestHandoff: se o assunto é de outro do time, oferece perguntar para ele.
function suggestHandoff(question, agent) {
  const other = finance.route(question);
  chat.suggest = other && other !== agent ? { agent: other, question } : null;
}

// pick troca quem responde. O que já estava escrito continua na caixa (lido só
// agora: a pessoa pode ter digitado enquanto a escolha era guardada).
async function pick(ctx, agent) {
  agentId = agent;
  chat.suggest = null;
  await db.setMeta(AGENT_KEY, agent).catch(() => {});
  chat.draft = document.querySelector('[data-form="ask"] [name=message]')?.value || "";
  ctx.rerender();
}

export function bind(root, ctx) {
  if (!chat) {
    loadChat(ctx);
    return;
  }
  const form = root.querySelector('[data-form="ask"]');
  if (!form) return; // só o aviso de configurar a IA
  // Vindo do "Perguntar para ..." do Painel: quem avisou e a pergunta já escrita.
  const want = ctx.params.get("agente"), question = ctx.params.get("pergunta");
  if (want !== null || question !== null) {
    history.replaceState(null, "", "/chat"); // recarregar não escreve a pergunta de novo
    if (team.some((a) => a.id === want)) {
      agentId = want;
      db.setMeta(AGENT_KEY, want).catch(() => {});
    }
    chat.draft = (question || "").slice(0, 500);
    chat.suggest = null;
    ctx.rerender();
    return;
  }
  if (chat.scroll) {
    root.querySelector(".messages > :last-child")?.scrollIntoView({ block: "end" });
    chat.scroll = chat.busy;
  }
  form.addEventListener("submit", (e) => {
    e.preventDefault();
    const q = form.elements.message.value.trim();
    if (q && !chat.busy) ask(ctx, q);
  });
  form.elements.message.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); form.requestSubmit(); }
  });
  // Safari não tem field-sizing (app.css): aqui a caixa cresce com o texto.
  if (!CSS.supports("field-sizing", "content")) {
    const box = form.elements.message;
    box.addEventListener("input", () => { box.style.height = "auto"; box.style.height = `${Math.min(box.scrollHeight, 160)}px`; });
  }
  if (chat.draft) { // a pergunta do Painel (ou a que já estava escrita), pronta para enviar ou mudar
    const box = form.elements.message;
    box.dispatchEvent(new Event("input"));
    box.focus();
    box.setSelectionRange(box.value.length, box.value.length);
    chat.draft = "";
  }
  // O cartão dos números fica como a pessoa deixou quando a tela é redesenhada.
  root.querySelectorAll("details[data-numbers]").forEach((d) => d.addEventListener("toggle", () => {
    const m = chat.history[d.dataset.numbers];
    if (m) m.numbersOpen = d.open;
  }));
  root.querySelectorAll("[data-agent]").forEach((b) => b.addEventListener("click", () => !chat.busy && pick(ctx, b.dataset.agent)));
  root.querySelector("[data-handoff]")?.addEventListener("click", async () => {
    const { agent, question } = chat.suggest;
    await pick(ctx, agent);
    ask(ctx, question);
  });
  root.querySelectorAll("[data-suggest]").forEach((b) => b.addEventListener("click", () => ask(ctx, b.dataset.suggest)));
  root.querySelector('[data-action="clear"]')?.addEventListener("click", async () => {
    if (!confirm("Apagar toda a conversa?")) return;
    if (chat.cloud) await cloud.saveChat([]);
    else await call("/api/chat/limpar", { method: "POST" }).catch(() => {});
    chat.history = [];
    ctx.rerender();
  });
  root.querySelectorAll("[data-propose]").forEach((b) => b.addEventListener("click", async () => {
    const [i, j] = b.dataset.propose.split(":").map(Number);
    const p = chat.history[i].proposals[j];
    try {
      await applyProposal(p);
      p.applied = true;
      await ctx.refresh("Pronto: " + p.label);
    } catch (err) {
      ctx.error(err);
    }
  }));
  root.querySelectorAll("[data-save]").forEach((b) => b.addEventListener("click", async () => {
    const i = Number(b.dataset.save);
    try {
      await saveNote(chat.history[i - 1]?.content, chat.history[i].content);
      await ctx.refresh("Resposta salva em Notas.");
    } catch (err) {
      ctx.error(err);
    }
  }));
}

// Os Ajustes mudaram a IA: a próxima abertura do consultor relê a configuração.
export function resetChat() { chat = null; }
