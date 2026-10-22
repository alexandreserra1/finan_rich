// Lançamentos do mês: formulário, lista e "copiar do mês anterior".
import * as finance from "../finance.js";
import { BUCKETS, html, shiftMonth, today } from "../ui.js";
import { pageHead, totalsCards } from "./dashboard.js";

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

function filterPanel(params, month, cats, found) {
  const m = finance.money;
  const active = FILTER_KEYS.some((k) => params.get(k));
  const value = (k) => params.get(k) || "";
  return html`
    <form data-form="filter" class="grid-form filters" role="search">
      <label class="wide">Buscar <input type="search" name="busca" value="${value("busca")}" placeholder="Ex.: mercado, iFood, aluguel"></label>
      <label>Categoria
        <select name="categoria">
          <option value="">Todas</option>
          ${cats.map((c) => html`<option value="${c.id}" ${c.id === params.get("categoria") ? "selected" : ""}>${c.name}</option>`)}
        </select>
      </label>
      <label>Tipo
        <select name="tipo">
          <option value="">Entradas e saídas</option>
          <option value="entrada" ${value("tipo") === "entrada" ? "selected" : ""}>Só entradas</option>
          <option value="saida" ${value("tipo") === "saida" ? "selected" : ""}>Só saídas</option>
        </select>
      </label>
      <label class="half">Valor de (R$) <input name="min" inputmode="decimal" value="${value("min")}" placeholder="0,00"></label>
      <label class="half">até (R$) <input name="max" inputmode="decimal" value="${value("max")}" placeholder="sem limite"></label>
      <label class="check"><input type="checkbox" name="todos" value="1" ${params.get("todos") ? "checked" : ""}> Em todos os meses</label>
      <div class="form-actions">
        <button type="submit" class="secondary">Buscar</button>
        ${active ? html`<a href="/lancamentos?mes=${month}">Limpar</a>` : ""}
      </div>
    </form>
    ${active ? html`<p class="filter-total" role="status">
      <strong>${found.count} lançamento(s)</strong>
      <span>Entradas <b class="pos">${m(found.inCents)}</b></span>
      <span>Saídas <b>${m(found.outCents)}</b></span>
      <span>Saldo <b class="${found.balanceCents < 0 ? "neg" : "pos"}">${m(found.balanceCents)}</b></span>
      ${found.count > found.transactions.length ? html`<small class="muted">mostrando os ${found.transactions.length} mais recentes</small>` : ""}
    </p>` : ""}`;
}

export function render({ ledger, month, params }) {
  const cats = activeCategories(ledger);
  const byId = Object.fromEntries(ledger.categories.map((c) => [c.id, c]));
  const filter = readFilter(params, month);
  const found = finance.searchTransactions(ledger, filter);
  const txs = found.transactions;
  const filtering = FILTER_KEYS.some((k) => params.get(k));
  const v = finance.dashboard(ledger, month);
  const defaultDate = today().startsWith(month) ? today() : `${month}-01`;
  const m = finance.money;

  return html`
    ${pageHead("Lançamentos", month)}
    <section class="panel">
      <h2>Novo lançamento</h2>
      <form data-form="transaction" class="grid-form">
        <label>Tipo
          <select name="kind">
            <option value="saida">Saída (gasto)</option>
            <option value="entrada">Entrada (salário, renda)</option>
          </select>
        </label>
        <label class="wide">Descrição <input name="description" required placeholder="Ex.: Aluguel"></label>
        <label>Categoria
          <select name="category_id" required>
            ${cats.map((c) => html`<option value="${c.id}" data-bucket="${c.bucket}">${c.name} · ${BUCKETS[c.bucket]}</option>`)}
          </select>
        </label>
        <label class="half">Valor (R$) <input name="amount" required inputmode="decimal" placeholder="1.234,56"></label>
        <label class="half">Data <input type="date" name="date" value="${defaultDate}" required></label>
        <label class="check repeat"><input type="checkbox" name="repeat"> Repetir todo mês</label>
        <button type="submit">Adicionar</button>
      </form>
    </section>

    ${totalsCards(v, false)}

    <section class="panel">
      <div class="row">
        <h2>Lançamentos de ${filter.month ? finance.monthLabel(month) : "todos os meses"}</h2>
        <div class="row-actions tight">
          <a class="button secondary-link" href="/importar">Importar extrato</a>
          <button class="secondary" type="button" data-action="copy-previous" title="Copia salário e contas fixas do mês anterior">Copiar do mês anterior</button>
        </div>
      </div>
      ${filterPanel(params, month, cats, found)}
      ${txs.length ? html`
        <div class="table-wrap"><table class="cards-mobile">
          <thead><tr><th>Data</th><th>Descrição</th><th>Categoria</th><th class="num">Valor</th><th></th></tr></thead>
          <tbody>
            ${txs.map((t) => {
              const c = byId[t.categoryId] || { name: "Sem categoria", bucket: "" };
              return html`<tr>
                <td data-label="Data">${t.date.slice(8, 10)}/${t.date.slice(5, 7)}${filter.month ? "" : `/${t.date.slice(0, 4)}`}</td>
                <td data-label="Descrição" class="primary">${t.description}${t.id.startsWith("rec-") ? html` <small class="muted" title="Lançamento recorrente">↻</small>` : ""}</td>
                <td data-label="Categoria"><span class="tag b-${c.bucket}">${c.name}</span></td>
                <td data-label="Valor" class="num amount ${t.kind === "entrada" ? "pos" : ""}">${t.kind === "entrada" ? "+" : "−"}${m(t.amountCents)}</td>
                <td class="num actions"><button class="link" type="button" data-delete="${t.id}">apagar</button></td>
              </tr>`;
            })}
          </tbody>
        </table></div>`
        : filtering ? html`<p class="empty">Nada encontrado com esses filtros.</p>`
        : html`<p class="empty">Nenhum lançamento neste mês. Use o formulário acima ou copie do mês anterior.</p>`}
    </section>
    ${recurringPanel(ledger, byId, m)}`;
}

