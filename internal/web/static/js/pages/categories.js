// Categorias e orçamento (design Rich): a divisão do salário em três grupos,
// o limite (teto) de cada categoria com o quanto já foi usado no mês e as
// categorias de entrada. Planejado, realizado e gasto vêm do Go (finance.Dashboard).
import * as finance from "../finance.js";
import { BUCKETS, SPENDING, centsInput, closeModal, html, iconTile, openModal, raw, today } from "../ui.js";
import { PLUS, metric, pageHead } from "./dashboard.js";
import { canDeleteCategory } from "../finance.js";

const ORDER = { renda: 0, necessidades: 1, desejos: 2, investimentos: 3 };
const ICON = {
  pencil: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20h4L19 9l-4-4L4 16v4zM14 6l4 4"/></svg>'),
  trash: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6"/></svg>'),
  close: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6L6 18M6 6l12 12"/></svg>'),
};

function currentSplit(ledger) {
  const pref = ledger.prefs.find((p) => p.id === "split" && !p.deleted);
  return pref?.value || { necessidades: 50, desejos: 30, investimentos: 20 };
}

// limitRow: uma categoria de gasto com a barra do mês (avisa a partir de 80%, como o Painel).
function limitRow(c, spent, m) {
  const budget = c.budgetCents || 0;
  const pct = budget ? Math.round((spent * 100) / budget) : 0;
  const level = !budget ? "" : pct >= 100 ? "over" : pct >= 80 ? "near" : "";
  return html`<li class="limit-row">
    <span class="limit-name">${iconTile(c, "saida", 32)}<span><strong>${c.name}</strong><small class="faint">${BUCKETS[c.bucket]}</small></span></span>
    <span class="limit-bar">
      <span class="limit-labels"><span>${m(spent)} usados</span><span>${budget ? `${m(budget)} de teto` : "Sem teto"}</span></span>
      <span class="bar"><span class="${level}" style="width: ${Math.min(pct, 100)}%"></span></span>
    </span>
    <span class="goal-chip ${level === "over" ? "neg" : level === "near" ? "warn" : "plain"} ${budget ? "" : "free"}">${!budget ? "Livre" : spent > budget ? `${m(spent - budget)} acima` : `${m(budget - spent)} restantes`}</span>
    <span class="limit-actions">
      <button type="button" class="round-btn" data-edit="${c.id}" aria-label="Editar ${c.name}">${ICON.pencil}</button>
      <button type="button" class="round-btn danger" data-delete="${c.id}" aria-label="Apagar ${c.name}">${ICON.trash}</button>
    </span>
  </li>`;
}

function splitForm(split) {
  return html`<div class="modal" id="split-modal" hidden>
    <div class="modal-scrim" data-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="split-title">
      <form data-form="split" class="modal-form">
        <div class="modal-head"><h2 id="split-title">Editar divisão</h2><button type="button" class="icon-btn" data-close aria-label="Fechar">${ICON.close}</button></div>
        <p class="small muted">Quanto da renda do mês vai para cada grupo. A regra clássica é 50/30/20; ajuste à sua realidade (a soma precisa dar 100%).</p>
        <div class="field-grid three">
          ${SPENDING.map((b) => html`<label class="field">${BUCKETS[b]} (%) <input type="number" name="${b}" min="0" max="100" value="${split[b] ?? 0}"></label>`)}
        </div>
        <div class="modal-foot"><span class="spacer"></span><button type="button" class="secondary" data-close>Cancelar</button><button type="submit">Salvar divisão</button></div>
      </form>
    </div>
  </div>`;
}

function categoryForm() {
  return html`<div class="modal" id="category-modal" hidden>
    <div class="modal-scrim" data-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="category-title">
      <form data-form="category" class="modal-form" novalidate>
        <div class="modal-head"><h2 id="category-title">Nova categoria</h2><button type="button" class="icon-btn" data-close aria-label="Fechar">${ICON.close}</button></div>
        <label class="field new-only">Nome <input name="name" maxlength="60" placeholder="Ex.: Pet" autocomplete="off"></label>
        <div class="field-grid">
          <label class="field">Grupo <select name="bucket">${Object.entries(BUCKETS).map(([k, label]) => html`<option value="${k}">${label}</option>`)}</select></label>
          <label class="field">Teto por mês (R$) <input name="budget" inputmode="decimal" placeholder="sem teto" autocomplete="off"></label>
        </div>
        <p class="small muted">O teto é o limite de gasto da categoria por mês (ex.: Restaurantes R$ 400). O Painel avisa ao chegar a 80% e ao estourar. Deixe vazio para não ter teto.</p>
        <div class="modal-foot"><span class="spacer"></span><button type="button" class="secondary" data-close>Cancelar</button><button type="submit">Salvar</button></div>
      </form>
    </div>
  </div>`;
}

