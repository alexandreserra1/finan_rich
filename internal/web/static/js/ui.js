// Utilitários de interface.

// html monta HTML escapando tudo que vem de dados (evita injeção de código).
// Use raw() para inserir HTML já montado.
const RAW = Symbol("raw");
export const raw = (s) => ({ [RAW]: String(s) });
const escape = (v) => String(v ?? "")
  .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
  .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
function piece(v) {
  if (Array.isArray(v)) return v.map(piece).join("");
  if (v && v[RAW] !== undefined) return v[RAW];
  if (v === false || v === null || v === undefined) return "";
  return escape(v);
}
export function html(strings, ...values) {
  return raw(strings.reduce((out, s, i) => out + s + (i < values.length ? piece(values[i]) : ""), ""));
}
export const toString = (h) => h[RAW];

// addPanel é o formulário de cadastro recolhido atrás de um botão "+ Novo…":
// a lista vem primeiro. No computador (tela larga) já abre; aberto ou fechado,
// continua assim quando a tela é redesenhada.
const opened = new Map();
document.addEventListener("toggle", (e) => {
  if (e.target.matches?.("details.add")) opened.set(e.target.dataset.add, e.target.open);
}, true);
export function addPanel(key, title, body, open = false) {
  const isOpen = open || (opened.get(key) ?? matchMedia("(min-width: 721px)").matches);
  return html`<details class="panel add" data-add="${key}" ${isOpen ? "open" : ""}><summary><h2>${title}</h2></summary>${body}</details>`;
}

// newId cria um UUID. crypto.randomUUID só existe em https/localhost; pelo
// Wi-Fi (http) usamos getRandomValues, que funciona em qualquer contexto.
export function newId() {
  if (crypto.randomUUID) return crypto.randomUUID();
  const b = crypto.getRandomValues(new Uint8Array(16));
  b[6] = (b[6] & 0x0f) | 0x40;
  b[8] = (b[8] & 0x3f) | 0x80;
  const h = [...b].map((x) => x.toString(16).padStart(2, "0")).join("");
  return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
}

export const BUCKETS = { necessidades: "Necessidades", desejos: "Desejos", investimentos: "Investimentos", renda: "Renda" };
export const SPENDING = ["necessidades", "desejos", "investimentos"];

export const today = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};
export const currentMonth = () => today().slice(0, 7);

