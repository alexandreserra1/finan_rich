// Lançamentos do mês (visual Esmeralda): resumo, busca e filtros (no endereço),
// lista agrupada por dia, janela para criar e editar, apagar com "Desfazer" e
// recorrentes. Quem filtra, soma e valida é o Go (finance.*); aqui só desenha.
import * as finance from "../finance.js";
import { BUCKETS, closeModal, html, iconTile, newId, openModal, raw, shiftMonth, toast, today } from "../ui.js";
import { PLUS, pageHead } from "./dashboard.js";

const ORDER = { renda: 0, necessidades: 1, desejos: 2, investimentos: 3 };

function activeCategories(ledger) {
  return ledger.categories.filter((c) => !c.deleted)
    .sort((a, b) => ORDER[a.bucket] - ORDER[b.bucket] || a.name.localeCompare(b.name, "pt-BR"));
}

function monthTransactions(ledger, month) {
  return ledger.transactions.filter((t) => !t.deleted && t.date.startsWith(month))
    .sort((a, b) => b.date.localeCompare(a.date) || b.updatedAt - a.updatedAt);
}

// Busca e filtros da lista: o estado fica no endereço (?busca=...&categoria=...),
// então voltar e avançar funcionam. Quem filtra e soma é o Go (finance.SearchTransactions).
// min e max continuam valendo no endereço (links antigos), sem campo na tela.
const FILTER_KEYS = ["busca", "categoria", "tipo", "min", "max", "todos"];

function readFilter(params, month) {
  const cents = (s) => { try { return s ? finance.parseMoney(s) : 0; } catch { return 0; } };
  return {
    text: params.get("busca") || "", categoryId: params.get("categoria") || "", kind: params.get("tipo") || "",
    minCents: cents(params.get("min")), maxCents: cents(params.get("max")),
    month: params.get("todos") ? "" : month,
  };
}

const ICON = {
  search: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="M21 21l-4.3-4.3"/></svg>'),
  download: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3"/></svg>'),
  trash: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6"/></svg>'),
  close: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6L6 18M6 6l12 12"/></svg>'),
};

const WEEKDAY = new Intl.DateTimeFormat("pt-BR", { weekday: "long", day: "numeric", month: "long" });
const WEEKDAY_YEAR = new Intl.DateTimeFormat("pt-BR", { weekday: "long", day: "numeric", month: "long", year: "numeric" });
const parseDay = (s) => { const [y, m, d] = s.split("-").map(Number); return new Date(y, m - 1, d); };
function dayLabel(date) {
  const now = today();
  const diff = Math.round((parseDay(now) - parseDay(date)) / 864e5);
  if (diff === 0) return "Hoje";
  if (diff === 1) return "Ontem";
  return (date.slice(0, 4) === now.slice(0, 4) ? WEEKDAY : WEEKDAY_YEAR).format(parseDay(date)).replace("-feira", "");
}

// "Notebook (3/6)" → descrição "Notebook" e selo "3/6".
function splitInstallment(description) {
  const m = description.match(/^(.*) \((\d+\/\d+)\)$/);
  return m ? [m[1], m[2]] : [description, ""];
}

function groupByDay(txs) {
  const groups = [];
  for (const t of txs) {
    let g = groups[groups.length - 1];
    if (!g || g.date !== t.date) groups.push((g = { date: t.date, rows: [], net: 0 }));
    g.rows.push(t);
    g.net += t.kind === "entrada" ? t.amountCents : -t.amountCents;
  }
  return groups;
}

function toolbar(params, month, cats) {
  const value = (k) => params.get(k) || "";
  const active = FILTER_KEYS.some((k) => params.get(k));
  return html`<form data-form="filter" class="tx-toolbar" role="search">
    <label class="search-field">${ICON.search}<input type="search" name="busca" value="${value("busca")}" placeholder="Buscar: mercado, iFood, aluguel…" aria-label="Buscar lançamentos"></label>
    <div class="segmented" role="radiogroup" aria-label="Tipo">
      ${[["", "Todos"], ["entrada", "Entradas"], ["saida", "Saídas"]].map(([v, label]) =>
        html`<label><input type="radio" name="tipo" value="${v}" ${value("tipo") === v ? "checked" : ""}><span>${label}</span></label>`)}
    </div>
    <select name="categoria" aria-label="Categoria">
      <option value="">Todas as categorias</option>
      ${cats.map((c) => html`<option value="${c.id}" ${c.id === params.get("categoria") ? "selected" : ""}>${c.name}</option>`)}
    </select>
    <label class="check"><input type="checkbox" name="todos" value="1" ${params.get("todos") ? "checked" : ""}> Todos os meses</label>
    ${active ? html`<a class="ghost-link" href="/lancamentos?mes=${month}">Limpar</a>` : ""}
    <button type="submit" class="sr-only">Buscar</button>
  </form>`;
}

