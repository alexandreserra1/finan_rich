// Consultor IA no app do celular. Duas formas:
// - pelo computador pareado (a IA roda lá; precisa estar ao alcance);
// - pela nuvem, direto do celular, com os nomes anonimizados (cloud-ai.js);
// - dentro do celular, sem internet e sem nada sair do aparelho (local-ai.js).
import * as cloud from "../cloud-ai.js";
import * as local from "../local-ai.js";
import { endpoint } from "../device.js";
import { applyProposal, proposalButton, saveNote } from "../notes-save.js";
import { html, raw } from "../ui.js";

export const needsData = false;

let chat = null; // { history: [{role, content, proposals}], model, ready, problem, busy, error, cloud, local }

const SUGGESTIONS = ["Onde estou gastando demais este mês?", "Qual dívida devo pagar primeiro?", "Quanto gastei com iFood este ano?", "Como chego na minha meta mais rápido?"];

const md = (text) => raw(window.Finan?.markdown ? window.Finan.markdown(text) : text.replace(/</g, "&lt;"));

export function render() {
  if (!chat) return html`<section class="panel"><h2>Consultor IA</h2><p class="muted loading">Abrindo o consultor…</p></section>`;
  if (chat.setup) {
    return html`<section class="panel"><h2>Consultor IA</h2>
      <p class="notice warn">Para usar o consultor sem computador, escolha uma IA na nuvem (há opções grátis) em
      <a href="/configuracoes#ia-nuvem">Ajustes → Consultor sem computador</a>. Ou conecte este celular a um computador com o finan_rich.</p></section>`;
  }
  return html`
    <div class="page-head"><h1>Consultor IA</h1>
      ${chat.history.length ? html`<button type="button" class="secondary small-btn" data-action="clear">Nova conversa</button>` : ""}</div>
    <section class="panel chat">
      ${chat.problem ? html`<p class="notice warn">${chat.problem}</p>` : chat.local
        ? html`<p class="small muted">Usando ${chat.model} dentro deste celular: nada sai do aparelho, nem precisa de internet. É uma IA pequena: responde mais devagar, erra mais e não lança nada por você.</p>`
        : chat.cloud
        ? html`<p class="small muted">Usando ${chat.model} na nuvem, direto deste celular. Nomes de lojas, dívidas e metas vão com apelidos, e as notas não vão; a IA vê só valores, datas e categorias.</p>`
        : html`<p class="small muted">Usando ${chat.model} no computador. O consultor vê seus números, metas e notas a cada pergunta.</p>`}
      <div class="messages">
        ${chat.history.map((m, i) => html`<div class="msg ${m.role}">${md(m.content)}
          ${m.role === "assistant" ? html`<button type="button" class="link plain save-note" data-save="${i}">Salvar como nota</button>` : ""}</div>
          ${m.proposals?.length ? html`<div class="proposals">${m.proposals.map((p, j) => html`<div class="proposal"><span>${p.label}</span>
            <button type="button" class="small-btn${p.action === "apagar" ? " danger" : ""}" data-propose="${i}:${j}" ${p.applied ? "disabled" : ""}>${proposalButton(p)[p.applied ? 1 : 0]}</button></div>`)}</div>` : ""}`)}
        ${chat.busy ? html`<div class="msg assistant pending">${chat.partial ? md(chat.partial) : chat.local ? "Pensando… (dentro do celular, a primeira resposta pode levar um minuto)" : "Pensando… (no computador, a primeira resposta pode demorar enquanto o modelo carrega)"}</div>` : ""}
        ${chat.error ? html`<div class="msg error">Erro: ${chat.error}</div>` : ""}
        ${!chat.history.length && !chat.busy ? html`<div class="suggestions">${SUGGESTIONS.map((s) => html`<button type="button" data-suggest>${s}</button>`)}</div>` : ""}
      </div>
      <form data-form="ask" class="chat-form">
        <textarea name="message" rows="2" placeholder="Pergunte sobre dívidas, gastos, metas..." required ${chat.busy ? "disabled" : ""}></textarea>
        <button type="submit" ${chat.busy ? "disabled" : ""}>Enviar</button>
      </form>
    </section>`;
}

async function call(path, init = {}) {
  const api = await endpoint(path);
  const res = await fetch(api.url, { ...api.init, ...init, headers: { ...api.init.headers, ...(init.headers || {}) } });
  return res;
}

async function loadChat(ctx) {
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
  chat.history.push({ role: "user", content: question });
  chat.busy = true;
  chat.error = "";
  ctx.rerender();
  try {
    if (chat.cloud) {
      const { answer, proposals } = chat.local
        ? await local.ask(ctx.ledger, chat.history.slice(0, -1), question, (text) => showPartial(text))
        : await cloud.ask(ctx.ledger, chat.history.slice(0, -1), question);
      chat.partial = "";
      chat.history.push({ role: "assistant", content: answer, proposals });
      await cloud.saveChat(chat.history);
      chat.busy = false;
      ctx.rerender();
      return;
    }
    await window.finanSync?.(); // a IA vê os lançamentos mais recentes deste celular
    const res = await call("/api/chat", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ message: question }) });
    // A resposta vem em eventos "data: {...}" (no app, chega tudo de uma vez).
    const text = await res.text();
    if (!res.ok) throw new Error(JSON.parse(text || "{}").error || res.statusText);
    let answer = "";
    const proposals = []; // lançamentos que a IA propôs: entram só com o "Adicionar"
    for (const line of text.split("\n")) {
      if (!line.startsWith("data: ")) continue;
      const ev = JSON.parse(line.slice(6));
      if (ev.error) throw new Error(ev.error);
      if (ev.text) answer += ev.text;
      if (ev.proposal) proposals.push(ev.proposal);
    }
    chat.history.push({ role: "assistant", content: answer, proposals });
  } catch (err) {
    chat.partial = "";
    chat.history.pop();
    // TypeError: fetch daqui; "network error": fetch do Go (IA na nuvem) que caiu no meio.
    chat.error = err instanceof TypeError || /network error/.test(err.message) ? (chat.cloud ? "sem internet" : "sem conexão com o computador") : err.message;
  }
  chat.busy = false;
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
