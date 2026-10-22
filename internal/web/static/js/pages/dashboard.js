// Painel do mês: totais, divisão do salário, gastos por categoria e visão anual.
import * as finance from "../finance.js";
import { BUCKETS, barPct, html, raw, shiftMonth, today } from "../ui.js";
import { goalCards } from "./goals.js";
import * as welcome from "./welcome.js";

export function monthNav(month) {
  return html`<div class="monthnav">
    <a href="?mes=${shiftMonth(month, -1)}" aria-label="Mês anterior">‹</a>
    <strong>${finance.monthLabel(month)}</strong>
    <a href="?mes=${shiftMonth(month, 1)}" aria-label="Próximo mês">›</a>
  </div>`;
}

// pageHead é o título da tela, com a navegação de mês ao lado quando houver.
export function pageHead(title, month) {
  return html`<div class="page-head"><h1>${title}</h1>${month ? monthNav(month) : ""}</div>`;
}

const ARROW_UP = raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 19V5M5 12l7-7 7 7"/></svg>');
const ARROW_DOWN = raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12l7 7 7-7"/></svg>');

// totalsCards mostra entradas, saídas e saldo. No painel (withSavings) o saldo
// vira o cartão de destaque, com quanto da renda foi guardado.
export function totalsCards(v, withSavings) {
  const m = finance.money;
  const balanceClass = v.balanceCents < 0 ? "neg" : "pos";
  const income = html`<div class="card"><span class="muted"><span class="badge in">${ARROW_UP}</span>Entradas</span><strong class="pos">${m(v.inCents)}</strong></div>`;
  const spent = html`<div class="card"><span class="muted"><span class="badge out">${ARROW_DOWN}</span>Saídas</span><strong>${m(v.outCents)}</strong></div>`;
  if (withSavings) {
    return html`<section class="cards has-hero">
      <div class="card hero ${v.balanceCents < 0 ? "neg-balance" : ""}">
        <span>Saldo do mês</span>
        <strong>${m(v.balanceCents)}</strong>
        <span class="hero-note">Guardado (saldo + investido): <b>${v.savingsPct}%</b> da renda</span>
      </div>
      ${income}${spent}
    </section>`;
  }
  return html`<section class="cards">
    ${income}${spent}
    <div class="card"><span class="muted">Saldo</span><strong class="${balanceClass}">${m(v.balanceCents)}</strong></div>
  </section>`;
}

// budgetBars mostra quanto já foi usado de cada orçamento (Painel e Análise).
export function budgetBars(budgets) {
  const m = finance.money;
  const level = (b) => (b.usedPct >= 100 ? "over" : b.usedPct >= 80 ? "near" : "");
  return html`<div class="buckets budgets">
    ${budgets.map((b) => html`<div class="bucket b-${b.bucket}">
      <div class="row"><strong>${b.name}</strong><span class="small ${level(b) ? "neg" : "muted"}">${b.usedPct}%</span></div>
      <div class="bar"><span style="width: ${Math.min(b.usedPct, 100)}%" class="${level(b)}"></span></div>
      <div class="row small">
        <span>${m(b.spentCents)} de ${m(b.budgetCents)}</span>
        ${b.remainingCents < 0
          ? html`<span class="neg">estourou ${m(-b.remainingCents)}</span>`
          : html`<span class="muted">resta ${m(b.remainingCents)}</span>`}
      </div>
    </div>`)}
  </div>`;
}

