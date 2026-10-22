// Análise do mês (docs/ANALISE.md): ritmo de gastos, quanto ainda dá para
// gastar por dia, onde o dinheiro foi e onde dá para economizar. As contas são
// de finance.Insights (Go, pelo WebAssembly); aqui só desenha.
import * as finance from "../finance.js";
import { barPct, html, raw, today } from "../ui.js";
import { budgetBars } from "./dashboard.js";
import { netWorthPanel } from "./accounts.js";

const W = 600, H = 220, PAD = { l: 8, r: 8, t: 12, b: 24 };

// paceChart desenha o gasto variável acumulado: este mês (linha cheia), o mês
// passado (tracejada, cinza) e a projeção até o fim do mês (pontilhada).
function paceChart(v) {
  const m = finance.money;
  const days = Math.max(v.daysInMonth, v.lastPace.length);
  const projected = v.day ? Math.round((v.variableCents * v.daysInMonth) / v.day) : 0;
  const top = Math.max(1, projected, ...v.pace, ...v.lastPace);
  const x = (d) => PAD.l + ((d - 1) / Math.max(1, days - 1)) * (W - PAD.l - PAD.r);
  const y = (c) => H - PAD.b - (c / top) * (H - PAD.t - PAD.b);
  const path = (s) => s.map((c, i) => `${i ? "L" : "M"}${x(i + 1).toFixed(1)},${y(c).toFixed(1)}`).join("");
  const ticks = [1, 10, 20, days].map((d) => `<text x="${x(d)}" y="${H - 6}" text-anchor="${d === 1 ? "start" : d === days ? "end" : "middle"}">${d}</text>`).join("");
  const svg = `<svg viewBox="0 0 ${W} ${H}" role="img" aria-label="Gasto variável acumulado: ${m(v.variableCents)} até o dia ${v.day}; no mês passado, ${m(v.lastMonthSameDayCents)} até o mesmo dia.">
    <line class="axis" x1="${PAD.l}" x2="${W - PAD.r}" y1="${H - PAD.b}" y2="${H - PAD.b}"/>
    <g class="ticks">${ticks}</g>
    ${v.lastPace.length ? `<path class="line last" d="${path(v.lastPace)}"/>` : ""}
    <path class="line proj" d="M${x(v.day)},${y(v.variableCents)}L${x(v.daysInMonth)},${y(projected)}"/>
    <path class="line now" d="${path(v.pace)}"/>
    <circle class="dot now" cx="${x(v.day)}" cy="${y(v.variableCents)}" r="4"/>
    <line class="cross" y1="${PAD.t}" y2="${H - PAD.b}" x1="-10" x2="-10"/>
  </svg>`;
  return html`
    <div class="pace-chart" data-days="${days}">
      <p class="legend">
        <span><span class="key now"></span>Este mês</span>
        <span><span class="key last"></span>Mês passado</span>
        <span><span class="key proj"></span>Projeção</span>
      </p>
      ${raw(svg)}
      <p class="chart-tip small" aria-live="polite" hidden></p>
    </div>
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
  const maxCat = v.categories.length ? v.categories[0].cents : 0;
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
      ${v.categories.length ? html`<ul class="hbars">
        ${v.categories.map((c) => html`<li>
          <span class="label">${c.name}</span>
          <span class="track"><span class="fill b-${c.bucket}" style="width: ${barPct(c.cents, maxCat)}%"></span></span>
          <span class="value">${m(c.cents)} ${diffBadge(c.diffPct, c.avg3Cents)}</span>
        </li>`)}
      </ul>` : html`<p class="empty">Nenhum gasto neste mês ainda.</p>`}
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

// bind liga a dica do gráfico: passar o dedo ou o mouse mostra o dia.
export function bind(root, ctx) {
  const box = root.querySelector(".pace-chart");
  if (!box) return;
  const v = finance.insights(ctx.ledger, today());
  const svg = box.querySelector("svg"), tip = box.querySelector(".chart-tip"), cross = box.querySelector(".cross");
  const days = Number(box.dataset.days);
  const show = (e) => {
    const r = svg.getBoundingClientRect();
    const fx = ((e.clientX - r.left) / r.width) * W;
    const d = Math.min(days, Math.max(1, Math.round(((fx - PAD.l) / (W - PAD.l - PAD.r)) * (days - 1)) + 1));
    const cx = PAD.l + ((d - 1) / Math.max(1, days - 1)) * (W - PAD.l - PAD.r);
    cross.setAttribute("x1", cx);
    cross.setAttribute("x2", cx);
    const now = d <= v.pace.length ? finance.money(v.pace[d - 1]) : "—";
    const last = d <= v.lastPace.length ? finance.money(v.lastPace[d - 1]) : "—";
    tip.textContent = `Dia ${d}: este mês ${now} · mês passado ${last}`;
    tip.hidden = false;
  };
  svg.addEventListener("pointermove", show);
  svg.addEventListener("pointerdown", show);
  svg.addEventListener("pointerleave", () => { tip.hidden = true; cross.setAttribute("x1", -10); cross.setAttribute("x2", -10); });
}
