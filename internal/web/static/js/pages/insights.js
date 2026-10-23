// Análise do mês (docs/ANALISE.md): ritmo de gastos, quanto ainda dá para
// gastar por dia, onde o dinheiro foi e onde dá para economizar. As contas são
// de finance.Insights (Go, pelo WebAssembly); aqui só desenha.
import * as finance from "../finance.js";
import { barPct, html, today } from "../ui.js";
import { chart } from "../charts.js";
import { budgetBars } from "./dashboard.js";
import { netWorthPanel } from "./accounts.js";

// paceChart desenha o gasto variável acumulado: este mês (área cheia), o mês
// passado (tracejada) e a projeção até o fim do mês (pontilhada).
function paceChart(v) {
  const m = finance.money;
  const days = Math.max(v.daysInMonth, v.lastPace.length);
  const projected = v.day ? Math.round((v.variableCents * v.daysInMonth) / v.day) : 0;
  const span = Math.max(1, v.daysInMonth - v.day);
  const proj = Array.from({ length: days }, (_, i) => (i + 1 < v.day || i + 1 > v.daysInMonth ? null
    : Math.round(v.variableCents + ((projected - v.variableCents) * (i + 1 - v.day)) / span)));
  return html`
    <div class="pace-chart">${chart({
      key: "ritmo", type: "line", height: 260,
      labels: Array.from({ length: days }, (_, i) => String(i + 1)),
      tip: (i) => `Dia ${i + 1}`,
      series: [
        { name: "Este mês", data: v.pace, color: "accent", fill: true },
        ...(v.lastPace.length ? [{ name: "Mês passado", data: v.lastPace, color: "muted", dash: true }] : []),
        { name: "Projeção", data: proj, color: "warn", dots: true },
      ],
    }, `Gasto variável acumulado: ${m(v.variableCents)} até o dia ${v.day}; no mês passado, ${m(v.lastMonthSameDayCents)} até o mesmo dia.`)}</div>
    <details><summary>Ver em tabela</summary>
      <div class="table-wrap"><table>
        <thead><tr><th>Dia</th><th class="num">Este mês</th><th class="num">Mês passado</th></tr></thead>
        <tbody>${Array.from({ length: days }, (_, i) => html`<tr><td>${i + 1}</td>
          <td class="num">${i < v.pace.length ? m(v.pace[i]) : "—"}</td>
          <td class="num">${i < v.lastPace.length ? m(v.lastPace[i]) : "—"}</td></tr>`)}</tbody>
      </table></div>
    </details>`;
}

function diffBadge(diffPct, avgCents) {
  if (!avgCents) return "";
  if (diffPct === 0) return html`<small class="muted" title="igual à média">=</small>`;
  return html`<small class="${diffPct > 0 ? "neg" : "pos"}" title="contra a média dos 3 meses anteriores">${diffPct > 0 ? "+" : "−"}${Math.abs(diffPct)}%</small>`;
}

