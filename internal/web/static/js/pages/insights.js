// Análise do mês (docs/ANALISE.md): ritmo de gastos, quanto ainda dá para
// gastar por dia, onde o dinheiro foi, onde dá para economizar e a taxa de
// economia. As contas são de finance.Insights (Go, pelo WebAssembly); aqui só desenha.
import * as finance from "../finance.js";
import { barPct, html, today } from "../ui.js";
import { chart } from "../charts.js";
import { budgetBars, pageHead } from "./dashboard.js";
import { bindNetWorth, netWorthPanel } from "./accounts.js";

// compareWith: com o que comparar o mês ("passado" ou "media", fica no endereço).
const compareWith = (params) => (params.get("comparar") === "media" ? "media" : "passado");

// paceChart desenha o gasto variável acumulado: este mês (área cheia), a
// comparação (tracejada: mês passado ou média de 3 meses) e a projeção até o
// fim do mês (pontilhada). A faixa de cima mostra o dia sob o mouse.
function paceChart(v, mode) {
  const m = finance.money;
  const ref = mode === "media" ? v.avg3Pace : v.lastPace;
  const refName = mode === "media" ? "Média de 3 meses" : "Mês passado";
  const days = Math.max(v.daysInMonth, ref.length);
  const projected = v.day ? Math.round((v.variableCents * v.daysInMonth) / v.day) : 0;
  const span = Math.max(1, v.daysInMonth - v.day);
  const proj = Array.from({ length: days }, (_, i) => (i + 1 < v.day || i + 1 > v.daysInMonth ? null
    : Math.round(v.variableCents + ((projected - v.variableCents) * (i + 1 - v.day)) / span)));
  const at = (list, i) => (i < list.length && list[i] != null ? m(list[i]) : "—");
  const hasRef = ref.some((c) => c > 0);
  return html`
    <div class="pace-chart">${chart({
      key: "ritmo", type: "line", height: 260, cursor: true,
      labels: Array.from({ length: days }, (_, i) => String(i + 1)),
      series: [
        { name: "Este mês", data: v.pace, color: "accent", fill: true, width: 2.5 },
        ...(hasRef ? [{ name: refName, data: ref, color: "muted", dash: true }] : []),
        { name: "Projeção", data: proj, color: "warn", dots: true },
      ],
      strip: { index: v.day - 1, render: (i) => html`<b>Dia ${i + 1}</b> · Este mês ${at(v.pace, i)}${hasRef ? ` · ${refName} ${at(ref, i)}` : ""}${proj[i] != null && i >= v.day ? ` · Projeção ${m(proj[i])}` : ""}` },
    }, `Gasto variável acumulado: ${m(v.variableCents)} até o dia ${v.day}; ${refName.toLowerCase()}, ${at(ref, v.day - 1)} até o mesmo dia.`)}</div>
    <details><summary>Ver em tabela</summary>
      <div class="table-wrap"><table>
        <thead><tr><th>Dia</th><th class="num">Este mês</th><th class="num">${refName}</th></tr></thead>
        <tbody>${Array.from({ length: days }, (_, i) => html`<tr><td>${i + 1}</td><td class="num">${at(v.pace, i)}</td><td class="num">${at(ref, i)}</td></tr>`)}</tbody>
      </table></div>
    </details>`;
}

// categoryBars: barra deste mês com um traço na média de 3 meses.
function categoryBars(v, m) {
  const top = Math.max(1, ...v.categories.map((c) => Math.max(c.cents, c.avg3Cents)));
  return html`<ul class="cat-avg">${v.categories.map((c) => {
    const over = c.avg3Cents && c.diffPct > 5;
    return html`<li>
      <span class="cat-avg-top"><span>${c.name}</span><span class="num">${m(c.cents)}
        ${c.avg3Cents ? html`<small class="${c.diffPct > 0 ? "neg" : c.diffPct < 0 ? "pos" : "faint"}">${c.diffPct > 0 ? "+" : c.diffPct < 0 ? "−" : ""}${Math.abs(c.diffPct)}%</small>` : html`<small class="faint">novo</small>`}</span></span>
      <span class="cat-avg-track"><span class="fill ${over ? "over" : ""}" style="width: ${barPct(c.cents, top)}%"></span>
        ${c.avg3Cents ? html`<i style="left: ${barPct(c.avg3Cents, top)}%" title="média de 3 meses: ${m(c.avg3Cents)}"></i>` : ""}</span>
    </li>`;
  })}</ul>`;
}