// shiftMonth("2026-01", -1) => "2025-12"
export function shiftMonth(key, delta) {
  const [y, m] = key.split("-").map(Number);
  const d = new Date(y, m - 1 + delta, 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

export const barPct = (v, max) => (max > 0 ? Math.floor((v * 100) / max) : 0);
export const ratePct = (bp) => `${Math.floor(bp / 100)},${String(bp % 100).padStart(2, "0")}%`;
export const centsInput = (c) => `${Math.floor(c / 100)},${String(c % 100).padStart(2, "0")}`;

// flash mostra uma mensagem no topo da página.
export function flash(message, kind = "ok") {
  const box = document.getElementById("flash");
  if (!box) return;
  box.className = `flash ${kind}`;
  box.textContent = message;
  box.hidden = false;
  clearTimeout(flash.timer);
  flash.timer = setTimeout(() => { box.hidden = true; }, 4000);
}

// Ícones (traço, desenhos do Lucide, ISC) dos lançamentos, pela categoria.
const ICONS = {
  food: "M3 2v7c0 1.1.9 2 2 2h4a2 2 0 0 0 2-2V2M7 2v20M21 15V2a5 5 0 0 0-5 5v6c0 1.1.9 2 2 2h3zm0 0v7",
  cart: "M8 21a1 1 0 1 0 0-2 1 1 0 0 0 0 2zM19 21a1 1 0 1 0 0-2 1 1 0 0 0 0 2zM2 2h3l2.7 12.4a2 2 0 0 0 2 1.6h9.7a2 2 0 0 0 2-1.6L23 6H6",
  car: "M19 17h2v-4l-3-5H6l-3 5v4h2M7 17a2 2 0 1 0 4 0 2 2 0 1 0-4 0M13 17a2 2 0 1 0 4 0 2 2 0 1 0-4 0",
  home: "M3 11l9-7 9 7v9a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1z",
  play: "M5 3l14 9-14 9z",
  heart: "M19 14c1.5-1.5 3-3.2 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.8 0-3 .5-4.5 2-1.5-1.5-2.7-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4 3 5.5l7 7z",
  bolt: "M13 2L3 14h9l-1 8 10-12h-9z",
  grow: "M22 7l-8.5 8.5-5-5L2 17M16 7h6v6",
  book: "M4 19.5A2.5 2.5 0 0 1 6.5 17H20V2H6.5A2.5 2.5 0 0 0 4 4.5v15zM20 17v5H6.5A2.5 2.5 0 0 1 4 19.5",
  in: "M12 19V5M5 12l7-7 7 7",
  tag: "M20.6 13.4l-7.2 7.2a2 2 0 0 1-2.8 0L2 12V2h10l8.6 8.6a2 2 0 0 1 0 2.8zM7 7h.01",
};
const fold = (s) => String(s || "").toLowerCase().normalize("NFD").replace(/[\u0300-\u036f]/g, "");
export function categoryIcon(category, kind) {
  if (kind === "entrada") return ICONS.in;
  const n = fold(category?.name);
  const rules = [[/restaur|delivery|ifood|lanche|comida|padaria/, "food"], [/mercado|supermerc|feira/, "cart"],
    [/transporte|uber|combust|carro|onibus|gasolina/, "car"], [/contas? d|luz|agua|internet|energia/, "bolt"],
    [/moradia|aluguel|condom|casa\b/, "home"], [/assinatura|lazer|stream|cinema|viage/, "play"], [/saude|farmac|medic|academia/, "heart"],
    [/invest|reserva|poupan/, "grow"], [/educa|curso|escola|livro/, "book"]];
  return ICONS[rules.find(([re]) => re.test(n))?.[1] || (category?.bucket === "investimentos" ? "grow" : "tag")];
}
// iconTile é o quadrado colorido com o ícone (cor do grupo da categoria).
export function iconTile(category, kind, size = 34) {
  const color = kind === "entrada" ? "accent" : category?.bucket || "muted";
  return html`<span class="icon-tile" style="--c: var(--${color}); --s: ${size}px"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="${categoryIcon(category, kind)}"/></svg></span>`;
}

// ---------- Janela (modal) e aviso com "Desfazer" (visual Esmeralda) ----------

// openModal mostra uma janela já desenhada na tela (<div class="modal" hidden>):
// Esc ou o fundo fecham, o foco entra nela e volta para quem abriu.
export function openModal(modal, trigger) {
  modal.hidden = false;
  document.body.classList.add("modal-open");
  // Uma janela aberta conta como formulário em uso: a sincronização não redesenha por baixo.
  modal.dispatchEvent(new Event("input", { bubbles: true }));
  const close = () => closeModal(modal);
  modal._onKey = (e) => { if (e.key === "Escape") close(); };
  document.addEventListener("keydown", modal._onKey);
  modal._trigger = trigger || document.activeElement;
  modal.querySelectorAll("[data-close]").forEach((el) => { el.onclick = close; });
  (modal.querySelector("[autofocus]") || modal.querySelector("input, select, textarea, button"))?.focus();
}

export function closeModal(modal) {
  if (modal.hidden) return;
  modal.hidden = true;
  document.body.classList.remove("modal-open");
  document.removeEventListener("keydown", modal._onKey);
  if (modal._trigger?.isConnected) modal._trigger.focus();
}

// toast mostra um aviso curto embaixo, no centro; com undo, ganha o botão "Desfazer".
export function toast(message, undo) {
  document.querySelector(".toast")?.remove();
  const box = Object.assign(document.createElement("div"), { className: "toast", role: "status" });
  box.append(Object.assign(document.createElement("i"), { className: "dot" }), Object.assign(document.createElement("span"), { textContent: message }));
  if (undo) {
    const btn = Object.assign(document.createElement("button"), { type: "button", textContent: "Desfazer" });
    btn.onclick = () => { box.remove(); undo(); };
    box.append(btn);
  }
  document.body.append(box);
  setTimeout(() => box.remove(), undo ? 4500 : 3500);
}