function emptyState(filtering, month) {
  return html`<div class="empty-state">
    <div class="empty-art" aria-hidden="true"><span></span><span><i></i><i></i></span></div>
    <strong>${filtering ? "Nada encontrado com esses filtros." : "Nenhum lançamento neste mês."}</strong>
    <span class="muted small">${filtering ? "Tente outra palavra, outra categoria ou marque Todos os meses." : `Lance os gastos e entradas de ${finance.monthLabel(month)} ou traga o salário e as contas fixas do mês anterior.`}</span>
    <div class="row-actions">
      ${filtering ? html`<a class="btn secondary" href="/lancamentos?mes=${month}">Limpar filtros</a>`
        : html`<button type="button" class="secondary" data-action="copy-previous" title="Copia salário e contas fixas do mês anterior">Copiar do mês anterior</button>`}
      <button type="button" data-action="new-tx">Novo lançamento</button>
    </div>
  </div>`;
}

function txTable(txs, byId, accounts) {
  const m = finance.money;
  return html`<div class="table-wrap"><table class="tx-table">
    <thead><tr><th><span class="sr-only">Ícone</span></th><th>Descrição</th><th>Categoria</th><th>Conta</th><th class="num">Valor</th><th><span class="sr-only">Ações</span></th></tr></thead>
    ${groupByDay(txs).map((g) => html`<tbody>
      <tr class="day-row"><th colspan="4" scope="rowgroup">${dayLabel(g.date)}</th>
        <td colspan="2" class="num ${g.net < 0 ? "" : "pos"}">${g.net > 0 ? "+" : g.net < 0 ? "−" : ""}${m(Math.abs(g.net))}</td></tr>
      ${g.rows.map((t) => {
        const c = byId[t.categoryId];
        const [desc, part] = splitInstallment(t.description);
        return html`<tr class="tx-row" data-edit="${t.id}" tabindex="0">
          <td class="tx-ico">${iconTile(c, t.kind, 36)}</td>
          <td class="primary"><span class="tx-desc">${desc}</span>
            ${t.id.startsWith("rec-") ? html`<span class="mini-chip" title="Repete todo mês">↻ mensal</span>` : ""}
            ${part ? html`<span class="mini-chip" title="Parcela">${part}</span>` : ""}
            <span class="tx-sub faint">${c?.name || "Sem categoria"}</span></td>
          <td class="tx-cat"><span class="tag b-${c?.bucket || ""}"><i class="dot"></i>${c?.name || "Sem categoria"}</span></td>
          <td class="tx-acc muted">${accounts[t.accountId] || "—"}</td>
          <td class="num amount ${t.kind === "entrada" ? "pos" : ""}">${t.kind === "entrada" ? "+" : "−"}${m(t.amountCents)}</td>
          <td class="actions"><button type="button" class="icon-btn danger" data-delete="${t.id}" aria-label="Apagar ${desc}">${ICON.trash}</button></td>
        </tr>`;
      })}
    </tbody>`)}
  </table></div>`;
}

function txForm(ledger, cats, defaultDate) {
  const accounts = ledger.accounts.filter((a) => !a.deleted).sort((a, b) => a.name.localeCompare(b.name, "pt-BR"));
  return html`<div class="modal" id="tx-modal" hidden>
    <div class="modal-scrim" data-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="tx-title">
      <form data-form="transaction" class="modal-form" novalidate>
        <div class="modal-head"><h2 id="tx-title">Novo lançamento</h2><button type="button" class="icon-btn" data-close aria-label="Fechar">${ICON.close}</button></div>
        <input type="hidden" name="kind" value="saida">
        <div class="segmented big" role="radiogroup" aria-label="Tipo">
          <button type="button" role="radio" data-kind="saida" aria-checked="true">Saída (gasto)</button>
          <button type="button" role="radio" data-kind="entrada" aria-checked="false">Entrada</button>
        </div>
        <label class="field">Valor
          <span class="amount-field"><span>R$</span><input name="amount" inputmode="decimal" placeholder="0,00" autocomplete="off" autofocus></span>
          <span class="field-error" data-error="amount" hidden></span>
        </label>
        <label class="field">Descrição <input name="description" placeholder="Ex.: Aluguel" autocomplete="off">
          <span class="field-error" data-error="description" hidden></span>
        </label>
        <div class="field-grid">
          <label class="field">Categoria
            <select name="category_id">${cats.map((c) => html`<option value="${c.id}" data-bucket="${c.bucket}">${c.name} · ${BUCKETS[c.bucket]}</option>`)}</select>
          </label>
          <label class="field">Data <input type="date" name="date" value="${defaultDate}" required></label>
          ${accounts.length ? html`<label class="field">Conta ou cartão
            <select name="account_id"><option value="">Sem conta</option>${accounts.map((a) => html`<option value="${a.id}">${a.name}</option>`)}</select>
          </label>` : ""}
          <label class="field new-only">Parcelas <input type="number" name="installments" min="1" max="72" value="1" title="Compra parcelada: o valor é o total, dividido em parcelas mensais"></label>
        </div>
        <label class="check-card new-only"><input type="checkbox" name="repeat"><span><strong>Repetir todo mês</strong><span class="faint small">Salário, aluguel e assinaturas entram sozinhos.</span></span></label>
        <span class="field-error" data-error="both" hidden></span>
        <div class="modal-foot">
          <button type="button" class="ghost danger edit-only" data-action="delete-editing">Apagar</button>
          <span class="spacer"></span>
          <button type="button" class="secondary" data-close>Cancelar</button>
          <button type="submit">Adicionar</button>
        </div>
      </form>
    </div>
  </div>`;
}

