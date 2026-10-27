// Painel do mês (visual Esmeralda): avisos, saldo, fluxo de caixa do ano,
// divisão do salário, para onde foi o dinheiro, últimos lançamentos, próximos
// vencimentos e dívidas. Os números vêm do Go (finance.*); aqui só desenha.
import * as finance from "../finance.js";
import { BUCKETS, barPct, html, iconTile, raw, shiftMonth, today } from "../ui.js";
import { goalCards } from "./goals.js";
import { chart } from "../charts.js";
import * as welcome from "./welcome.js";

export function monthNav(month) {
  return html`<div class="monthnav">
    <a href="?mes=${shiftMonth(month, -1)}" aria-label="Mês anterior">‹</a>
    <strong>${finance.monthLabel(month)}</strong>
    <a href="?mes=${shiftMonth(month, 1)}" aria-label="Próximo mês">›</a>
  </div>`;
}

// pageHead é o título da tela (com o "kicker" acima), com a navegação de mês e
// os botões à direita quando houver.
export function pageHead(title, month, { kicker = "", actions = "" } = {}) {
  return html`<div class="page-head">
    <div class="title">${kicker ? html`<span class="kicker">${kicker}</span>` : ""}<h1>${title}</h1></div>
    ${month || actions ? html`<div class="actions">${month ? monthNav(month) : ""}${actions}</div>` : ""}
  </div>`;
}

export const PLUS = raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14"/></svg>');

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

const MONTHS = ["jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"];
const DAY = 864e5;
const isoDay = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
const parseDay = (s) => { const [y, m, d] = s.split("-").map(Number); return new Date(y, m - 1, d); };
// nextOn: a próxima data (hoje ou depois) que cai no dia do mês "day".
function nextOn(day, from) {
  for (let k = 0; k < 2; k++) {
    const last = new Date(from.getFullYear(), from.getMonth() + k + 1, 0).getDate();
    const d = new Date(from.getFullYear(), from.getMonth() + k, Math.min(day, last));
    if (d >= from) return d;
  }
  return null;
}
const inDays = (n) => (n === 0 ? "hoje" : n === 1 ? "amanhã" : `em ${n} dias`);
const shortDate = (s, now) => {
  const n = Math.round((parseDay(isoDay(now)) - parseDay(s)) / DAY);
  return n === 0 ? "hoje" : n === 1 ? "ontem" : `${Number(s.slice(8))} ${MONTHS[Number(s.slice(5, 7)) - 1]}`;
};
const thousands = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL", notation: "compact", maximumFractionDigits: 1 });

// upcoming: parcelas de dívidas, faturas de cartão e contas fixas que vencem
// a partir de hoje, das mais próximas para as mais distantes.
function upcoming(ledger, now) {
  const from = parseDay(isoDay(now));
  const list = [];
  for (const d of ledger.debts) {
    if (d.deleted || !d.dueDay || d.balanceCents <= 0) continue;
    const at = nextOn(d.dueDay, from);
    if (at) list.push({ name: d.name, at, cents: d.minPaymentCents, href: "/dividas" });
  }
  for (const r of ledger.recurring) {
    if (r.deleted || r.paused || r.kind !== "saida" || (r.end && r.end < isoDay(from).slice(0, 7))) continue;
    const at = nextOn(r.day, from);
    if (at) list.push({ name: r.description, at, cents: r.amountCents, href: "/lancamentos" });
  }
  for (const a of finance.accounts(ledger, isoDay(now)).accounts) {
    const inv = a.current;
    if (a.kind !== "cartao" || !inv || inv.totalCents <= 0) continue;
    list.push({ name: `Fatura ${a.name}`, at: parseDay(inv.dueDate), cents: inv.totalCents, href: "/contas" });
  }
  return list.filter((x) => x.at >= from).sort((a, b) => a.at - b.at).slice(0, 4)
    .map((x) => ({ ...x, days: Math.round((x.at - from) / DAY) }));
}

let painted = false; // os cartões sobem suavemente só na primeira vez

