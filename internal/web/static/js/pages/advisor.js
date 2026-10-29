// Consultor IA no app do celular. Duas formas:
// - pelo computador pareado (a IA roda lá; precisa estar ao alcance);
// - pela nuvem, direto do celular, com os nomes anonimizados (cloud-ai.js);
// - dentro do celular, sem internet e sem nada sair do aparelho (local-ai.js).
import * as cloud from "../cloud-ai.js";
import * as db from "../db.js";
import * as finance from "../finance.js";
import * as local from "../local-ai.js";
import { endpoint } from "../device.js";
import { applyProposal, proposalButton, saveNote } from "../notes-save.js";
import { html, raw } from "../ui.js";

export const needsData = false;

let chat = null; // { history: [{role, content, proposals, agent}], suggest, model, ready, problem, busy, error, cloud, local }

// O time: cada personagem aconselha do seu jeito (internal/advisor/personas.go).
const SUGGESTIONS = {
  jade: ["O que eu faço com o meu dinheiro este mês?", "Qual é o meu maior problema agora?", "Me ajuda a montar um plano para o mês?"],
  bento: ["Qual dívida devo pagar primeiro?", "Quando eu fico livre das dívidas?", "Quanto adianta pagar R$ 200 a mais por mês?"],
  luna: ["O que vence esta semana?", "Algum orçamento perto de estourar?", "Quanto é a fatura do cartão?"],
  tito: ["Como chego na minha meta mais rápido?", "Quanto guardar por mês para a reserva?", "Minhas metas estão no ritmo?"],
  otavio: ["Onde estou gastando demais este mês?", "Quanto gastei com iFood este ano?", "Tenho assinatura esquecida?"],
};
const AGENT_KEY = "consultor-agente";
let team = [];
let agentId = "jade"; // com quem a pessoa está falando (lembrado entre aberturas)
const member = (id) => team.find((a) => a.id === id) || team[0];
const face = (a, size) => html`<img class="face" src="/static/img/avatar-${a.animal}.png" alt="" width="${size}" height="${size}">`;

const md = (text) => raw(window.Finan?.markdown ? window.Finan.markdown(text) : text.replace(/</g, "&lt;"));

export function render() {
  if (!chat) return html`<section class="panel"><h2>Consultor IA</h2><p class="muted loading">Abrindo o consultor…</p></section>`;
  if (chat.setup) {
    return html`<section class="panel"><h2>Consultor IA</h2>
      <p class="notice warn">Para usar o consultor sem computador, escolha uma IA na nuvem (há opções grátis) em
      <a href="/configuracoes#ia-nuvem">Ajustes → Consultor sem computador</a>. Ou conecte este celular a um computador com o finan_rich.</p></section>`;
  }
  const me = member(agentId);
  return html`
    <div class="page-head"><h1>Consultor IA</h1>
      ${chat.history.length ? html`<button type="button" class="secondary small-btn" data-action="clear">Nova conversa</button>` : ""}</div>
    <section class="panel chat">
      ${chat.problem ? html`<p class="notice warn">${chat.problem}</p>` : chat.local
        ? html`<p class="small muted">Usando ${chat.model} dentro deste celular: nada sai do aparelho, nem precisa de internet. É uma IA pequena: responde mais devagar, erra mais e não lança nada por você.</p>`
        : chat.cloud
        ? html`<p class="small muted">Usando ${chat.model} na nuvem, direto deste celular. Nomes de lojas, dívidas e metas vão com apelidos, e as notas não vão; a IA vê só valores, datas e categorias.</p>`
        : html`<p class="small muted">Usando ${chat.model} no computador. O consultor vê seus números, metas e notas a cada pergunta.</p>`}
      <div class="team-pick" role="group" aria-label="Com quem do time falar">
        ${team.map((a) => html`<button type="button" class="team-member" data-agent="${a.id}" aria-pressed="${a.id === me.id}">${face(a, 40)}<span><strong>${a.name}</strong><small>${a.role}</small></span></button>`)}
      </div>
      <div class="messages">
        ${chat.history.map((m, i) => html`<div class="msg ${m.role}">${m.role === "assistant" ? html`<span class="msg-who">${face(member(m.agent), 22)}${member(m.agent).name} · ${member(m.agent).role}</span>` : ""}${md(m.content)}
          ${m.app ? html`<p class="small muted app-answer">${m.aiError ? `Resposta do app, sem IA. A IA falhou: ${m.aiError}` : "Resposta do app, sem IA."}</p>` : ""}
          ${m.limit ? limitNotice() : ""}
          ${m.role === "assistant" ? html`<button type="button" class="link plain save-note" data-save="${i}">Salvar como nota</button>` : ""}</div>
          ${m.proposals?.length ? html`<div class="proposals">${m.proposals.map((p, j) => html`<div class="proposal"><span>${p.label}</span>
            <button type="button" class="small-btn${p.action === "apagar" ? " danger" : ""}" data-propose="${i}:${j}" ${p.applied ? "disabled" : ""}>${proposalButton(p)[p.applied ? 1 : 0]}</button></div>`)}</div>` : ""}`)}
        ${chat.busy ? html`<div class="msg assistant pending">${chat.partial ? md(chat.partial) : chat.local ? "Pensando… (dentro do celular, a primeira resposta pode levar um minuto)" : "Pensando… (no computador, a primeira resposta pode demorar enquanto o modelo carrega)"}</div>` : ""}
        ${chat.error ? html`<div class="msg error">Erro: ${chat.error}</div>` : ""}
        ${chat.suggest && !chat.busy ? html`<div class="handoff">${face(member(chat.suggest.agent), 28)}<span>Isso é com ${member(chat.suggest.agent).name} (${member(chat.suggest.agent).role}).</span>
          <button type="button" class="small-btn secondary" data-handoff>Perguntar para ${member(chat.suggest.agent).name}</button></div>` : ""}
        ${!chat.history.length && !chat.busy ? html`<div class="suggestions">${(SUGGESTIONS[me.id] || SUGGESTIONS.jade).map((s) => html`<button type="button" data-suggest>${s}</button>`)}</div>` : ""}
      </div>
      <form data-form="ask" class="chat-form">
        <textarea name="message" rows="2" placeholder="Mensagem para ${me.name}…" aria-label="Mensagem para ${me.name}" required ${chat.busy ? "disabled" : ""}></textarea>
        <button type="submit" ${chat.busy ? "disabled" : ""}>Enviar</button>
      </form>
    </section>`;
}

