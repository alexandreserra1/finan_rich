// Ponto de entrada do app no aparelho. Roda em todas as páginas:
// - em todas: sincronização em segundo plano e indicador de status;
// - nas telas de dados (Painel, Lançamentos, Dívidas, Categorias): monta a tela
//   com os dados guardados no aparelho, que funcionam sem conexão.
// No app do celular (Capacitor), Ajustes e Consultor também são telas daqui.
import * as db from "./db.js";
import * as device from "./device.js";
import "./lock.js"; // app do celular: pede digital/rosto ou senha para abrir
import { applyProposal, proposalButton, saveNote } from "./notes-save.js";
import { scheduleDebtReminders } from "./native.js";
import * as finance from "./finance.js";
import * as charts from "./charts.js";
import * as sync from "./sync.js";
import { currentMonth, flash, newId, today, toString } from "./ui.js";
import * as dashboard from "./pages/dashboard.js";
import * as transactions from "./pages/transactions.js";
import * as debts from "./pages/debts.js";
import * as categories from "./pages/categories.js";
import * as notes from "./pages/notes.js";
import * as exportPage from "./pages/export.js";
import * as importPage from "./pages/import.js";
import * as goals from "./pages/goals.js";
import * as insights from "./pages/insights.js";
import * as devicePage from "./pages/device.js";
import * as advisor from "./pages/advisor.js";
import * as accounts from "./pages/accounts.js";
import * as start from "./pages/start.js";
import * as oauthPage from "./pages/oauth.js";

const ROUTES = { "/": dashboard, "/lancamentos": transactions, "/dividas": debts, "/categorias": categories, "/notas": notes, "/exportar": exportPage, "/importar": importPage, "/metas": goals, "/analise": insights };
ROUTES["/contas"] = accounts;
if (device.isApp) Object.assign(ROUTES, { "/configuracoes": devicePage, "/chat": advisor, "/oauth": oauthPage });

const root = document.getElementById("app");
let ledger = null;
let server = null; // computador pareado (só no app do celular)
let booted = false; // as regras (WebAssembly) já carregaram e a primeira tela foi desenhada
let changedDuringBoot = false; // chegaram dados novos enquanto o app iniciava
let dirty = false; // há um formulário sendo preenchido: não redesenhar por baixo
let rerenderPending = false;

// ---------- status da sincronização ----------

const pill = document.getElementById("sync-status");
const LABELS = {
  ok: () => "Sincronizado",
  syncing: () => "Sincronizando…",
  offline: (s) => s.pending ? `Offline · ${s.pending} pendente(s)` : "Offline",
  login: () => device.isApp ? "Pareie de novo" : "Entre de novo para sincronizar",
  unpaired: () => device.isSite ? "Só neste navegador" : "Só neste celular", // não é erro: toca para conectar a um computador
  google: () => "Entre de novo com Google",
  error: () => "Erro ao sincronizar",
};
function showStatus(s) {
  if (!pill) return;
  pill.hidden = false;
  pill.dataset.status = s.status;
  pill.textContent = LABELS[s.status](s);
  pill.title = s.status === "offline"
    ? "Sem conexão com o computador. Tudo que você fizer fica salvo neste aparelho e é enviado quando a conexão voltar."
    : s.message || (s.lastSync ? `Última sincronização: ${s.lastSync.toLocaleTimeString("pt-BR")}` : "");
  pill.onclick = null;
  if (device.isApp && (s.status === "login" || s.status === "unpaired" || s.status === "google")) pill.onclick = () => navigate("/configuracoes");
  else if (s.status === "login") pill.onclick = () => { location.href = "/entrar?volta=" + encodeURIComponent(location.pathname); };
}
window.addEventListener("finan-sync", async (e) => {
  showStatus(e.detail);
  if (e.detail.dataChanged && root) {
    if (booted) await reloadAndRender();
    else changedDuringBoot = true; // relê assim que terminar de iniciar
  }
  if (e.detail.notice) flash(e.detail.notice, "err"); // ex.: categoria apagada aqui, mas usada em outro aparelho
});