export function render({ ledger }) {
  const v = finance.insights(ledger, today());
  const m = finance.money;
  const maxPlace = v.places.length ? v.places[0].cents : 0;
  const vsLast = v.variableCents - v.lastMonthSameDayCents;
  return html`
    <div class="page-head"><h1>Análise</h1><p class="muted">${v.label} · dia ${v.day} de ${v.daysInMonth}</p></div>

    <section class="cards">
      <div class="card"><span class="muted">Pode gastar por dia</span>
        <strong class="${v.freeCents < 0 ? "neg" : "pos"}">${v.freeCents < 0 ? m(0) : m(v.perDayCents)}</strong>
        <span class="small muted">${v.freeCents < 0
          ? html`o mês já passou ${m(-v.freeCents)} do que cabe`
          : html`sobram ${m(v.freeCents)} até o fim do mês${v.incomeEstimated ? " (renda estimada pela média)" : ""}`}</span></div>
      <div class="card"><span class="muted">Gastos variáveis até hoje</span><strong>${m(v.variableCents)}</strong>
        <span class="small ${vsLast > 0 ? "neg" : "muted"}">${vsLast > 0 ? m(vsLast) + " a mais" : m(-vsLast) + " a menos"} que no mês passado até o dia ${v.day}</span></div>
      <div class="card"><span class="muted">Projeção do mês</span><strong>${m(v.projectedCents)}</strong>
        <span class="small muted">com ${m(v.fixedCents)} de contas fixas${v.lastMonthCents ? html` · mês passado: ${m(v.lastMonthCents)}` : ""}</span></div>
    </section>

    <section class="panel">
      <h2>Ritmo do mês</h2>
      <p class="small muted">Só gastos variáveis (as contas fixas não entram, para a linha não dar saltos). A projeção segue o ritmo de até hoje.</p>
      ${paceChart(v)}
    </section>

    <section class="panel">
      <h2>Onde dá para economizar</h2>
      ${v.savings.length ? html`<ul class="savings">
        ${v.savings.map((s) => html`<li class="saving ${s.kind}">
          <div class="row"><strong>${s.title}</strong>${!s.monthlyCents ? "" : s.kind === "ritmo"
            ? html`<span class="neg">+${m(s.monthlyCents)} no mês</span>`
            : html`<span class="pos">até ${m(s.monthlyCents)}/mês</span>`}</div>
          <p class="small">${s.text}</p>
        </li>`)}
      </ul>` : html`<p class="muted">Nada fora do normal neste mês. Bom sinal!</p>`}
    </section>

    ${v.budgets.length ? html`<section class="panel">
      <div class="row"><h2>Orçamentos por categoria</h2><a class="small" href="/categorias">Ajustar tetos</a></div>
      ${budgetBars(v.budgets)}
    </section>` : ""}

    <div class="grid-2">
    <section class="panel">
      <h2>Gastei onde</h2>
      ${v.places.length ? html`<ul class="hbars">
        ${v.places.slice(0, 12).map((p) => html`<li>
          <span class="label">${p.name} <small class="muted">${p.count}×</small></span>
          <span class="track"><span class="fill place" style="width: ${barPct(p.cents, maxPlace)}%"></span></span>
          <span class="value">${m(p.cents)} <small class="muted">${p.sharePct}%</small></span>
        </li>`)}
      </ul>` : html`<p class="empty">Nenhum gasto neste mês ainda.</p>`}
    </section>

    <section class="panel">
      <h2>Categorias contra a média</h2>
      <p class="small muted">A porcentagem compara com a média dos 3 meses anteriores.</p>
      ${v.categories.length ? html`${chart({
        key: "media", type: "bar", horizontal: true, height: 44 + v.categories.length * 40,
        labels: v.categories.map((c) => c.name),
        tip: (i) => v.categories[i].name + (v.categories[i].avg3Cents ? ` (${v.categories[i].diffPct > 0 ? "+" : ""}${v.categories[i].diffPct}% contra a média)` : ""),
        series: [
          { name: "Este mês", data: v.categories.map((c) => c.cents), color: "accent" },
          { name: "Média de 3 meses", data: v.categories.map((c) => c.avg3Cents), color: "border-strong" },
        ],
      }, `Gastos por categoria neste mês contra a média dos 3 meses anteriores.`)}
      <details><summary>Ver em tabela</summary>
        <div class="table-wrap"><table>
          <thead><tr><th>Categoria</th><th class="num">Este mês</th><th class="num">Média</th><th class="num">Diferença</th></tr></thead>
          <tbody>${v.categories.map((c) => html`<tr><td>${c.name}</td><td class="num">${m(c.cents)}</td><td class="num">${m(c.avg3Cents)}</td><td class="num">${diffBadge(c.diffPct, c.avg3Cents)}</td></tr>`)}</tbody>
        </table></div>
      </details>` : html`<p class="empty">Nenhum gasto neste mês ainda.</p>`}
    </section>
    </div>

    <div class="grid-2">
    ${v.subscriptions.length ? html`<section class="panel">
      <h2>Assinaturas e contas fixas</h2>
      <ul class="plain subs">${v.subscriptions.map((s) => html`<li class="row">
        <span>${s.name}${s.registered ? "" : "*"}</span>
        <span class="num"><strong>${m(s.monthlyCents)}/mês</strong><small class="muted">${m(s.yearlyCents)}/ano</small></span>
      </li>`)}</ul>
      ${v.subscriptions.some((s) => !s.registered) ? html`<p class="small muted">* achada nos lançamentos (cobra todo mês, mas não está cadastrada como recorrente).</p>` : ""}
    </section>` : ""}

    ${v.biggest.length ? html`<section class="panel">
      <h2>Maiores gastos do mês</h2>
      <ul class="plain">${v.biggest.map((t) => html`<li class="row"><span>${t.description} <small class="muted">${t.date.slice(8)}/${t.date.slice(5, 7)}</small></span><strong>${m(t.amountCents)}</strong></li>`)}</ul>
    </section>` : ""}
    </div>
    ${netWorthPanel(ledger)}`;
}
