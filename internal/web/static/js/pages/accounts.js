// Contas e cartões: cadastro, saldo de cada um, fatura atual e próxima do
// cartão (pelo dia de fechamento) e o patrimônio mês a mês. As contas são de
// finance.Accounts e finance.NetWorth (Go, pelo WebAssembly); aqui só desenha.
import * as finance from "../finance.js";
import { barPct, centsInput, html, today } from "../ui.js";
import { pageHead } from "./dashboard.js";

export const ACCOUNT_KINDS = { corrente: "Conta corrente", poupanca: "Poupança", carteira: "Carteira", cartao: "Cartão de crédito" };

const ddmm = (date) => `${date.slice(8, 10)}/${date.slice(5, 7)}`;
// signedInput mostra centavos com sinal no campo ("-350,00").
const signedInput = (c) => (c < 0 ? "-" : "") + centsInput(Math.abs(c));

function invoiceBlock(title, inv, m) {
  return html`<div class="invoice ${inv.closed ? "closed" : ""}">
    <div class="row"><span>${title} · <strong>${inv.label}</strong></span><strong>${m(inv.totalCents)}</strong></div>
    <p class="small muted">${inv.closed ? "fechou" : "fecha"} ${ddmm(inv.closingDate)} · vence ${ddmm(inv.dueDate)}${inv.closed ? " · fechada" : ""}</p>
    ${inv.items.length ? html`<details><summary>${inv.items.length} lançamento(s)</summary>
      <ul class="plain">${inv.items.map((t) => html`<li class="row"><span>${t.description} <small class="muted">${ddmm(t.date)}</small></span>
        <span class="${t.kind === "entrada" ? "pos" : ""}">${t.kind === "entrada" ? "−" : ""}${m(t.amountCents)}</span></li>`)}</ul>
    </details>` : ""}
  </div>`;
}

// netWorthPanel é o patrimônio dos últimos 12 meses (também usado na Análise).
export function netWorthPanel(ledger) {
  if (!ledger.accounts.some((a) => !a.deleted)) {
    return html`<section class="panel"><h2>Patrimônio</h2>
      <p class="empty">Cadastre suas <a href="/contas">contas e cartões</a> para ver o patrimônio mês a mês.</p></section>`;
  }
  const m = finance.money;
  const points = finance.netWorth(ledger, today());
  const last = points[points.length - 1];
  const diff = last.netCents - points[0].netCents;
  const top = Math.max(1, ...points.map((p) => Math.abs(p.netCents)));
  return html`<section class="panel networth">
    <div class="chart-head">
      <h2>Patrimônio</h2>
      <strong class="${last.netCents < 0 ? "neg" : "pos"}">${m(last.netCents)}</strong>
    </div>
    <p class="small muted">Contas ${m(last.accountsCents)} − cartões em aberto ${m(last.cardsCents)} − dívidas ${m(last.debtsCents)}.
      ${diff >= 0 ? "Subiu" : "Caiu"} ${m(Math.abs(diff))} em 12 meses.</p>
    <div class="yearchart" role="img" aria-label="Patrimônio nos últimos 12 meses: de ${m(points[0].netCents)} para ${m(last.netCents)}">
      ${points.map((p) => html`<span class="col ${p === last ? "current" : ""}" title="${p.label}: ${m(p.netCents)}">
        <span class="bars"><span class="${p.netCents < 0 ? "out" : "in"}" style="height: ${barPct(Math.abs(p.netCents), top)}%"></span></span>
        <small>${p.label.slice(0, 3)}</small>
      </span>`)}
    </div>
    <details><summary>Ver em tabela</summary>
      <div class="table-wrap"><table>
        <thead><tr><th>Mês</th><th class="num">Contas</th><th class="num">Cartões</th><th class="num">Dívidas</th><th class="num">Patrimônio</th></tr></thead>
        <tbody>${points.map((p) => html`<tr><td>${p.label}</td><td class="num">${m(p.accountsCents)}</td><td class="num">${m(p.cardsCents)}</td>
          <td class="num">${m(p.debtsCents)}</td><td class="num ${p.netCents < 0 ? "neg" : "pos"}">${m(p.netCents)}</td></tr>`)}</tbody>
      </table></div>
    </details>
    <p class="small muted">Dívidas entram pelo saldo de hoje em todos os meses. Só lançamentos com conta mexem nos saldos.</p>
  </section>`;
}