// O chat chama isto antes de perguntar, para a IA ver os dados mais recentes.
window.finanSync = () => sync.sync();
// E isto para guardar uma resposta em Notas.
window.finanSaveNote = saveNote;
// E isto para aplicar o que o consultor propôs (depois do toque no botão).
window.finanProposal = { apply: applyProposal, button: proposalButton };

// Ao sair da conta, apaga os dados guardados neste aparelho.
document.querySelectorAll('form[action="/sair"]').forEach((form) => form.addEventListener("submit", async (e) => {
  e.preventDefault();
  await db.wipe();
  form.submit();
}));

sync.start();

// ---------- telas de dados ----------

function context(path, params) {
  return {
    ledger,
    server,
    params,
    month: /^\d{4}-\d{2}$/.test(params.get("mes") || "") ? params.get("mes") : currentMonth(),
    // save valida (mesma regra do servidor), grava no aparelho e agenda o envio.
    async save(type, partial, message, { rerender = true } = {}) {
      const list = listOf(type);
      const previous = list.find((r) => r.id === partial.id);
      const record = { ...(previous || {}), ...partial, id: partial.id || newId(), deleted: false };
      record.updatedAt = Math.max(Date.now(), (previous?.updatedAt || 0) + 1);
      delete record.deleted;
      const normalized = finance.validate(type, record);
      await db.saveLocal(type, normalized);
      upsert(list, normalized);
      if (type === "recurring") await applyRecurring();
      if (type === "debt") scheduleDebtReminders(ledger).catch(() => {});
      sync.soon();
      if (rerender) render(message);
    },
    async remove(type, record, message) {
      const gone = { ...record, deleted: true, updatedAt: Math.max(Date.now(), record.updatedAt + 1) };
      await db.saveLocal(type, gone);
      upsert(listOf(type), gone);
      sync.soon();
      render(message);
    },
    async refresh(message) { await reloadAndRender(message); },
    go(url) { navigate(url); },
    error(err) { flash(err.message.replace(/^registro inválido: /, ""), "err"); },
    flash(message) { flash(message); },
    rerender() { render(); }, // redesenha sem reler os dados (estado só da tela mudou)
  };
}

const listOf = (type) => ({
  category: ledger.categories, transaction: ledger.transactions, debt: ledger.debts, pref: ledger.prefs,
  note: ledger.notes, goal: ledger.goals, recurring: ledger.recurring,
  account: ledger.accounts,
})[type];
function upsert(list, record) {
  const i = list.findIndex((r) => r.id === record.id);
  if (i >= 0) list[i] = record; else list.push(record);
}

function render(message) {
  const path = location.pathname;
  const page = ROUTES[path];
  if (!page || !ledger) return;
  if (!ledger.categories.length && page.needsData !== false && device.isSite) {
    // Site na primeira visita: boas-vindas em vez de "conecte ao computador".
    const ctx = context(path, new URLSearchParams(location.search));
    root.innerHTML = `<p id="flash" hidden></p>` + toString(start.render(ctx));
    start.bind(root, ctx);
    return;
  }
  if (!ledger.categories.length && page.needsData !== false) {
    root.innerHTML = device.isApp && !server
      ? `<p class="notice warn">Este celular ainda não está conectado ao computador. <a href="/configuracoes">Conectar agora</a> ou <a href="/configuracoes#sozinho">usar só neste celular</a>.</p>`
      : sync.state.status === "offline"
        ? `<p class="notice warn">Este aparelho ainda não tem seus dados. Conecte-se ao computador onde o finan_rich está aberto uma vez para baixá-los; depois ele funciona sem conexão.</p>`
        : `<p class="muted loading">Baixando seus dados…</p>`;
    return;
  }
  const ctx = context(path, new URLSearchParams(location.search));
  root.innerHTML = `<p id="flash" hidden></p>` + toString(page.render(ctx));
  page.bind?.(root, ctx);
  charts.mount(root);
  dirty = false;
  rerenderPending = false;
  document.querySelectorAll("nav.tabs a").forEach((a) => {
    if (new URL(a.href).pathname === path) a.setAttribute("aria-current", "page");
    else a.removeAttribute("aria-current");
  });
  if (message) flash(message);
}

