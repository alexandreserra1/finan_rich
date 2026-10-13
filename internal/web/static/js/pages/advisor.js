// Consultor IA no app do celular: conversa com a IA que roda no computador
// pareado (precisa estar ao alcance), usando o token deste aparelho.
import { endpoint } from "../device.js";
import { saveNote } from "../notes-save.js";
import { html, raw } from "../ui.js";

export const needsData = false;

let chat = null; // { history: [{role, content}], model, ready, problem, busy, error }

const SUGGESTIONS = ["Onde estou gastando demais este mês?", "Qual dívida devo pagar primeiro?", "Quanto gastei com iFood este ano?", "Como chego na minha meta mais rápido?"];

const md = (text) => raw(window.Finan?.markdown ? window.Finan.markdown(text) : text.replace(/</g, "&lt;"));

export function render({ server }) {
  if (!server) {
    return html`<section class="panel"><h2>Consultor IA</h2>
      <p class="notice warn">Conecte este celular ao computador primeiro, em <a href="/configuracoes">Ajustes</a>. A IA roda lá.</p></section>`;
  }
  if (!chat) return html`<section class="panel"><h2>Consultor IA</h2><p class="muted loading">Falando com o computador…</p></section>`;
  return html`
    <div class="page-head"><h1>Consultor IA</h1>
      ${chat.history.length ? html`<button type="button" class="secondary small-btn" data-action="clear">Nova conversa</button>` : ""}</div>
    <section class="panel chat">
      ${chat.problem ? html`<p class="notice warn">${chat.problem}</p>` : html`<p class="small muted">Usando ${chat.model} no computador. O consultor vê seus números, metas e notas a cada pergunta.</p>`}
      <div class="messages">
        ${chat.history.map((m, i) => html`<div class="msg ${m.role}">${md(m.content)}
          ${m.role === "assistant" ? html`<button type="button" class="link plain save-note" data-save="${i}">Salvar como nota</button>` : ""}</div>`)}
        ${chat.busy ? html`<div class="msg assistant pending">Pensando… (no computador, a primeira resposta pode demorar enquanto o modelo carrega)</div>` : ""}
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

async function ask(ctx, question) {
  chat.history.push({ role: "user", content: question });
  chat.busy = true;
  chat.error = "";
  ctx.rerender();
  try {
    await window.finanSync?.(); // a IA vê os lançamentos mais recentes deste celular
    const res = await call("/api/chat", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ message: question }) });
    // A resposta vem em eventos "data: {...}" (no app, chega tudo de uma vez).
    const text = await res.text();
    if (!res.ok) throw new Error(JSON.parse(text || "{}").error || res.statusText);
    let answer = "";
    for (const line of text.split("\n")) {
      if (!line.startsWith("data: ")) continue;
      const ev = JSON.parse(line.slice(6));
      if (ev.error) throw new Error(ev.error);
      if (ev.text) answer += ev.text;
    }
    chat.history.push({ role: "assistant", content: answer });
  } catch (err) {
    chat.history.pop();
    chat.error = err instanceof TypeError ? "sem conexão com o computador" : err.message;
  }
  chat.busy = false;
  ctx.rerender();
}

export function bind(root, ctx) {
  if (!ctx.server) return;
  if (!chat) {
    loadChat(ctx);
    return;
  }
  const form = root.querySelector('[data-form="ask"]');
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
    await call("/api/chat/limpar", { method: "POST" }).catch(() => {});
    chat.history = [];
    ctx.rerender();
  });
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