export function render({ ledger, month, params }) {
  const cats = activeCategories(ledger);
  const byId = Object.fromEntries(ledger.categories.map((c) => [c.id, c]));
  const accounts = Object.fromEntries(ledger.accounts.map((a) => [a.id, a.name]));
  const filter = readFilter(params, month);
  const found = finance.searchTransactions(ledger, filter);
  const txs = found.transactions;
  const filtering = FILTER_KEYS.some((k) => params.get(k));
  const defaultDate = today().startsWith(month) ? today() : `${month}-01`;
  const m = finance.money;

  return html`
    ${pageHead("Lançamentos", month, { kicker: "Receitas e despesas", actions: html`
      <a class="btn secondary" href="/importar">${ICON.download}Importar extrato</a>
      <button type="button" data-action="new-tx">${PLUS}Novo lançamento</button>` })}

    <section class="sum-strip ${filtering ? "filter-total" : ""}" role="status">
      <div><span>${filtering ? "Encontrados" : "Lançamentos no mês"}</span><strong>${found.count}</strong></div>
      <div><span>Entradas</span><strong class="pos">${m(found.inCents)}</strong></div>
      <div><span>Saídas</span><strong>${m(found.outCents)}</strong></div>
      <div><span>Saldo</span><strong class="${found.balanceCents < 0 ? "neg" : "pos"}">${m(found.balanceCents)}</strong></div>
    </section>

    <section class="panel tx-card">
      ${toolbar(params, month, cats)}
      ${txs.length ? txTable(txs, byId, accounts) : emptyState(filtering, month)}
      ${found.count > txs.length ? html`<p class="small muted tx-more">Mostrando os ${txs.length} mais recentes de ${found.count}.</p>` : ""}
    </section>

    ${recurringPanel(ledger, byId, m)}
    ${txForm(ledger, cats, defaultDate)}
    <button type="button" class="fab" data-action="new-tx" aria-label="Novo lançamento">${PLUS}</button>`;
}

// recurringPanel lista o que se repete todo mês (com pausar e apagar).
function recurringPanel(ledger, byId, m) {
  const list = ledger.recurring.filter((r) => !r.deleted).sort((a, b) => a.day - b.day);
  if (!list.length) return "";
  return html`<section class="panel" id="recorrentes">
    <div class="panel-head"><h2>Recorrentes</h2><span class="faint small">Entram sozinhos todo mês, no dia marcado, em todos os aparelhos.</span></div>
    <ul class="recurring">
      ${list.map((r) => html`<li class="${r.paused ? "paused" : ""}">
        <span class="rec-text"><strong>${r.description}</strong><small class="faint">dia ${r.day} · ${(byId[r.categoryId] || {}).name || "Sem categoria"}${r.paused ? " · pausado" : ""}</small></span>
        <span class="num ${r.kind === "entrada" ? "pos" : ""}">${r.kind === "entrada" ? "+" : "−"}${m(r.amountCents)}</span>
        <span class="actions">
          <button type="button" class="ghost" data-pause="${r.id}">${r.paused ? "Retomar" : "Pausar"}</button>
          <button type="button" class="icon-btn danger" data-stop="${r.id}" aria-label="Apagar a recorrência ${r.description}">${ICON.trash}</button>
        </span>
      </li>`)}
    </ul>
  </section>`;
}