// rateColumns: a taxa de economia de cada mês, com a linha da meta.
function rateColumns(v) {
  const target = v.savingsTargetPct || 20;
  const top = Math.max(60, target + 10, ...v.savingsRates.map((r) => Math.abs(r.pct) + 10));
  const h = (pct) => `${(Math.min(Math.abs(pct), top) * 100) / top}%`;
  return html`<div class="rate-cols" role="img"
    aria-label="Taxa de economia por mês: ${v.savingsRates.filter((r) => r.hasIncome).map((r) => `${r.label} ${r.pct}%`).join(", ") || "sem renda lançada"}">
    <div class="rate-plot">
      <span class="rate-target" style="bottom: ${h(target)}" aria-hidden="true"><small>meta ${target}%</small></span>
      ${v.savingsRates.map((r) => html`<span class="rate-bar ${!r.hasIncome ? "none" : r.pct < 0 ? "neg" : r.pct >= target ? "ok" : "low"}" style="height: ${r.hasIncome ? h(r.pct) : "2px"}"></span>`)}
    </div>
    <div class="rate-labels" aria-hidden="true">${v.savingsRates.map((r) => html`<span><b class="num">${r.hasIncome ? `${r.pct}%` : "—"}</b><small class="faint">${r.label.slice(0, 3)}</small></span>`)}</div>
  </div>`;
}