export function render({ ledger, params }) {
  const m = finance.money;
  const v = finance.accounts(ledger, today());
  const editing = ledger.accounts.find((a) => a.id === params.get("editar") && !a.deleted);
  const e = editing || { kind: "corrente" };
  const isCard = e.kind === "cartao";
  return html`
    ${pageHead("Contas e cartões")}
    <section class="cards">
      <div class="card"><span class="muted">Nas contas</span><strong class="${v.accountsCents < 0 ? "neg" : "pos"}">${m(v.accountsCents)}</strong></div>
      <div class="card"><span class="muted">Em aberto nos cartões</span><strong class="${v.cardsOpenCents > 0 ? "neg" : ""}">${m(v.cardsOpenCents)}</strong></div>
    </section>

    <section class="panel">
      <h2>${editing ? "Editar conta" : "Nova conta ou cartão"}</h2>
      <form data-form="account" class="grid-form">
        <label class="wide">Nome <input name="name" required maxlength="60" value="${e.name || ""}" placeholder="Ex.: Nubank, Itaú, Carteira, Cartão Nubank"></label>
        <label>Tipo
          <select name="kind">
            ${Object.entries(ACCOUNT_KINDS).map(([k, label]) => html`<option value="${k}" ${k === e.kind ? "selected" : ""}>${label}</option>`)}
          </select>
        </label>
        <label class="half"><span data-initial-label>${isCard ? "Em aberto hoje (R$)" : "Saldo inicial (R$)"}</span>
          <input name="initial" inputmode="decimal" value="${editing ? signedInput(isCard ? -e.initialCents : e.initialCents) : ""}" placeholder="0,00"></label>
        <label class="half" data-card>Dia do fechamento <input type="number" name="closing_day" min="1" max="31" value="${e.closingDay || ""}"></label>
        <label class="half" data-card>Dia do vencimento <input type="number" name="due_day" min="1" max="31" value="${e.dueDay || ""}"></label>
        <div class="form-actions">
          <button type="submit">Salvar conta</button>
          ${editing ? html`<a href="/contas">Cancelar</a>` : ""}
        </div>
      </form>
      <p class="small muted">Saldo = saldo inicial + entradas − saídas lançadas na conta. No cartão, a compra feita depois do fechamento cai na fatura do mês seguinte. Escolha a conta ao lançar.</p>
    </section>

    <section class="panel">
      <h2>Suas contas</h2>
      ${v.accounts.length ? html`<ul class="accounts">
        ${v.accounts.map((a) => html`<li class="account">
          <div class="row">
            <span><strong>${a.name}</strong> <small class="muted">${ACCOUNT_KINDS[a.kind]}${a.kind === "cartao" ? ` · fecha dia ${a.closingDay}, vence dia ${a.dueDay}` : ""}</small></span>
            <strong class="${a.balanceCents < 0 ? "neg" : "pos"}">${a.kind === "cartao" ? (a.balanceCents < 0 ? `${m(-a.balanceCents)} em aberto` : m(a.balanceCents)) : m(a.balanceCents)}</strong>
          </div>
          ${a.current ? html`<div class="invoices">${invoiceBlock("Fatura atual", a.current, m)}${invoiceBlock("Próxima", a.next, m)}</div>` : ""}
          <div class="note-actions">
            <a href="/contas?editar=${a.id}">editar</a>
            <button type="button" class="link" data-delete="${a.id}">apagar</button>
          </div>
        </li>`)}
      </ul>` : html`<p class="empty">Nenhuma conta ainda. Cadastre onde seu dinheiro fica (banco, carteira) e seus cartões.</p>`}
    </section>
    ${netWorthPanel(ledger)}`;
}

export function bind(root, ctx) {
  const form = root.querySelector('[data-form="account"]');
  const f = form.elements;
  const toggle = () => {
    const card = f.kind.value === "cartao";
    form.querySelectorAll("[data-card]").forEach((el) => { el.hidden = !card; el.querySelector("input").required = card; });
    form.querySelector("[data-initial-label]").textContent = card ? "Em aberto hoje (R$)" : "Saldo inicial (R$)";
  };
  f.kind.addEventListener("change", toggle);
  toggle();

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const editing = ctx.ledger.accounts.find((a) => a.id === ctx.params.get("editar"));
    const card = f.kind.value === "cartao";
    try {
      const text = f.initial.value.trim();
      // O campo aceita sinal ("-350,00": conta no negativo); a regra de valor é a do Go.
      const value = text ? finance.parseMoney(text.replace(/^-/, "")) * (text.startsWith("-") ? -1 : 1) : 0;
      await ctx.save("account", {
        ...(editing ? { id: editing.id } : {}),
        name: f.name.value.trim(), kind: f.kind.value, initialCents: card ? -value : value,
        closingDay: card ? Number(f.closing_day.value) : undefined, dueDay: card ? Number(f.due_day.value) : undefined,
      }, "Conta salva.");
      if (editing) ctx.go("/contas");
    } catch (err) {
      ctx.error(err);
    }
  });
  root.querySelectorAll("[data-delete]").forEach((btn) => btn.addEventListener("click", async () => {
    const a = ctx.ledger.accounts.find((x) => x.id === btn.dataset.delete);
    if (!confirm(`Apagar "${a.name}"? Os lançamentos continuam, só sem conta.`)) return;
    await ctx.remove("account", a, "Conta apagada.");
  }));
}