// filterCategories mostra só categorias de renda para entradas, e de gasto para saídas.
function filterCategories(form) {
  const wantIncome = form.elements.kind.value === "entrada";
  const select = form.elements.category_id;
  let first = null;
  for (const opt of select.options) {
    opt.hidden = opt.disabled = (opt.dataset.bucket === "renda") !== wantIncome;
    if (!opt.hidden && !first) first = opt;
  }
  if (select.selectedOptions[0]?.hidden && first) select.value = first.value;
}

function setKind(form, kind) {
  form.elements.kind.value = kind;
  form.querySelectorAll("[data-kind]").forEach((b) => b.setAttribute("aria-checked", String(b.dataset.kind === kind)));
  filterCategories(form);
}

// showErrors marca os campos com problema (borda vermelha e a mensagem embaixo).
function showErrors(form, errors) {
  form.querySelectorAll("[data-error]").forEach((el) => {
    const msg = errors[el.dataset.error];
    el.hidden = !msg;
    el.textContent = msg || "";
    const input = el.dataset.error === "both" ? null : form.elements[el.dataset.error];
    input?.setAttribute("aria-invalid", msg ? "true" : "false");
    input?.closest(".amount-field")?.classList.toggle("invalid", !!msg);
  });
}

export function bind(root, ctx) {
  const modal = root.querySelector("#tx-modal");
  const form = root.querySelector('[data-form="transaction"]');
  const f = form.elements;
  let editing = null;

  const open = (t, trigger) => {
    editing = t || null;
    form.reset();
    showErrors(form, {});
    modal.classList.toggle("editing", !!t);
    form.querySelector("#tx-title").textContent = t ? "Editar lançamento" : "Novo lançamento";
    form.querySelector('[type="submit"]').textContent = t ? "Salvar alterações" : "Adicionar";
    setKind(form, t?.kind || "saida");
    if (t) {
      f.amount.value = finance.money(t.amountCents).replace(/^R\$\s*/, "");
      f.description.value = t.description;
      f.category_id.value = t.categoryId;
      f.date.value = t.date;
      if (f.account_id) f.account_id.value = t.accountId || "";
    } else {
      f.date.value = today().startsWith(ctx.month) ? today() : `${ctx.month}-01`;
    }
    openModal(modal, trigger);
  };
  root.querySelectorAll('[data-action="new-tx"]').forEach((b) => b.addEventListener("click", () => open(null, b)));
  form.querySelectorAll("[data-kind]").forEach((b) => b.addEventListener("click", () => setKind(form, b.dataset.kind)));
  filterCategories(form);

  // Clique (ou Enter) numa linha abre a edição; o botão da lixeira apaga.
  root.querySelectorAll("tr[data-edit]").forEach((row) => {
    const edit = () => open(ctx.ledger.transactions.find((x) => x.id === row.dataset.edit), row);
    row.addEventListener("click", (e) => { if (!e.target.closest("button")) edit(); });
    row.addEventListener("keydown", (e) => { if (e.key === "Enter" && e.target === row) edit(); });
  });
  const wanted = ctx.params.get("editar");
  if (wanted) {
    const t = ctx.ledger.transactions.find((x) => x.id === wanted && !x.deleted);
    if (t) open(t);
  }

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const errors = {};
    let amountCents = 0;
    try {
      amountCents = finance.parseMoney(f.amount.value);
    } catch {}
    if (!(amountCents > 0)) errors.amount = "Valor inválido. Use o formato 1.234,56.";
    if (!f.description.value.trim()) errors.description = "Escreva uma descrição.";
    const parts = editing ? 1 : Number(f.installments.value) || 1;
    if (!editing && parts > 1 && f.repeat.checked) errors.both = "Escolha repetir todo mês ou parcelar, não os dois.";
    showErrors(form, errors);
    if (Object.keys(errors).length) return;
    const base = { kind: f.kind.value, description: f.description.value.trim(), amountCents,
      categoryId: f.category_id.value, date: f.date.value, accountId: f.account_id?.value || undefined };
    try {
      if (editing) {
        await ctx.save("transaction", { ...base, id: editing.id }, "Lançamento salvo.");
      } else if (parts > 1) {
        // As parcelas (regra em Go) têm ID fixo a partir do da compra: não duplicam entre aparelhos.
        const list = finance.installments({ id: newId(), ...base }, parts);
        for (const [i, p] of list.entries()) {
          const last = i === list.length - 1;
          await ctx.save("transaction", p, last ? `Compra parcelada em ${parts}x salva.` : null, { rerender: last });
        }
      } else if (f.repeat.checked) {
        // A recorrência gera este mês e os próximos (o gerador roda ao salvar).
        await ctx.save("recurring", {
          kind: base.kind, description: base.description, amountCents,
          categoryId: base.categoryId, day: Number(base.date.slice(8, 10)), start: base.date.slice(0, 7),
        }, `Recorrente salvo: entra todo dia ${Number(base.date.slice(8, 10))}.`);
        return;
      } else {
        await ctx.save("transaction", base, "Lançamento salvo.");
      }
      if (!base.date.startsWith(ctx.month)) ctx.go(`/lancamentos?mes=${base.date.slice(0, 7)}`);
    } catch (err) {
      ctx.error(err);
    }
  });

  // Apagar sem perguntar: o aviso traz "Desfazer" (grava de novo, não apagado).
  const remove = async (t) => {
    closeModal(modal);
    await ctx.remove("transaction", t);
    toast("Lançamento apagado.", () => ctx.save("transaction", { ...t, deleted: false }, "Lançamento de volta."));
  };
  root.querySelectorAll("[data-delete]").forEach((btn) => btn.addEventListener("click", () =>
    remove(ctx.ledger.transactions.find((x) => x.id === btn.dataset.delete))));
  form.querySelector('[data-action="delete-editing"]').addEventListener("click", () => editing && remove(editing));

  bindFilter(root, ctx);

  root.querySelectorAll("[data-pause]").forEach((btn) => btn.addEventListener("click", async () => {
    const r = ctx.ledger.recurring.find((x) => x.id === btn.dataset.pause);
    await ctx.save("recurring", { id: r.id, paused: !r.paused }, r.paused ? "Recorrente retomado." : "Recorrente pausado.");
  }));
  root.querySelectorAll("[data-stop]").forEach((btn) => btn.addEventListener("click", async () => {
    const r = ctx.ledger.recurring.find((x) => x.id === btn.dataset.stop);
    if (!confirm(`Parar de lançar "${r.description}" todo mês? Os lançamentos que já entraram continuam.`)) return;
    await ctx.remove("recurring", r, "Recorrente apagado.");
  }));

  root.querySelector('[data-action="copy-previous"]')?.addEventListener("click", async () => {
    const prev = monthTransactions(ctx.ledger, shiftMonth(ctx.month, -1));
    const current = monthTransactions(ctx.ledger, ctx.month);
    const exists = new Set(current.map((t) => `${t.description}|${t.categoryId}`));
    const [y, mo] = ctx.month.split("-").map(Number);
    const lastDay = new Date(y, mo, 0).getDate();
    let copied = 0;
    for (const t of prev.reverse()) {
      const key = `${t.description}|${t.categoryId}`;
      if (exists.has(key)) continue;
      exists.add(key);
      const day = String(Math.min(Number(t.date.slice(8, 10)), lastDay)).padStart(2, "0");
      await ctx.save("transaction", { kind: t.kind, description: t.description, amountCents: t.amountCents,
        categoryId: t.categoryId, date: `${ctx.month}-${day}` }, null, { rerender: false });
      copied++;
    }
    await ctx.refresh(`${copied} lançamento(s) copiado(s) do mês anterior.`);
  });
}