export function render({ ledger }) {
  const m = finance.money;
  const month = today().slice(0, 7);
  const d = finance.dashboard(ledger, month);
  const split = currentSplit(ledger);
  const spentByName = Object.fromEntries(d.categories.map((c) => [c.name, c.spentCents]));
  const cats = ledger.categories.filter((c) => !c.deleted)
    .sort((a, b) => ORDER[a.bucket] - ORDER[b.bucket] || a.name.localeCompare(b.name, "pt-BR"));
  const spending = cats.filter((c) => c.bucket !== "renda")
    .sort((a, b) => !!b.budgetCents - !!a.budgetCents || (spentByName[b.name] || 0) - (spentByName[a.name] || 0));
  const income = cats.filter((c) => c.bucket === "renda");
  const bucket = (b) => d.buckets.find((x) => x.bucket === b) || { pct: split[b] ?? 0, plannedCents: 0, spentCents: 0 };

  return html`
    ${pageHead("Um plano que cabe na vida.", null, { sub: "Seu orçamento é uma intenção. O realizado vem dos lançamentos.", actions: html`<button type="button" data-action="new-category">${PLUS}Nova categoria</button>` })}

    <section class="panel">
      <div class="panel-head"><div><h2>A divisão do seu salário</h2><p class="faint small">${split.necessidades} / ${split.desejos} / ${split.investimentos} · personalize para o seu momento</p></div>
        <button type="button" class="secondary" data-action="edit-split">Editar divisão</button></div>
      <div class="cards metricgrid three split-cards">
        ${SPENDING.map((b) => { const x = bucket(b);
          return metric(`${BUCKETS[b]} · ${x.pct}%`, d.inCents ? m(x.plannedCents) : "—", d.inCents ? `Planejado · realizado ${m(x.spentCents)}` : `lance a renda de ${d.label} para calcular`); })}
      </div>
    </section>

    <section class="panel">
      <div class="panel-head"><div><h2>Seus limites por categoria</h2><p class="faint small">${d.label} · os alertas começam em 80%</p></div></div>
      <ul class="limits">${spending.map((c) => limitRow(c, spentByName[c.name] || 0, m))}</ul>
    </section>

    <section class="panel">
      <div class="panel-head"><h2>Categorias de entrada</h2></div>
      <ul class="income-cats">${income.map((c) => html`<li><span>${c.name}</span><button type="button" class="link" data-delete="${c.id}">Excluir</button></li>`)}</ul>
    </section>
    ${splitForm(split)}
    ${categoryForm()}`;
}

export function bind(root, ctx) {
  const splitModal = root.querySelector("#split-modal");
  root.querySelector('[data-action="edit-split"]').addEventListener("click", (e) => openModal(splitModal, e.currentTarget));
  root.querySelector('[data-form="split"]').addEventListener("submit", async (e) => {
    e.preventDefault();
    const value = Object.fromEntries(SPENDING.map((b) => [b, Number(e.target.elements[b].value)]));
    try {
      closeModal(splitModal);
      await ctx.save("pref", { id: "split", value }, "Divisão do salário atualizada.");
    } catch (err) {
      ctx.error(err);
    }
  });

  // Uma janela para criar e editar: criando, pede o nome; editando, muda grupo e teto.
  const modal = root.querySelector("#category-modal");
  const form = root.querySelector('[data-form="category"]');
  const f = form.elements;
  let editing = null;
  const open = (c, trigger) => {
    editing = c || null;
    form.reset();
    modal.classList.toggle("editing", !!c);
    form.querySelector("#category-title").textContent = c ? c.name : "Nova categoria";
    if (c) {
      f.bucket.value = c.bucket;
      f.budget.value = c.budgetCents ? centsInput(c.budgetCents) : "";
    }
    openModal(modal, trigger);
  };
  root.querySelector('[data-action="new-category"]').addEventListener("click", (e) => open(null, e.currentTarget));
  root.querySelectorAll("[data-edit]").forEach((b) => b.addEventListener("click", () => open(ctx.ledger.categories.find((x) => x.id === b.dataset.edit), b)));

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const name = editing ? editing.name : f.name.value.trim();
    if (!editing && !name) {
      ctx.error(new Error("dê um nome à categoria"));
      return;
    }
    if (!editing && ctx.ledger.categories.some((c) => !c.deleted && c.name.toLowerCase() === name.toLowerCase())) {
      ctx.error(new Error("já existe uma categoria com esse nome"));
      return;
    }
    try {
      const text = f.budget.value.trim();
      const budgetCents = text && f.bucket.value !== "renda" ? finance.parseMoney(text) : 0;
      closeModal(modal);
      if (editing) {
        await ctx.save("category", { id: editing.id, bucket: f.bucket.value, budgetCents },
          budgetCents ? `Orçamento de ${name}: ${finance.money(budgetCents)} por mês.` : `${name} ficou sem orçamento.`);
      } else {
        await ctx.save("category", { name, bucket: f.bucket.value, budgetCents }, "Categoria criada.");
      }
    } catch (err) {
      ctx.error(err);
    }
  });

  root.querySelectorAll("[data-delete]").forEach((btn) => btn.addEventListener("click", async () => {
    const c = ctx.ledger.categories.find((x) => x.id === btn.dataset.delete);
    try {
      canDeleteCategory(ctx.ledger, c.id); // regra em Go: lançamentos, recorrentes ou metas usando
    } catch (err) {
      ctx.error(err);
      return;
    }
    if (!confirm(`Apagar a categoria ${c.name}?`)) return;
    await ctx.remove("category", c, "Categoria apagada.");
  }));
}