export function render({ ledger, params }) {
  const v = finance.insights(ledger, today());
  const m = finance.money;
  const mode = compareWith(params);
  const maxPlace = v.places.length ? v.places[0].cents : 0;
  const refList = mode === "media" ? v.avg3Pace : v.lastPace;
  const refDay = refList[Math.min(v.day, refList.length) - 1] || 0;
  const vsRef = v.variableCents - refDay;
  const refText = mode === "media" ? "a média de 3 meses" : "o mês passado";
  const toggle = html`<div class="segmented" role="radiogroup" aria-label="Comparar com">
    <button type="button" role="radio" data-compare="passado" aria-checked="${mode === "passado"}">Mês passado</button>
    <button type="button" role="radio" data-compare="media" aria-checked="${mode === "media"}">Média 3 meses</button>
  </div>`;
  return html`
    ${pageHead("Análise", null, { kicker: `${v.label} · dia ${v.day} de ${v.daysInMonth}`, actions: toggle })}

    <section class="cards ins-kpis">
      <div class="card perday"><span>Pode gastar por dia</span>
        <strong class="${v.freeCents < 0 ? "neg" : "pos"}">${v.freeCents < 0 ? m(0) : m(v.perDayCents)}</strong>
        <span class="faint small">${v.freeCents < 0
          ? html`o mês já passou ${m(-v.freeCents)} do que cabe`
          : html`sobram ${m(v.freeCents)} até o fim do mês${v.incomeEstimated ? " (renda estimada pela média)" : ""}`}</span></div>
      <div class="card"><span>Gastos variáveis até hoje</span><strong>${m(v.variableCents)}</strong>
        <span class="small ${vsRef > 0 ? "neg" : "faint"}">${refDay ? `${m(Math.abs(vsRef))} ${vsRef > 0 ? "a mais" : "a menos"} que ${refText} até o dia ${v.day}` : `sem ${refText} para comparar`}</span></div>
      <div class="card"><span>Projeção do mês</span><strong>${m(v.projectedCents)}</strong>
        <span class="faint small">com ${m(v.fixedCents)} de contas fixas${v.lastMonthCents ? ` · mês passado: ${m(v.lastMonthCents)}` : ""}</span></div>
      <div class="card"><span>Taxa de economia</span><strong class="${v.savingsPct < 0 ? "neg" : v.savingsPct >= v.savingsTargetPct ? "pos" : ""}">${v.incomeCents ? `${v.savingsPct}%` : "—"}</strong>
        <span class="faint small">${v.incomeCents ? `média ${v.savingsAvgPct}% · meta da divisão ${v.savingsTargetPct}%` : "lance a renda do mês para calcular"}</span></div>
    </section>

    <section class="panel">
      <div class="panel-head"><h2>Ritmo do mês</h2><span class="faint small">só gastos variáveis; a projeção segue o ritmo de até hoje</span></div>
      ${paceChart(v, mode)}
    </section>

    <section class="ins-row">
      <section class="panel">
        <div class="panel-head"><h2>Onde dá para economizar</h2><a class="small" href="/chat">Falar com o Otávio</a></div>
        ${v.savings.length ? html`<ul class="savings">
          ${v.savings.map((s) => html`<li class="saving ${s.kind}">
            <div class="saving-top"><strong>${s.title}</strong>${!s.monthlyCents ? "" : s.kind === "ritmo"
              ? html`<span class="goal-chip neg">+${m(s.monthlyCents)} no mês</span>`
              : html`<span class="goal-chip">até ${m(s.monthlyCents)}/mês</span>`}</div>
            <p class="small muted">${s.text}</p>
          </li>`)}
        </ul>` : html`<p class="muted">Nada fora do normal neste mês. Bom sinal!</p>`}
      </section>
      <section class="panel">
        <div class="panel-head"><h2>Categorias contra a média</h2><span class="faint small">traço = média dos 3 meses anteriores</span></div>
        ${v.categories.length ? categoryBars(v, m) : html`<p class="empty">Nenhum gasto neste mês ainda.</p>`}
      </section>
    </section>

    ${v.budgets.length ? html`<section class="panel">
      <div class="panel-head"><h2>Orçamentos por categoria</h2><a class="small" href="/categorias">Ajustar tetos</a></div>
      ${budgetBars(v.budgets)}
    </section>` : ""}

    <section class="ins-row">
      <section class="panel" id="gastei">
        <div class="panel-head"><h2>Gastei onde</h2><span class="faint small">este mês</span></div>
        ${v.places.length ? html`<ul class="hbars">
          ${v.places.slice(0, 10).map((p) => html`<li>
            <span class="label">${p.name} <small class="muted">${p.count}×</small></span>
            <span class="track"><span class="fill place" style="width: ${barPct(p.cents, maxPlace)}%"></span></span>
            <span class="value">${m(p.cents)} <small class="muted">${p.sharePct}%</small></span>
          </li>`)}
        </ul>` : html`<p class="empty">Nenhum gasto neste mês ainda.</p>`}
      </section>
      <section class="panel">
        <div class="panel-head"><h2>Taxa de economia por mês</h2><span class="faint small">sobra + investido ÷ renda</span></div>
        ${rateColumns(v)}
      </section>
    </section>

    <section class="ins-row">
      ${v.subscriptions.length ? html`<section class="panel">
        <div class="panel-head"><h2>Assinaturas e contas fixas</h2></div>
        <ul class="plain subs">${v.subscriptions.map((s) => html`<li class="row">
          <span>${s.name}${s.registered ? "" : "*"}</span>
          <span class="num"><strong>${m(s.monthlyCents)}/mês</strong><small class="muted">${m(s.yearlyCents)}/ano</small></span>
        </li>`)}</ul>
        ${v.subscriptions.some((s) => !s.registered) ? html`<p class="small muted">* achada nos lançamentos (cobra todo mês, mas não está cadastrada como recorrente).</p>` : ""}
      </section>` : ""}
      ${v.biggest.length ? html`<section class="panel">
        <div class="panel-head"><h2>Maiores gastos do mês</h2></div>
        <ul class="plain">${v.biggest.map((t) => html`<li class="row"><span>${t.description} <small class="muted">${t.date.slice(8)}/${t.date.slice(5, 7)}</small></span><strong class="num">${m(t.amountCents)}</strong></li>`)}</ul>
      </section>` : ""}
    </section>
    ${netWorthPanel(ledger)}`;
}

export function bind(root, ctx) {
  bindNetWorth(root);
  root.querySelectorAll("[data-compare]").forEach((b) => b.addEventListener("click", () => {
    const url = new URL(location.href);
    if (b.dataset.compare === "media") url.searchParams.set("comparar", "media");
    else url.searchParams.delete("comparar");
    history.replaceState(null, "", url); // trocar a comparação não enche o histórico
    ctx.rerender();
  }));
}