// bindFilter põe a busca no endereço (entra no histórico do navegador) e
// redesenha sem levar a tela para o topo.
function bindFilter(root, ctx) {
  const form = root.querySelector('[data-form="filter"]');
  const apply = () => {
    // O "search" do Enter chega depois do submit, quando este formulário já foi redesenhado.
    if (!form.isConnected) return;
    const q = new URLSearchParams({ mes: ctx.month });
    for (const [k, v] of new FormData(form)) if (String(v).trim()) q.set(k, String(v).trim());
    for (const k of ["min", "max"]) if (ctx.params.get(k)) q.set(k, ctx.params.get(k));
    const url = `/lancamentos?${q}`;
    if (url === location.pathname + location.search) return;
    const focused = document.activeElement?.name;
    history.pushState(null, "", url);
    ctx.rerender();
    if (focused) root.querySelector(`[data-form="filter"] [name="${focused}"]`)?.focus();
  };
  form.addEventListener("submit", (e) => { e.preventDefault(); apply(); });
  form.addEventListener("change", (e) => { if (e.target.name !== "busca") apply(); });
  form.elements.busca.addEventListener("search", apply); // o "x" do campo limpa a busca
  // Trocar de mês mantém a busca.
  root.querySelectorAll(".monthnav a").forEach((a) => {
    const u = new URL(a.href);
    for (const k of FILTER_KEYS) if (ctx.params.get(k)) u.searchParams.set(k, ctx.params.get(k));
    a.href = u.pathname + u.search;
  });
}