// recurringPanel lista o que se repete todo mês (com pausar e apagar).
function recurringPanel(ledger, byId, m) {
  const list = ledger.recurring.filter((r) => !r.deleted).sort((a, b) => a.day - b.day);
  if (!list.length) return "";
  return html`<section class="panel" id="recorrentes">
    <h2>Recorrentes</h2>
    <p class="small muted">Entram sozinhos todo mês, no dia marcado (em todos os aparelhos). Apagar um lançamento gerado não apaga a recorrência.</p>
    <ul class="recurring">
      ${list.map((r) => html`<li class="${r.paused ? "paused" : ""}">
        <span><strong>${r.description}</strong> <small class="muted">dia ${r.day} · ${(byId[r.categoryId] || {}).name || "Sem categoria"}${r.paused ? " · pausado" : ""}</small></span>
        <span class="${r.kind === "entrada" ? "pos" : ""}">${r.kind === "entrada" ? "+" : "−"}${m(r.amountCents)}</span>
        <span class="actions">
          <button type="button" class="link plain" data-pause="${r.id}">${r.paused ? "retomar" : "pausar"}</button>
          <button type="button" class="link" data-stop="${r.id}">apagar</button>
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

export function bind(root, ctx) {
  const form = root.querySelector('[data-form="transaction"]');
  form.elements.kind.addEventListener("change", () => filterCategories(form));
  filterCategories(form);

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = form.elements;
    try {
      const amountCents = finance.parseMoney(f.amount.value);
      if (f.repeat.checked) {
        // A recorrência gera este mês e os próximos (o gerador roda ao salvar).
        await ctx.save("recurring", {
          kind: f.kind.value, description: f.description.value.trim(), amountCents,
          categoryId: f.category_id.value, day: Number(f.date.value.slice(8, 10)), start: f.date.value.slice(0, 7),
        }, `Recorrente salvo: entra todo dia ${Number(f.date.value.slice(8, 10))}.`);
        return;
      }
      await ctx.save("transaction", {
        kind: f.kind.value, description: f.description.value.trim(), amountCents,
        categoryId: f.category_id.value, date: f.date.value,
      }, "Lançamento salvo.");
      if (!f.date.value.startsWith(ctx.month)) ctx.go(`/lancamentos?mes=${f.date.value.slice(0, 7)}`);
    } catch (err) {
      ctx.error(err);
    }
  });

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

  root.querySelectorAll("[data-delete]").forEach((btn) => btn.addEventListener("click", async () => {
    if (!confirm("Apagar este lançamento?")) return;
    const t = ctx.ledger.transactions.find((x) => x.id === btn.dataset.delete);
    await ctx.remove("transaction", t, "Lançamento apagado.");
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
  const form = root.querySelector('[data-form="filter"]');
  const apply = () => {
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
  form.addEventListener("change", (e) => { if (e.target.tagName === "SELECT" || e.target.type === "checkbox") apply(); });
  form.elements.busca.addEventListener("search", apply); // o "x" do campo limpa a busca
  // Trocar de mês mantém a busca.
  root.querySelectorAll(".monthnav a").forEach((a) => {
    const u = new URL(a.href);
    for (const k of FILTER_KEYS) if (ctx.params.get(k)) u.searchParams.set(k, ctx.params.get(k));
    a.href = u.pathname + u.search;
  });
}
