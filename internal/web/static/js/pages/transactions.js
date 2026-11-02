// Lançamentos do mês (design Rich): resumo, abas Lançamentos e Recorrências,
// busca e filtros (no endereço), a lista, janela para criar e editar e apagar
// com "Desfazer". Quem filtra, soma e valida é o Go (finance.*); aqui só desenha.
import * as finance from "../finance.js";
import { BUCKETS, closeModal, html, iconTile, newId, openModal, raw, shiftMonth, toast, today } from "../ui.js";
import { PLUS, metric, pageHead } from "./dashboard.js";

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
  pencil: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20h4L19 9l-4-4L4 16v4zM14 6l4 4"/></svg>'),
  copy: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 7a9 9 0 1 0 1 8M20 2v6h-6"/></svg>'),
  trash: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6"/></svg>'),
  close: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6L6 18M6 6l12 12"/></svg>'),
};

// "Notebook (3/6)" → descrição "Notebook" e selo "3/6".
function splitInstallment(description) {
  const m = description.match(/^(.*) \((\d+\/\d+)\)$/);
  return m ? [m[1], m[2]] : [description, ""];
}

// filters: o cartão de busca do design (campos com rótulo). Tudo vai para o endereço.
function filters(params, month, cats) {
  const value = (k) => params.get(k) || "";
  return html`<form data-form="filter" class="panel tx-filters" role="search">
    <label class="field tx-search">Buscar descrição <input type="search" name="busca" value="${value("busca")}" placeholder="Mercado, salário, cafezinho…"></label>
    <label class="field">Tipo <select name="tipo">
      ${[["", "Todos"], ["entrada", "Entradas"], ["saida", "Saídas"]].map(([v, label]) => html`<option value="${v}" ${value("tipo") === v ? "selected" : ""}>${label}</option>`)}
    </select></label>
    <label class="field">Categoria <select name="categoria">
      <option value="">Todas</option>
      ${cats.map((c) => html`<option value="${c.id}" ${c.id === params.get("categoria") ? "selected" : ""}>${c.name}</option>`)}
    </select></label>
    <label class="field">Valor mínimo <input name="min" inputmode="decimal" value="${value("min")}" placeholder="R$ 0" autocomplete="off"></label>
    <label class="field">Valor máximo <input name="max" inputmode="decimal" value="${value("max")}" placeholder="Sem limite" autocomplete="off"></label>
    <label class="check"><input type="checkbox" name="todos" value="1" ${params.get("todos") ? "checked" : ""}> Todo histórico</label>
    <a class="btn secondary" href="/lancamentos?mes=${month}">Limpar</a>
    <button type="submit" class="sr-only">Buscar</button>
  </form>`;
}

function emptyState(filtering, month) {
  return html`<div class="empty-state">
    <div class="empty-art" aria-hidden="true"><span></span><span><i></i><i></i></span></div>
    <strong>${filtering ? "Nada encontrado com esses filtros." : "Nenhum lançamento neste mês."}</strong>
    <span class="muted small">${filtering ? "Tente outra palavra, outra categoria ou marque Todos os meses." : `Lance os gastos e entradas de ${finance.monthLabel(month)} ou use “Copiar mês anterior” para trazer o salário e as contas fixas.`}</span>
    <div class="row-actions">
      ${filtering ? html`<a class="btn secondary" href="/lancamentos?mes=${month}">Limpar filtros</a>` : ""}
      <button type="button" data-action="new-tx">Novo lançamento</button>
    </div>
  </div>`;
}

const dmy = (d) => `${d.slice(8, 10)}/${d.slice(5, 7)}/${d.slice(0, 4)}`;