export function render(ctx) {
  if (welcome.show(ctx)) return welcome.render(ctx);
  const { ledger, month } = ctx;
  const v = finance.dashboard(ledger, month);
  const m = finance.money;
  const prevMonth = shiftMonth(month, -1);
  const prev = finance.dashboard(ledger, prevMonth);
  const now = new Date();
  const day = today();
  const rise = painted ? "" : "rise";
  painted = true;

  const alerts = month === day.slice(0, 7) ? finance.alerts(ledger, day) : [];
  const briefs = month === day.slice(0, 7) ? finance.briefings(ledger, day) : [];
  const goalViews = finance.goals(ledger, day).filter((g) => !g.done || g.deadline >= month);
  const inMonth = ledger.transactions.filter((t) => !t.deleted && t.date.startsWith(month));
  const count = (kind) => inMonth.filter((t) => t.kind === kind).length;
  const outPct = v.inCents ? Math.round((v.outCents * 100) / v.inCents) : 0;
  const delta = v.balanceCents - prev.balanceCents;
  const prevName = finance.monthLabel(prevMonth).split(" de ")[0];
  const yearIn = v.year.reduce((s, t) => s + t.inCents, 0);
  const yearOut = v.year.reduce((s, t) => s + t.outCents, 0);
  const spentTotal = v.buckets.reduce((s, b) => s + b.spentCents, 0);
  const maxCat = v.categories.length ? v.categories[0].spentCents : 0;
  const cats = new Map(ledger.categories.map((c) => [c.id, c]));
  const recent = ledger.transactions.filter((t) => !t.deleted && t.date <= day)
    .sort((a, b) => b.date.localeCompare(a.date) || b.updatedAt - a.updatedAt).slice(0, 6);
  const due = upcoming(ledger, now);
  const dv = finance.debts(ledger, 0);
  const current = Math.max(0, v.year.findIndex((t) => t.month === month));
  const yearHas = (i) => v.year[i].inCents > 0 || v.year[i].outCents > 0;
  const stripFor = (i) => {
    const t = v.year[i];
    return html`<strong>${finance.monthLabel(t.month)}</strong>
      <span>Entradas <b>${m(t.inCents)}</b></span>
      <span>Saídas <b>${m(t.outCents)}</b></span>
      <span>Saldo <b class="${t.balanceCents < 0 ? "neg" : "pos"}">${t.balanceCents > 0 ? "+" : ""}${m(t.balanceCents)}</b></span>`;
  };

  return html`
    ${pageHead("Painel", month, { kicker: "Visão do mês", actions: html`<a class="btn" href="/lancamentos">${PLUS}Novo lançamento</a>` })}

    ${alerts.length ? html`<section class="alerts" aria-label="Avisos">
      ${alerts.map((a) => html`<a class="alert ${a.level}" href="${a.link}"><img class="face" src="/static/img/avatar-${a.agent.animal}.png" alt="" width="24" height="24"><b class="who">${a.agent.name}</b><strong>${a.title}</strong><span>${a.text}</span></a>`)}
    </section>` : ""}

    <section class="cards has-hero dash-kpis">
      <div class="card hero ${rise}">
        <div class="hero-top">
          <span class="hero-label">Saldo do mês</span>
          ${prev.inCents || prev.outCents ? html`<span class="chip ${delta < 0 ? "neg" : ""}">${delta >= 0 ? "+" : "−"}${m(Math.abs(delta))} vs. ${prevName}</span>` : ""}
        </div>
        <div class="hero-main">
          <strong class="${v.balanceCents < 0 ? "neg" : ""}">${m(v.balanceCents)}</strong>
          <span class="hero-note">Guardado (saldo + investido): <b>${v.savingsPct}% da renda</b></span>
        </div>
        ${v.inCents ? html`<div class="hero-split">
          <div class="split-bar" aria-hidden="true"><span style="width: ${Math.min(outPct, 100)}%"></span><span></span></div>
          <div class="hero-legend"><span>${outPct}% da renda já saiu</span>
            <span>${v.balanceCents >= 0 ? `sobra ${m(v.balanceCents)}` : `faltam ${m(-v.balanceCents)}`}</span></div>
        </div>` : html`<span class="hero-note">Lance o salário e as outras entradas para ver quanto da renda já saiu.</span>`}
      </div>
      <div class="card kpi ${rise}" style="animation-delay: .05s">
        <span class="kpi-label"><span class="kpi-icon in">${raw('<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 19V5M5 12l7-7 7 7"/></svg>')}</span>Entradas</span>
        <strong>${m(v.inCents)}</strong>
        <span class="faint small">${count("entrada")} ${count("entrada") === 1 ? "entrada" : "entradas"} no mês</span>
      </div>
      <div class="card kpi ${rise}" style="animation-delay: .1s">
        <span class="kpi-label"><span class="kpi-icon out">${raw('<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12l7 7 7-7"/></svg>')}</span>Saídas</span>
        <strong>${m(v.outCents)}</strong>
        <span class="faint small">${v.inCents ? `${outPct}% do que entrou` : `${count("saida")} ${count("saida") === 1 ? "saída" : "saídas"} no mês`}</span>
      </div>
    </section>

    <section class="dash-row dash-row2">
      <div class="panel flow">
        <div class="panel-head">
          <div><h2>Fluxo de caixa · ${month.slice(0, 4)}</h2><span class="faint small">Entradas e saídas por mês, em R$</span></div>
          <p class="legend"><span><i class="sq" style="--c: var(--accent)"></i>Entradas</span><span><i class="sq" style="--c: var(--neg)"></i>Saídas</span></p>
        </div>
        ${chart({
          key: "fluxo", type: "bar", height: 220, legend: false, dashed: true, bar: { max: 11, radius: 3 },
          labels: v.year.map((t) => MONTHS[Number(t.month.slice(5)) - 1]),
          series: [
            { name: "Entradas", data: v.year.map((t, i) => (yearHas(i) ? t.inCents : null)), color: "accent" },
            { name: "Saídas", data: v.year.map((t, i) => (yearHas(i) ? t.outCents : null)), color: "neg" },
          ],
          strip: { index: current, render: stripFor, valid: yearHas },
        }, `Entradas e saídas por mês em ${month.slice(0, 4)}: ${m(yearIn)} de entradas e ${m(yearOut)} de saídas.`)}
      </div>

      <div class="panel salary">
        <div class="panel-head"><h2>Divisão do salário</h2><a class="small" href="/categorias">Ajustar</a></div>
        <div class="salary-top">
          ${spentTotal ? chart({
            key: "divisao", type: "doughnut", height: 118, size: 118, cutout: "76%",
            labels: v.buckets.map((b) => BUCKETS[b.bucket]),
            series: [{ name: "Gasto", data: v.buckets.map((b) => b.spentCents), colors: v.buckets.map((b) => b.bucket) }],
            center: { value: thousands.format(spentTotal / 100), text: "gastos no mês" },
          }, `Gastos do mês por grupo: ${v.buckets.map((b) => `${BUCKETS[b.bucket]} ${m(b.spentCents)}`).join(", ")}.`) : ""}
          <p class="small muted">${v.inCents
            ? html`Regra <b>${v.buckets.map((b) => b.pct).join("/")}</b> sobre ${m(v.inCents)} de renda.`
            : "Lance suas entradas (salário, renda extra) para ver quanto cabe em cada grupo."}</p>
        </div>
        <div class="bucket-list">
          ${v.buckets.map((b) => html`<div class="bucket-line b-${b.bucket}">
            <div class="row"><span class="name"><i class="sq"></i>${BUCKETS[b.bucket]}</span><span class="muted num">${b.usedPct}% usado</span></div>
            <div class="bar"><span style="width: ${Math.min(b.usedPct, 100)}%" class="${b.usedPct > 100 ? "over" : ""}"></span></div>
            <div class="row faint num tiny">
              <span>${m(b.spentCents)} de ${m(b.plannedCents)}</span>
              ${b.remainingCents < 0 ? html`<span class="neg">estourou ${m(-b.remainingCents)}</span>` : html`<span>resta ${m(b.remainingCents)}</span>`}
            </div>
          </div>`)}
        </div>
      </div>
    </section>

    <section class="dash-row dash-row3">
      <div class="panel">
        <div class="panel-head"><h2>Para onde o dinheiro foi</h2><a class="small" href="/analise">Análise</a></div>
        ${v.categories.length ? html`<ul class="hbars slim">
          ${v.categories.slice(0, 8).map((c) => html`<li>
            <span class="label">${c.name}</span>
            <span class="value">${m(c.spentCents)} <small class="faint">${c.sharePct}%</small></span>
            <span class="track"><span class="fill b-${c.bucket}" style="width: ${barPct(c.spentCents, maxCat)}%"></span></span>
          </li>`)}
        </ul>` : html`<p class="empty">Nenhuma saída neste mês. <a href="/lancamentos?mes=${month}">Lançar gastos</a></p>`}
      </div>

      <div class="panel recent">
        <div class="panel-head"><h2>Últimos lançamentos</h2><a class="small" href="/lancamentos">Ver todos</a></div>
        ${recent.length ? html`<ul class="tx-mini">
          ${recent.map((t) => {
            const cat = cats.get(t.categoryId);
            return html`<li><a href="/lancamentos?mes=${t.date.slice(0, 7)}&editar=${t.id}">
              ${iconTile(cat, t.kind)}
              <span class="tx-text"><span class="tx-desc">${t.description}</span><span class="faint tiny">${cat?.name || "Sem categoria"} · ${shortDate(t.date, now)}</span></span>
              <span class="tx-amt num ${t.kind === "entrada" ? "pos" : ""}">${t.kind === "entrada" ? "+" : "−"}${m(t.amountCents)}</span>
            </a></li>`;
          })}
        </ul>` : html`<p class="empty">Nada lançado ainda. <a href="/lancamentos">Lançar o primeiro</a></p>`}
      </div>

      <div class="dash-stack">
        <div class="panel">
          <h2>Próximos vencimentos</h2>
          ${due.length ? html`<ul class="due-list">
            ${due.map((d) => html`<li><a href="${d.href}">
              <span class="day-tile ${d.days <= 3 ? "soon" : ""}"><b>${d.at.getDate()}</b><small>${MONTHS[d.at.getMonth()].toUpperCase()}</small></span>
              <span class="tx-text"><span class="tx-desc">${d.name}</span><span class="tiny ${d.days <= 3 ? "neg" : "faint"}">${inDays(d.days)}</span></span>
              <span class="num small">${d.cents ? m(d.cents) : ""}</span>
            </a></li>`)}
          </ul>` : html`<p class="empty small">Nada vencendo. Cadastre o dia do vencimento das dívidas e das contas fixas.</p>`}
        </div>
        <a class="panel link-panel debt-sum" href="/dividas">
          <div class="panel-head"><h2>Dívidas</h2><span class="small accent">Ver plano →</span></div>
          ${dv.debts.length ? html`
            <div class="debt-figs"><strong class="num">${m(dv.totalCents)}</strong><span class="small neg">${m(dv.monthlyInterestCents)} de juros/mês</span></div>
            <span class="small muted">${dv.debts.length} ${dv.debts.length === 1 ? "dívida" : "dívidas"} · ${dv.plan.ok
              ? html`seguindo o plano, livre em <b>${finance.monthLabel(shiftMonth(day.slice(0, 7), dv.plan.months))}</b>`
              : "só no mínimo elas não acabam: veja o plano"}</span>`
            : html`<span class="small muted">Nenhuma dívida cadastrada. Ótimo sinal!</span>`}
        </a>
      </div>
    </section>

    ${briefs.length ? html`<section class="panel" id="time" aria-labelledby="time-h">
      <div class="panel-head"><h2 id="time-h">O que o time viu</h2><a class="small" href="/chat">Conversar</a></div>
      <div class="briefs">${briefs.map((b) => html`<a class="brief" href="${b.link}">
        <span class="brief-who"><img class="face" src="/static/img/avatar-${b.agent.animal}.png" alt="" width="36" height="36"><span><strong>${b.agent.name}</strong><small>${b.title}</small></span></span>
        <ul>${b.lines.map((l) => html`<li>${l}</li>`)}</ul></a>`)}</div>
    </section>` : ""}

    ${v.budgets.length ? html`<section class="panel" id="orcamentos">
      <div class="panel-head"><h2>Orçamentos do mês</h2><a class="small" href="/categorias">Ajustar tetos</a></div>
      ${budgetBars(v.budgets)}
    </section>` : ""}

    ${goalViews.length ? html`<section class="panel">
      <div class="panel-head"><h2>Metas</h2><a class="small" href="/metas">Ver todas</a></div>
      ${goalCards(goalViews.slice(0, 3), m, { compact: true })}
    </section>` : ""}

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
