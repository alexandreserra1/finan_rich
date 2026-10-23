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