export function render(ctx) {
  if (welcome.show(ctx)) return welcome.render(ctx);
  const { ledger, month } = ctx;
  const v = finance.dashboard(ledger, month);
  const m = finance.money;
  const maxCat = v.categories.length ? v.categories[0].spentCents : 0;
  const maxMonth = Math.max(0, ...v.year.map((t) => Math.max(t.inCents, t.outCents)));
  const yearIn = v.year.reduce((s, t) => s + t.inCents, 0);
  const yearOut = v.year.reduce((s, t) => s + t.outCents, 0);
  const monthAbbr = (key) => finance.monthLabel(key).slice(0, 3);

  const day = today();
  const alerts = month === day.slice(0, 7) ? finance.alerts(ledger, day) : [];
  const goalViews = finance.goals(ledger, day).filter((g) => !g.done || g.deadline >= month);
  return html`
    ${pageHead("Painel", month)}
    ${alerts.length ? html`<section class="alerts" aria-label="Avisos">
      ${alerts.map((a) => html`<a class="alert ${a.level}" href="${a.link}"><strong>${a.title}</strong><span>${a.text}</span></a>`)}
    </section>` : ""}
    ${totalsCards(v, true)}

    <section class="panel">
      <div class="row"><h2>Divisão do salário</h2><a class="small" href="/categorias">Ajustar percentuais</a></div>
      ${v.inCents === 0 ? html`<p class="muted">Lance suas entradas (salário, renda extra) para ver quanto cabe em cada grupo.</p>` : ""}
      <div class="buckets">
        ${v.buckets.map((b) => html`
          <div class="bucket b-${b.bucket}">
            <div class="row"><strong>${BUCKETS[b.bucket]}</strong><span class="muted small">${b.pct}% da renda</span></div>
            <div class="bar"><span style="width: ${Math.min(b.usedPct, 100)}%" class="${b.usedPct > 100 ? "over" : ""}"></span></div>
            <div class="row small">
              <span>${m(b.spentCents)} de ${m(b.plannedCents)}</span>
              ${b.remainingCents < 0
                ? html`<span class="neg">estourou ${m(-b.remainingCents)}</span>`
                : html`<span class="muted">resta ${m(b.remainingCents)}</span>`}
            </div>
          </div>`)}
      </div>
    </section>

    ${v.budgets.length ? html`<section class="panel" id="orcamentos">
      <div class="row"><h2>Orçamentos do mês</h2><a class="small" href="/categorias">Ajustar tetos</a></div>
      ${budgetBars(v.budgets)}
    </section>` : ""}

    ${goalViews.length ? html`<section class="panel">
      <div class="row"><h2>Metas</h2><a class="small" href="/metas">Ver todas</a></div>
      ${goalCards(goalViews.slice(0, 3), m, { compact: true })}
    </section>` : ""}

    <div class="grid-2">
    <section class="panel">
      <div class="row"><h2>Para onde o dinheiro foi</h2><a class="small" href="/analise">Ritmo e onde economizar</a></div>
      ${v.categories.length ? html`
        <ul class="hbars">
          ${v.categories.map((c) => html`
            <li>
              <span class="label">${c.name}</span>
              <span class="track"><span class="fill b-${c.bucket}" style="width: ${barPct(c.spentCents, maxCat)}%"></span></span>
              <span class="value">${m(c.spentCents)} <small class="muted">${c.sharePct}%</small></span>
            </li>`)}
        </ul>`
        : html`<p class="empty">Nenhuma saída neste mês. <a href="/lancamentos?mes=${month}">Lançar gastos</a></p>`}
    </section>

    <section class="panel">
      <div class="chart-head">
        <h2>Visão anual ${month.slice(0, 4)}</h2>
        <p class="legend"><span><span class="dot in"></span>entradas</span><span><span class="dot out"></span>saídas</span></p>
      </div>
      <div class="yearchart">
        ${v.year.map((t) => html`
          <a class="col ${t.month === month ? "current" : ""}" href="/?mes=${t.month}" title="${finance.monthLabel(t.month)}: entradas ${m(t.inCents)}, saídas ${m(t.outCents)}">
            <span class="bars">
              <span class="in" style="height: ${barPct(t.inCents, maxMonth)}%"></span>
              <span class="out" style="height: ${barPct(t.outCents, maxMonth)}%"></span>
            </span>
            <small>${monthAbbr(t.month)}</small>
          </a>`)}
      </div>
    </section>
    </div>

    <section class="panel">
      <h2>Resumo do ano</h2>
      <div class="table-wrap"><table>
        <thead><tr><th>Mês</th><th class="num">Entradas</th><th class="num">Saídas</th><th class="num">Saldo</th></tr></thead>
        <tbody>
          ${v.year.filter((t) => t.inCents || t.outCents).map((t) => html`
            <tr><td><a href="/?mes=${t.month}">${finance.monthLabel(t.month)}</a></td><td class="num">${m(t.inCents)}</td><td class="num">${m(t.outCents)}</td>
              <td class="num ${t.balanceCents < 0 ? "neg" : "pos"}">${m(t.balanceCents)}</td></tr>`)}
        </tbody>
        <tfoot><tr><th>Total</th><th class="num">${m(yearIn)}</th><th class="num">${m(yearOut)}</th>
          <th class="num ${yearIn - yearOut < 0 ? "neg" : "pos"}">${m(yearIn - yearOut)}</th></tr></tfoot>
      </table></div>
    </section>`;
}

export function bind(root, ctx) {
  if (welcome.show(ctx)) welcome.bind(root, ctx);
}
