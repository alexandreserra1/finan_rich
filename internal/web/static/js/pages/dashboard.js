// Painel do mês: totais, divisão do salário, gastos por categoria e visão anual.
import * as finance from "../finance.js";
import { BUCKETS, barPct, html, shiftMonth } from "../ui.js";

export function monthNav(month) {
  return html`<div class="monthnav">
    <a href="?mes=${shiftMonth(month, -1)}" aria-label="Mês anterior">‹</a>
    <strong>${finance.monthLabel(month)}</strong>
    <a href="?mes=${shiftMonth(month, 1)}" aria-label="Próximo mês">›</a>
  </div>`;
}

export function totalsCards(v, withSavings) {
  const m = finance.money;
  return html`<section class="cards">
    <div class="card"><span class="muted">Entradas</span><strong class="pos">${m(v.inCents)}</strong></div>
    <div class="card"><span class="muted">Saídas</span><strong>${m(v.outCents)}</strong></div>
    <div class="card"><span class="muted">Saldo</span><strong class="${v.balanceCents < 0 ? "neg" : "pos"}">${m(v.balanceCents)}</strong></div>
    ${withSavings ? html`<div class="card"><span class="muted">Guardado (saldo + investido)</span><strong>${v.savingsPct}%</strong></div>` : ""}
  </section>`;
}

export function render({ ledger, month }) {
  const v = finance.dashboard(ledger, month);
  const m = finance.money;
  const maxCat = v.categories.length ? v.categories[0].spentCents : 0;
  const maxMonth = Math.max(0, ...v.year.map((t) => Math.max(t.inCents, t.outCents)));
  const yearIn = v.year.reduce((s, t) => s + t.inCents, 0);
  const yearOut = v.year.reduce((s, t) => s + t.outCents, 0);
  const monthAbbr = (key) => finance.monthLabel(key).slice(0, 3);

  return html`
    ${monthNav(month)}
    ${totalsCards(v, true)}

    <section class="panel">
      <h2>Divisão do salário</h2>
      ${v.inCents === 0 ? html`<p class="muted">Lance suas entradas (salário, renda extra) para ver quanto cabe em cada grupo.</p>` : ""}
      <div class="buckets">
        ${v.buckets.map((b) => html`
          <div class="bucket b-${b.bucket}">
            <div class="row"><strong>${BUCKETS[b.bucket]}</strong><span class="muted">${b.pct}% da renda</span></div>
            <div class="bar"><span style="width: ${Math.min(b.usedPct, 100)}%" class="${b.usedPct > 100 ? "over" : ""}"></span></div>
            <div class="row small">
              <span>${m(b.spentCents)} de ${m(b.plannedCents)}</span>
              ${b.remainingCents < 0
                ? html`<span class="neg">estourou ${m(-b.remainingCents)}</span>`
                : html`<span class="muted">resta ${m(b.remainingCents)}</span>`}
            </div>
          </div>`)}
      </div>
      <p class="small muted">Ajuste os percentuais em <a href="/categorias">Categorias</a>.</p>
    </section>

    <section class="panel">
      <h2>Para onde o dinheiro foi</h2>
      ${v.categories.length ? html`
        <ul class="hbars">
          ${v.categories.map((c) => html`
            <li>
              <span class="label">${c.name}</span>
              <span class="track"><span class="fill b-${c.bucket}" style="width: ${barPct(c.spentCents, maxCat)}%"></span></span>
              <span class="value">${m(c.spentCents)} <small class="muted">${c.sharePct}%</small></span>
            </li>`)}
        </ul>`
        : html`<p class="muted">Nenhuma saída neste mês. <a href="/lancamentos?mes=${month}">Lançar gastos</a></p>`}
    </section>

    <section class="panel">
      <h2>Visão anual ${month.slice(0, 4)}</h2>
      <div class="yearchart">
        ${v.year.map((t) => html`
          <a class="col" href="/?mes=${t.month}" title="${finance.monthLabel(t.month)}: entradas ${m(t.inCents)}, saídas ${m(t.outCents)}">
            <span class="bars">
              <span class="in" style="height: ${barPct(t.inCents, maxMonth)}%"></span>
              <span class="out" style="height: ${barPct(t.outCents, maxMonth)}%"></span>
            </span>
            <small>${monthAbbr(t.month)}</small>
          </a>`)}
      </div>
      <p class="small"><span class="dot in"></span> entradas <span class="dot out"></span> saídas</p>
      <table>
        <thead><tr><th>Mês</th><th class="num">Entradas</th><th class="num">Saídas</th><th class="num">Saldo</th></tr></thead>
        <tbody>
          ${v.year.filter((t) => t.inCents || t.outCents).map((t) => html`
            <tr><td><a href="/?mes=${t.month}">${finance.monthLabel(t.month)}</a></td><td class="num">${m(t.inCents)}</td><td class="num">${m(t.outCents)}</td>
              <td class="num ${t.balanceCents < 0 ? "neg" : "pos"}">${m(t.balanceCents)}</td></tr>`)}
        </tbody>
        <tfoot><tr><th>Total</th><th class="num">${m(yearIn)}</th><th class="num">${m(yearOut)}</th>
          <th class="num ${yearIn - yearOut < 0 ? "neg" : "pos"}">${m(yearIn - yearOut)}</th></tr></tfoot>
      </table>
    </section>`;
}