// limitNotice: acabou o grátis do finan_rich; o caminho para a chave grátis da própria pessoa.
function limitNotice() {
  return chat.cloud
    ? html`<p class="notice warn">O grátis do finan_rich chegou ao limite por agora. Para seguir com IA, use sua própria chave grátis (o Gemini dá uma de graça) em
      <a href="/configuracoes#ia-nuvem">Ajustes → Consultor sem computador</a>.</p>`
    : html`<p class="notice warn">O grátis do finan_rich chegou ao limite por agora. Para seguir com IA, use sua própria chave grátis nos Ajustes do computador.</p>`;
}

async function call(path, init = {}) {
  const api = await endpoint(path);
  const res = await fetch(api.url, { ...api.init, ...init, headers: { ...api.init.headers, ...(init.headers || {}) } });
  return res;
}

async function loadChat(ctx) {
  team = finance.team();
  agentId = await db.getMeta(AGENT_KEY, "jade").catch(() => "jade");
  await openChat(ctx);
}

async function openChat(ctx) {
  // A IA dentro do celular, quando ligada, vale sobre as outras: nada sai do aparelho.
  if (await local.ready().catch(() => false)) {
    chat = { history: await cloud.loadChat(), model: local.MODEL.name, ready: true, problem: "", busy: false, error: "", cloud: true, local: true };
    ctx.rerender();
    return;
  }
  const cfg = await cloud.config().catch(() => null);
  if (cloud.ready(cfg) && (cfg.enabled || !ctx.server)) {
    const history = await cloud.loadChat();
    for (const m of history) {
      for (const p of m.proposals || []) p.applied = await cloud.applied(ctx.ledger, p).catch(() => false);
    }
    chat = { history, model: `${cfg.preset} · ${cfg.model}`, ready: true, problem: "", busy: false, error: "", cloud: true };
    ctx.rerender();
    return;
  }
  if (!ctx.server) {
    chat = { setup: true };
    ctx.rerender();
    return;
  }
  try {
    const res = await call("/api/chat");
    const body = await res.json();
    if (!res.ok) throw new Error(body.error || res.statusText);
    chat = { history: body.history, model: body.model, ready: body.ready, problem: body.problem || "", busy: false, error: "" };
  } catch (err) {
    chat = { history: [], model: "", ready: false, problem: err instanceof TypeError ? "Sem conexão com o computador. O consultor funciona quando o celular estiver no mesmo Wi-Fi e o finan_rich aberto lá." : err.message, busy: false, error: "" };
  }
  ctx.rerender();
}

// showPartial mostra a resposta enquanto ela é gerada, sem redesenhar a tela toda.
function showPartial(text) {
  chat.partial = text;
  const el = document.querySelector(".msg.pending");
  if (el) el.innerHTML = window.Finan?.markdown ? window.Finan.markdown(text) : text.replace(/</g, "&lt;");
}