async function load() {
  [ledger, server] = await Promise.all([db.loadLedger(), device.server()]);
  await applyRecurring();
  scheduleDebtReminders(ledger).catch(() => {}); // no app: avisos de vencimento
}

// applyRecurring grava os lançamentos recorrentes que já venceram e ainda não
// existem. O ID é fixo por recorrência e mês: outro aparelho que gerar o mesmo
// lançamento não duplica.
async function applyRecurring() {
  const due = finance.recurringDue(ledger, today());
  if (!due.length) return;
  const stamp = Date.now();
  const items = due.map((t) => ({ type: "transaction", record: finance.validate("transaction", { ...t, updatedAt: stamp }) }));
  await db.saveManyLocal(items);
  for (const { record } of items) upsert(ledger.transactions, record);
  sync.soon();
}

async function reloadAndRender(message) {
  await load();
  if (dirty && !message) {
    rerenderPending = true; // redesenha quando o usuário terminar de preencher
    return;
  }
  render(message);
}

// Navegação entre as telas de dados sem recarregar a página (funciona offline).
function navigate(url, replace = false) {
  const u = new URL(url, location.href);
  if (replace) history.replaceState(null, "", u);
  else history.pushState(null, "", u);
  render();
  window.scrollTo(0, 0);
}

document.addEventListener("click", (e) => {
  const a = e.target.closest("a[href]");
  if (!a || !root || e.metaKey || e.ctrlKey || e.shiftKey || a.target) return;
  const u = new URL(a.href, location.href);
  if (u.origin !== location.origin || !ROUTES[u.pathname]) return;
  e.preventDefault();
  navigate(u.href);
});
window.addEventListener("popstate", () => render());

if (root) {
  root.addEventListener("input", () => { dirty = true; });
  root.addEventListener("focusout", () => {
    setTimeout(() => {
      if (rerenderPending && !root.contains(document.activeElement)) render();
    }, 0);
  });
  // Primeiro as regras; só depois lê os dados, para pegar também o que uma
  // sincronização que terminou durante a carga já gravou no aparelho.
  finance.load()
    .then(load)
    .then(() => {
      booted = true;
      render();
      if (changedDuringBoot) reloadAndRender();
    })
    .catch((err) => {
      root.innerHTML = `<p class="notice warn">Não foi possível iniciar o app: ${String(err.message || err).replace(/</g, "&lt;")}</p>`;
    });
}

// App do celular: links finanrich://parear?url=...&codigo=... (QR code do
// computador) abrem a tela de Ajustes já preenchida, para a pessoa confirmar.
const capApp = window.Capacitor?.Plugins?.App;
if (device.isApp && capApp) {
  const openLink = (link) => {
    // Volta do login com Google (o site repassa para cá): termina em /oauth.
    if (link?.startsWith("finanrich://google?")) return navigate("/oauth?" + link.slice("finanrich://google?".length));
    const p = link && device.pairingLink(link);
    if (p) navigate("/configuracoes?" + new URLSearchParams({ url: p.url, codigo: p.code }));
  };
  capApp.addListener("appUrlOpen", (e) => openLink(e.url));
  capApp.getLaunchUrl().then((r) => openLink(r?.url)).catch(() => {});
  // Voltar do Android: volta uma tela; na primeira, fecha o app.
  capApp.addListener("backButton", ({ canGoBack }) => (canGoBack ? history.back() : capApp.exitApp()));
}
