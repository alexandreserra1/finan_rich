// Sininho dos avisos, no cabeçalho de todas as telas: quantos avisos há, a lista,
// "Limpar tudo" e cada aviso levando à tela dele. Limpar vale em todos os aparelhos
// (preferência avisos-limpos, sincronizada); o aviso limpo só volta se a situação
// mudar, porque o ID dele (finance.Alert.ID) muda junto.
import * as finance from "./finance.js";
import { askLink, html, toString, today } from "./ui.js";

const CLEARED = "avisos-limpos";
// shortcut: guarda só os últimos 200 IDs limpos (o limite do Go); um aviso limpo há
// muito tempo pode voltar. Se incomodar, podar em Go pelos avisos ainda ativos.
const KEEP = 200;
const buttons = [...document.querySelectorAll("[data-bell]")];
const panel = document.getElementById("avisos");
let list = [];
let cleared = [];
let save = async () => {};
let go = (url) => location.assign(url);
let opener = null;

// mount recebe de main.js como gravar a preferência e como abrir uma tela.
export function mount(opts) {
  ({ save, go } = opts);
}

// update redesenha o sininho com os avisos de agora (ledger null: esconde, ex.: site sem login).
export function update(ledger) {
  if (!panel) return;
  try {
    list = ledger ? finance.alerts(ledger, today()) : [];
  } catch {
    list = []; // as regras ainda não carregaram
  }
  cleared = ledger?.prefs.find((p) => p.id === CLEARED && !p.deleted)?.value || [];
  const n = list.length;
  for (const b of buttons) {
    b.hidden = !ledger;
    const count = b.querySelector("[data-bell-count]");
    count.textContent = n > 9 ? "9+" : String(n);
    count.hidden = !n;
    b.dataset.level = list[0]?.level || "";
    b.setAttribute("aria-label", n ? `Avisos: ${n} novo${n > 1 ? "s" : ""}` : "Avisos: nenhum novo");
  }
  if (!ledger) close();
  panel.innerHTML = toString(view());
}

function view() {
  return html`<div class="bell-head"><h2 id="avisos-h" tabindex="-1">Avisos</h2>
    ${list.length ? html`<button type="button" class="link plain" data-bell-clear>Limpar tudo</button>` : ""}</div>
  ${list.length ? html`<ul class="bell-list">${list.map((a) => html`<li class="bell-item ${a.level}">
    <a class="bell-main" href="${a.link}" data-alert="${a.id}"><img class="face" src="/static/img/avatar-${a.agent.animal}.png" alt="" width="32" height="32">
      <span><b>${a.agent.name}</b> · ${a.title}<small>${a.text}</small></span></a>
    ${askLink(a, a.title)}</li>`)}</ul>`
    : html`<p class="muted small bell-empty">Nenhum aviso novo. Quando o time vir algo, aparece aqui.</p>`}`;
}

function open(button) {
  opener = button;
  panel.hidden = false;
  buttons.forEach((b) => b.setAttribute("aria-expanded", "true"));
  panel.querySelector("h2")?.focus();
}

function close() {
  if (!panel || panel.hidden) return;
  panel.hidden = true;
  buttons.forEach((b) => b.setAttribute("aria-expanded", "false"));
}

// clear junta os IDs aos já limpos e grava (main.js redesenha o sininho depois).
const clear = (ids) => save([...new Set([...cleared, ...ids])].slice(-KEEP));

buttons.forEach((b) => b.addEventListener("click", () => (panel.hidden ? open(b) : close())));
panel?.addEventListener("click", async (e) => {
  if (e.target.closest("[data-bell-clear]")) {
    await clear(list.map((a) => a.id));
    panel.querySelector("h2")?.focus();
    return;
  }
  const link = e.target.closest("a[data-alert], a.ask-team");
  if (!link || e.metaKey || e.ctrlKey || e.shiftKey) return;
  e.preventDefault(); // grava antes de sair da tela; main.js não navega de novo
  const item = link.closest(".bell-item")?.querySelector("[data-alert]");
  await clear([item.dataset.alert]).catch(() => {});
  close();
  go(link.href);
});
document.addEventListener("keydown", (e) => {
  if (e.key !== "Escape" || !panel || panel.hidden) return;
  close();
  opener?.focus();
});
document.addEventListener("click", (e) => {
  if (panel && !panel.hidden && !panel.contains(e.target) && !e.target.closest("[data-bell]")) close();
});