async function ask(ctx, question) {
  const agent = agentId;
  chat.history.push({ role: "user", content: question });
  chat.busy = true;
  chat.error = "";
  chat.suggest = null;
  ctx.rerender();
  try {
    if (chat.cloud) {
      let reply;
      if (chat.local) {
        // A IA do celular não passa pelo Go: a resposta pronta e a sem IA vêm daqui.
        reply = await cloud.prepared(ctx.ledger, question, agent);
        if (!reply) {
          try {
            reply = await local.ask(ctx.ledger, chat.history.slice(0, -1), question, (text) => showPartial(text), agent);
          } catch (err) {
            reply = { ...(await cloud.offline(ctx.ledger, question, agent)), aiError: err.message };
          }
        }
      } else {
        reply = await cloud.ask(ctx.ledger, chat.history.slice(0, -1), question, agent);
      }
      chat.partial = "";
      chat.history.push(assistantMessage(reply, agent));
      suggestHandoff(question, agent);
      await cloud.saveChat(chat.history);
      chat.busy = false;
      ctx.rerender();
      return;
    }
    await window.finanSync?.(); // a IA vê os lançamentos mais recentes deste celular
    const res = await call("/api/chat", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ message: question, agent }) });
    // A resposta vem em eventos "data: {...}" (no app, chega tudo de uma vez).
    const text = await res.text();
    if (!res.ok) throw new Error(JSON.parse(text || "{}").error || res.statusText);
    let answer = "";
    let app = null; // resposta do app, sem IA: { text, reason, limit }
    const proposals = []; // lançamentos que a IA propôs: entram só com o "Adicionar"
    for (const line of text.split("\n")) {
      if (!line.startsWith("data: ")) continue;
      const ev = JSON.parse(line.slice(6));
      if (ev.error) throw new Error(ev.error);
      if (ev.text) answer += ev.text;
      if (ev.proposal) proposals.push(ev.proposal);
      if (ev.app) app = ev.app;
    }
    chat.history.push(app
      ? assistantMessage({ answer: app.text, proposals, app: true, aiError: app.reason, limit: app.limit }, agent)
      : { role: "assistant", content: answer, proposals, agent });
    suggestHandoff(question, agent);
  } catch (err) {
    chat.partial = "";
    // TypeError: fetch daqui; "network error": fetch do Go (IA na nuvem) que caiu no meio.
    const reason = err instanceof TypeError || /network error/.test(err.message) ? (chat.cloud ? "sem internet" : "sem conexão com o computador") : err.message;
    // Sem IA, o personagem responde com as contas do app (só onde há o advisor.wasm: app e site).
    const reply = await cloud.offline(ctx.ledger, question, agent).catch(() => null);
    if (reply) {
      chat.history.push(assistantMessage({ ...reply, aiError: reason }, agent));
      if (chat.cloud) await cloud.saveChat(chat.history).catch(() => {});
    } else {
      chat.history.pop();
      chat.error = reason;
    }
  }
  chat.busy = false;
  ctx.rerender();
}

// assistantMessage: a resposta para a conversa; app/aiError/limit marcam a resposta sem IA.
function assistantMessage({ answer, proposals = [], app, aiError, limit }, agent) {
  const m = { role: "assistant", content: answer, proposals, agent };
  if (app) Object.assign(m, { app: true, aiError: aiError || "", limit: !!limit });
  return m;
}

// suggestHandoff: se o assunto é de outro do time, oferece perguntar para ele.
function suggestHandoff(question, agent) {
  const other = finance.route(question);
  chat.suggest = other && other !== agent ? { agent: other, question } : null;
}

async function pick(ctx, agent) {
  agentId = agent;
  chat.suggest = null;
  await db.setMeta(AGENT_KEY, agent).catch(() => {});
  ctx.rerender();
}

export function bind(root, ctx) {
  if (!chat) {
    loadChat(ctx);
    return;
  }
  const form = root.querySelector('[data-form="ask"]');
  if (!form) return; // só o aviso de configurar a IA
  form.addEventListener("submit", (e) => {
    e.preventDefault();
    const q = form.elements.message.value.trim();
    if (q && !chat.busy) ask(ctx, q);
  });
  form.elements.message.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); form.requestSubmit(); }
  });
  root.querySelectorAll("[data-agent]").forEach((b) => b.addEventListener("click", () => !chat.busy && pick(ctx, b.dataset.agent)));
  root.querySelector("[data-handoff]")?.addEventListener("click", async () => {
    const { agent, question } = chat.suggest;
    await pick(ctx, agent);
    ask(ctx, question);
  });
  root.querySelectorAll("[data-suggest]").forEach((b) => b.addEventListener("click", () => ask(ctx, b.textContent)));
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
