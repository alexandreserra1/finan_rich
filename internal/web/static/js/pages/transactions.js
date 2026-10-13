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

export function render({ ledger, month }) {
  const cats = activeCategories(ledger);
  const byId = Object.fromEntries(ledger.categories.map((c) => [c.id, c]));
  const txs = monthTransactions(ledger, month);
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
        <button type="submit">Adicionar</button>
      </form>
    </section>

    ${totalsCards(v, false)}

    <section class="panel">
      <div class="row">
        <h2>Lançamentos de ${finance.monthLabel(month)}</h2>
        <button class="secondary" type="button" data-action="copy-previous" title="Copia salário e contas fixas do mês anterior">Copiar do mês anterior</button>
      </div>
      ${txs.length ? html`
        <div class="table-wrap"><table class="cards-mobile">
          <thead><tr><th>Data</th><th>Descrição</th><th>Categoria</th><th class="num">Valor</th><th></th></tr></thead>
          <tbody>
            ${txs.map((t) => {
              const c = byId[t.categoryId] || { name: "Sem categoria", bucket: "" };
              return html`<tr>
                <td data-label="Data">${t.date.slice(8, 10)}/${t.date.slice(5, 7)}</td>
                <td data-label="Descrição" class="primary">${t.description}</td>
                <td data-label="Categoria"><span class="tag b-${c.bucket}">${c.name}</span></td>
                <td data-label="Valor" class="num amount ${t.kind === "entrada" ? "pos" : ""}">${t.kind === "entrada" ? "+" : "−"}${m(t.amountCents)}</td>
                <td class="num actions"><button class="link" type="button" data-delete="${t.id}">apagar</button></td>
              </tr>`;
            })}
          </tbody>
        </table></div>`
        : html`<p class="empty">Nenhum lançamento neste mês. Use o formulário acima ou copie do mês anterior.</p>`}
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
      await ctx.save("transaction", {
        kind: f.kind.value, description: f.description.value.trim(), amountCents,
        categoryId: f.category_id.value, date: f.date.value,
      }, "Lançamento salvo.");
      if (!f.date.value.startsWith(ctx.month)) ctx.go(`/lancamentos?mes=${f.date.value.slice(0, 7)}`);
    } catch (err) {
      ctx.error(err);
    }
  });

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