function txTable(txs, byId, accounts) {
  const m = finance.money;
  return html`<div class="table-wrap"><table class="tx-table">
    <thead><tr><th>Descrição / categoria</th><th class="tx-date">Data</th><th class="tx-acc">Conta</th><th class="num">Valor</th><th>Ações</th></tr></thead>
    <tbody>${txs.map((t) => {
      const c = byId[t.categoryId];
      const [desc, part] = splitInstallment(t.description);
      return html`<tr class="tx-row" data-edit="${t.id}" tabindex="0">
        <td class="primary"><span class="tx-main">${iconTile(c, t.kind, 32)}<span class="tx-text"><span class="tx-desc">${desc}</span>
          ${t.id.startsWith("rec-") ? html`<span class="mini-chip" title="Repete todo mês">↻ mensal</span>` : ""}
          ${part ? html`<span class="mini-chip" title="Parcela">${part}</span>` : ""}
          <span class="tx-sub faint">${c?.name || "Sem categoria"}<span class="tx-sub-date"> · ${dmy(t.date)}</span></span></span></span></td>
        <td class="tx-date">${dmy(t.date)}</td>
        <td class="tx-acc muted">${accounts[t.accountId] || "—"}</td>
        <td class="num amount ${t.kind === "entrada" ? "pos" : ""}">${t.kind === "entrada" ? "+" : "−"} ${m(t.amountCents)}</td>
        <td class="actions"><button type="button" class="icon-btn" data-open="${t.id}" aria-label="Editar ${desc}">${ICON.pencil}</button><button type="button" class="icon-btn danger" data-delete="${t.id}" aria-label="Apagar ${desc}">${ICON.trash}</button></td>
      </tr>`;
    })}</tbody>
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

  const recurringTab = params.get("aba") === "recorrentes";
  const recurringCount = ledger.recurring.filter((r) => !r.deleted).length;
  const tab = (on, href, label) => html`<a class="pill ${on ? "on" : ""}" href="${href}" aria-current="${on ? "page" : "false"}">${label}</a>`;

  return html`
    ${pageHead("Cada movimento conta.", month, { sub: "Entradas, saídas e a história do seu mês.", actions: html`<button type="button" data-action="new-tx">${PLUS}Novo lançamento</button>` })}

    <section class="cards metricgrid three ${filtering ? "filter-total" : ""}" role="status">
      ${metric(filtering ? "Entradas encontradas" : `Entradas · ${finance.monthLabel(month)}`, m(found.inCents), filtering ? `${found.count} ${found.count === 1 ? "lançamento encontrado" : "lançamentos encontrados"}` : "Receitas registradas")}
      ${metric(filtering ? "Saídas encontradas" : `Saídas · ${finance.monthLabel(month)}`, m(found.outCents), "Despesas e investimentos")}
      ${metric(filtering ? "Saldo" : "Saldo do mês", m(found.balanceCents), "Entradas menos saídas", { primary: true })}
    </section>

    <div class="tx-tabs">
      <nav class="pills" aria-label="Lançamentos ou recorrências">
        ${tab(!recurringTab, `/lancamentos?mes=${month}`, "Lançamentos")}
        ${tab(recurringTab, `/lancamentos?mes=${month}&aba=recorrentes`, `Recorrências (${recurringCount})`)}
      </nav>
      <button type="button" class="secondary" data-action="copy-previous" title="Copia salário e contas fixas do mês anterior">${ICON.copy}Copiar mês anterior</button>
    </div>

    ${recurringTab ? recurringPanel(ledger, byId, m) : html`
      ${filters(params, month, cats)}
      <section class="panel tx-card">
        <div class="panel-head"><h2>Seus lançamentos</h2><span class="faint small">${found.count} ${found.count === 1 ? "resultado" : "resultados"} · entradas ${m(found.inCents)} · saídas ${m(found.outCents)}</span></div>
        ${txs.length ? txTable(txs, byId, accounts) : emptyState(filtering, month)}
        ${found.count > txs.length ? html`<p class="small muted tx-more">Mostrando os ${txs.length} mais recentes de ${found.count}.</p>` : ""}
      </section>`}

    ${txForm(ledger, cats, defaultDate)}
    <button type="button" class="fab" data-action="new-tx" aria-label="Novo lançamento">${PLUS}</button>`;
}

// recurringPanel lista o que se repete todo mês (com pausar e apagar).
function recurringPanel(ledger, byId, m) {
  const list = ledger.recurring.filter((r) => !r.deleted).sort((a, b) => a.day - b.day);
  return html`<section class="panel" id="recorrentes">
    <div class="panel-head"><h2>Recorrências</h2><span class="faint small">Entram sozinhas todo mês, no dia marcado, em todos os aparelhos.</span></div>
    ${list.length ? "" : html`<p class="empty">Nenhuma recorrência ainda. Ao lançar salário, aluguel ou uma assinatura, marque “Repetir todo mês”.</p>`}
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
  // "+ Novo lançamento" do topo (de qualquer tela): chega com ?novo=1 e já abre a janela.
  if (ctx.params.get("novo")) {
    history.replaceState(null, "", "/lancamentos");
    open(null);
  }
  form.querySelectorAll("[data-kind]").forEach((b) => b.addEventListener("click", () => setKind(form, b.dataset.kind)));
  filterCategories(form);

  // Clique (ou Enter) numa linha, ou o lápis, abre a edição; o botão da lixeira apaga.
  root.querySelectorAll("[data-open]").forEach((b) => b.addEventListener("click", () => open(ctx.ledger.transactions.find((x) => x.id === b.dataset.open), b)));
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

  root.querySelector('[data-action="copy-previous"]').addEventListener("click", async () => {
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
  // Trocar de mês mantém a busca.
  root.querySelectorAll(".monthnav a").forEach((a) => {
    const u = new URL(a.href);
    for (const k of FILTER_KEYS) if (ctx.params.get(k)) u.searchParams.set(k, ctx.params.get(k));
    a.href = u.pathname + u.search;
  });
  const form = root.querySelector('[data-form="filter"]');
  if (!form) return; // aba Recorrências
  const apply = () => {
    // O "search" do Enter chega depois do submit, quando este formulário já foi redesenhado.
    if (!form.isConnected) return;
    const q = new URLSearchParams({ mes: ctx.month });
    for (const [k, v] of new FormData(form)) if (String(v).trim()) q.set(k, String(v).trim());
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
}
