// Painel do mês (design Rich, docs/rich-mais-recente): a situação do mês, o ajuste
// que faz diferença, os quatro números, o ano, para onde foi o dinheiro, os
// últimos movimentos, as metas, o que pede atenção, os saldos e o que o time viu.
// Os números vêm do Go (finance.*); aqui só desenha.
import * as finance from "../finance.js";
import { askLink, html, iconTile, raw, shiftMonth, today } from "../ui.js";
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

const ARROW = "↗";
// greeting: a saudação do topo, pela hora do aparelho.
const greeting = (now) => (now.getHours() < 5 ? "Boa noite" : now.getHours() < 12 ? "Bom dia" : now.getHours() < 18 ? "Boa tarde" : "Boa noite");
// O que cada um do time diz quando não tem achado no mês (o card continua, com o caminho para conversar).
const QUIET = {
  otavio: "Nenhuma dívida pedindo plano agora. Quer revisar o mês comigo?",
  tito: "Sem metas em andamento. Um dia é uma ideia; com um plano, vira uma data.",
  lia: "Nada fora do padrão nos gastos deste mês. Seu cafezinho segue liberado.",
};

function metric(label, value, note, { primary = false, href = "" } = {}) {
  const body = html`<span class="metric-top">${label}<span aria-hidden="true">${ARROW}</span></span><strong>${value}</strong><small>${note}</small>`;
  return href ? html`<a class="card metric ${primary ? "primary" : ""}" href="${href}">${body}</a>` : html`<div class="card metric ${primary ? "primary" : ""}">${body}</div>`;
}

export function render(ctx) {
  if (welcome.show(ctx)) return welcome.render(ctx);
  const { ledger, month } = ctx;
  const v = finance.dashboard(ledger, month);
  const m = finance.money;
  const prevMonth = shiftMonth(month, -1);
  const prev = finance.dashboard(ledger, prevMonth);
  const now = new Date();
  const day = today();
  const isNow = month === day.slice(0, 7);
  const rise = painted ? "" : "rise";
  painted = true;

  const briefs = isNow ? finance.briefings(ledger, day) : [];
  const step = isNow ? finance.alerts(ledger, day)[0] : null;
  const goalViews = finance.goals(ledger, day).filter((g) => !g.done || g.deadline >= month);
  const inMonth = ledger.transactions.filter((t) => !t.deleted && t.date.startsWith(month));
  const has = inMonth.length > 0;
  const outPct = v.inCents ? Math.round((v.outCents * 100) / v.inCents) : 0;
  const prevName = finance.monthLabel(prevMonth).split(" de ")[0];
  const yearIn = v.year.reduce((s, t) => s + t.inCents, 0);
  const yearOut = v.year.reduce((s, t) => s + t.outCents, 0);
  const yearMonths = v.year.filter((t) => t.inCents || t.outCents).length;
  const invested = v.buckets.find((b) => b.bucket === "investimentos")?.spentCents || 0;
  const watch = v.budgets.filter((b) => b.usedPct >= 80).sort((a, b) => b.usedPct - a.usedPct);
  const over = watch.filter((b) => b.remainingCents < 0);
  const cats = new Map(ledger.categories.map((c) => [c.id, c]));
  const accountName = new Map(ledger.accounts.filter((a) => !a.deleted).map((a) => [a.id, a.name]));
  const recent = ledger.transactions.filter((t) => !t.deleted && t.date <= day)
    .sort((a, b) => b.date.localeCompare(a.date) || b.updatedAt - a.updatedAt).slice(0, 5);
  const due = upcoming(ledger, now);
  const dv = finance.debts(ledger, 0);
  const acc = finance.accounts(ledger, day);
  const fixed = ledger.recurring.filter((r) => !r.deleted && !r.paused && r.kind === "saida" && (!r.end || r.end >= month))
    .reduce((s, r) => s + r.amountCents, 0);
  const worth = acc.accountsCents - acc.cardsOpenCents - dv.totalCents;
  const yearHas = (i) => v.year[i].inCents > 0 || v.year[i].outCents > 0;
  const current = Math.max(0, v.year.findIndex((t) => t.month === month));
  const stripFor = (i) => {
    const t = v.year[i];
    return html`<strong>${finance.monthLabel(t.month)}</strong>
      <span>Entradas <b>${m(t.inCents)}</b></span>
      <span>Saídas <b>${m(t.outCents)}</b></span>
      <span>Saldo <b class="${t.balanceCents < 0 ? "neg" : "pos"}">${t.balanceCents > 0 ? "+" : ""}${m(t.balanceCents)}</b></span>`;
  };
  const top = v.categories.slice(0, 5);
  const DONUT = ["accent", "lilac", "desejos", "pink", "faint"];
  const briefOf = new Map(briefs.map((b) => [b.agent.id, b]));

  return html`
    <div class="page-head dash-head">
      <div class="title"><h1>${greeting(now)} <span class="star" aria-hidden="true">✳</span></h1>
        <p class="sub">Um olhar para ${finance.monthLabel(month)}. Seu próximo passo começa aqui.</p></div>
      <div class="actions">${monthNav(month)}</div>
    </div>

    <section class="hero-panel ${rise}">
      <div class="hero-copy">
        <span class="eyebrow">Sua situação, sem complicar</span>
        <h2>${!has ? "Começar também é um plano." : v.balanceCents >= 0 ? "Tem espaço para os seus próximos planos." : "Um mês apertado. Vamos organizar o próximo passo?"}</h2>
        <p>${!has ? "Lance uma entrada e as despesas. Os números aparecem conforme você avança."
          : html`Entraram ${m(v.inCents)} e saíram ${m(v.outCents)}. ${over.length
            ? `Vale revisar ${over.length} ${over.length === 1 ? "categoria acima do orçamento" : "categorias acima do orçamento"}.`
            : "Que tal dar um destino para o que sobrou?"}`}</p>
        <a class="btn" href="${has ? "/analise" : "/lancamentos?novo=1"}">${has ? "Ver meu próximo passo" : "Fazer o primeiro lançamento"} ${ARROW}</a>
      </div>
      <div class="hero-art" aria-hidden="true"><img src="/static/img/time/lia.webp" alt="" width="180" height="210"><span>um passo<br><em>de cada vez.</em> ✳</span></div>
    </section>

    ${step ? html`<section class="next-step">
      <img class="portrait" src="/static/img/time/${step.agent.id}-mini.webp" alt="" width="52" height="52">
      <div class="next-copy"><span class="eyebrow">Um ajuste que faz diferença</span><h3>${step.title}</h3><p>${step.text}</p></div>
      <div class="next-actions"><a class="btn secondary" href="${step.link}">Ver o que fazer ${ARROW}</a>${askLink(step, step.title)}</div>
    </section>` : ""}

    <section class="cards has-hero dash-kpis metricgrid">
      ${metric("Entradas do mês", m(v.inCents), prev.inCents ? `${v.inCents >= prev.inCents ? "+" : "−"}${m(Math.abs(v.inCents - prev.inCents))} em relação a ${prevName}` : "Sem comparação com o mês anterior", { href: "/lancamentos?tipo=entrada" })}
      ${metric("Saídas do mês", m(v.outCents), v.inCents ? `${outPct}% da renda já saiu` : "Sem renda lançada para calcular a %", { href: "/lancamentos?tipo=saida" })}
      ${metric("Saldo do mês", m(v.balanceCents), "Entradas menos todas as saídas", { primary: true, href: "/contas" })}
      ${metric("Taxa de economia", v.inCents ? `${v.savingsPct}%` : "—", `${m(invested)} destinados a investimentos`, { href: "/metas" })}
    </section>

    <section class="grid2">
      <div class="panel flow">
        <div class="panel-head"><div><h2>O ano, em perspectiva</h2><span class="faint small">Entradas e saídas · ${month.slice(0, 4)}</span></div>
          <p class="legend"><span><i class="sq" style="--c: var(--accent)"></i>Entradas</span><span><i class="sq" style="--c: var(--desejos)"></i>Saídas</span></p></div>
        ${chart({
          key: "fluxo", type: "bar", height: 220, legend: false, dashed: true, bar: { max: 11, radius: 4 },
          labels: v.year.map((t) => MONTHS[Number(t.month.slice(5)) - 1]),
          series: [
            { name: "Entradas", data: v.year.map((t, i) => (yearHas(i) ? t.inCents : null)), color: "accent" },
            { name: "Saídas", data: v.year.map((t, i) => (yearHas(i) ? t.outCents : null)), color: "desejos" },
          ],
          strip: { index: current, render: stripFor, valid: yearHas },
        }, `Entradas e saídas por mês em ${month.slice(0, 4)}: ${m(yearIn)} de entradas e ${m(yearOut)} de saídas.`)}
        <details class="see-table"><summary>Ver valores em tabela</summary><div class="table-wrap"><table>
          <thead><tr><th>Mês</th><th class="num">Entradas</th><th class="num">Saídas</th><th class="num">Saldo</th></tr></thead>
          <tbody>${v.year.filter((t) => t.inCents || t.outCents).map((t) => html`
            <tr><td><a href="/?mes=${t.month}">${finance.monthLabel(t.month)}</a></td><td class="num">${m(t.inCents)}</td><td class="num">${m(t.outCents)}</td>
              <td class="num ${t.balanceCents < 0 ? "neg" : "pos"}">${m(t.balanceCents)}</td></tr>`)}</tbody>
        </table></div></details>
      </div>

      <div class="panel">
        <div class="panel-head"><div><h2>Para onde a grana foi</h2><span class="faint small">${finance.monthLabel(month)}</span></div><a class="small" href="/analise">Ver análise ${ARROW}</a></div>
        ${top.length ? html`<div class="ringbox">
          ${chart({
            key: "categorias", type: "doughnut", height: 150, size: 150, cutout: "72%",
            labels: top.map((c) => c.name),
            series: [{ name: "Saídas", data: top.map((c) => c.spentCents), colors: top.map((_, i) => DONUT[i]) }],
            center: { value: m(v.outCents), text: "em saídas" },
          }, `Saídas do mês por categoria: ${top.map((c) => `${c.name} ${m(c.spentCents)}`).join(", ")}.`)}
          <ul class="catlegend">${top.map((c, i) => html`<li><i class="sq" style="--c: var(--${DONUT[i]})"></i><span>${c.name}</span><b class="num">${c.sharePct}%</b></li>`)}</ul>
        </div>` : html`<p class="empty">Nenhuma saída neste mês. <a href="/lancamentos?novo=1">Lançar um gasto</a></p>`}
      </div>
    </section>

    <section class="grid2">
      <div class="panel recent">
        <div class="panel-head"><div><h2>Últimos movimentos</h2><span class="faint small">Pequenos registros. Mais clareza.</span></div><a class="small" href="/lancamentos">Ver todos ${ARROW}</a></div>
        ${recent.length ? html`<div class="table-wrap"><table class="tx-table">
          <thead><tr><th>Descrição / categoria</th><th>Data</th><th class="hide-sm">Conta</th><th class="num">Valor</th></tr></thead>
          <tbody>${recent.map((t) => {
            const cat = cats.get(t.categoryId);
            return html`<tr><td><a class="txnlabel" href="/lancamentos?mes=${t.date.slice(0, 7)}&editar=${t.id}">${iconTile(cat, t.kind)}<span><b>${t.description}</b><small>${cat?.name || "Sem categoria"}</small></span></a></td>
              <td class="nowrap">${shortDate(t.date, now)}</td><td class="hide-sm faint">${accountName.get(t.accountId) || "—"}</td>
              <td class="num nowrap ${t.kind === "entrada" ? "pos" : ""}">${t.kind === "entrada" ? "+" : "−"} ${m(t.amountCents)}</td></tr>`;
          })}</tbody></table></div>` : html`<p class="empty">Nada lançado ainda. <a href="/lancamentos?novo=1">Lançar o primeiro</a></p>`}
      </div>

      <div class="panel">
        <div class="panel-head"><div><h2>Seu “um dia”, mais perto</h2><span class="faint small">Planos com nome, valor e um caminho.</span></div><a class="small" href="/metas">Ver metas ${ARROW}</a></div>
        ${goalViews.length ? html`<div class="goal-rows">${goalViews.slice(0, 3).map((g) => html`<a class="goalrow" href="/metas">
          <span class="row"><b>✳ ${g.name}</b><span class="faint small num">${g.pct}%</span></span>
          <span class="progress"><i style="width: ${Math.min(g.pct, 100)}%"></i></span>
          <span class="row faint tiny num"><span>${m(g.progressCents)}</span><span>de ${m(g.targetCents)}</span></span>
        </a>`)}</div>` : html`<p class="empty">Nenhuma meta ainda. <a href="/metas">Criar a primeira</a>: um dia pode virar uma data.</p>`}
      </div>
    </section>

    <section class="grid2 equal">
      <div class="panel" id="atencao">
        <div class="panel-head"><div><h2>O que pede atenção</h2><span class="faint small">${dv.debts.length} ${dv.debts.length === 1 ? "dívida" : "dívidas"} · ${over.length} ${over.length === 1 ? "orçamento acima do teto" : "orçamentos acima do teto"}</span></div></div>
        ${due.length ? html`<ul class="deadlines">${due.map((d) => html`<li><a href="${d.href}">
          <span class="day ${d.days <= 3 ? "soon" : ""}">${d.at.getDate()}<small>${MONTHS[d.at.getMonth()].toUpperCase()}</small></span>
          <span class="dl-text"><b>${d.name}</b><small class="${d.days <= 3 ? "neg" : "faint"}">${inDays(d.days)}</small></span>
          <b class="num">${d.cents ? m(d.cents) : ""}</b></a></li>`)}</ul>` : html`<p class="empty small">Nada vencendo. Cadastre o dia do vencimento das dívidas e das contas fixas.</p>`}
        ${dv.debts.length ? html`<p class="tip">Saldo devedor: <b class="num">${m(dv.totalCents)}</b> · ${m(dv.monthlyInterestCents)} de juros por mês. <a href="/dividas">Revisar plano ${ARROW}</a></p>` : ""}
        ${watch.length ? html`<ul class="budget-watch">${watch.map((b) => html`<li class="${b.remainingCents < 0 ? "neg" : "warn-text"}">${b.remainingCents < 0
          ? `${b.name}: ${m(-b.remainingCents)} acima do teto.`
          : `${b.name}: ${m(b.spentCents)} de ${m(b.budgetCents)} (${b.usedPct}%).`}</li>`)}</ul>` : ""}
      </div>

      <div class="panel">
        <div class="panel-head"><div><h2>Cada saldo tem seu papel</h2><span class="faint small">Posição até hoje</span></div><a class="small" href="/contas">Contas ${ARROW}</a></div>
        <dl class="balances">
          <div><dt>Saldo das contas</dt><dd class="num">${m(acc.accountsCents)}</dd></div>
          <div><dt>Compromissos recorrentes mensais</dt><dd class="num">${m(fixed)}</dd></div>
          <div><dt>Cartões em aberto</dt><dd class="num">${m(acc.cardsOpenCents)}</dd></div>
          <div><dt>Patrimônio líquido cadastrado</dt><dd class="num ${worth < 0 ? "neg" : ""}">${m(worth)}</dd></div>
        </dl>
        <p class="faint tiny">Patrimônio = contas − cartões − dívidas. Compromissos são referências mensais, não um saldo a debitar de novo.</p>
      </div>
    </section>

    <section class="panel" id="time" aria-labelledby="time-h">
      <div class="panel-head"><div><h2 id="time-h">O que o time viu</h2><span class="faint small">Achados calculados aqui. Uma conversa pode ser o próximo passo.</span></div><a class="small" href="/chat">Conhecer o time ${ARROW}</a></div>
      <div class="teamgrid briefs">${finance.team().map((a) => {
        const b = briefOf.get(a.id);
        return html`<article class="teamcard brief">
          <a class="brief-main" href="${b?.link || "/chat?agente=" + a.id}">
            <span class="team-portrait"><img src="/static/img/time/${a.id}.webp" alt="" width="640" height="640" loading="lazy"><span>SEU GUIA VIRTUAL ✳</span></span>
            <span class="teamcard-body"><strong>${a.name}</strong><small>${b?.title || a.role}</small>
              ${b ? html`<ul>${b.lines.map((l) => html`<li>${l}</li>`)}</ul>` : html`<p>${QUIET[a.id] || ""}</p>`}</span>
          </a>
          ${b ? askLink(b, b.title) : html`<a class="ask-team" href="/chat?agente=${a.id}">Conversar com ${a.name}</a>`}
        </article>`;
      })}</div>
    </section>

    ${yearMonths ? html`<p class="year-strip"><b>Resumo de ${month.slice(0, 4)}:</b> ${yearMonths} ${yearMonths === 1 ? "mês" : "meses"} com registros · ${m(yearIn)} em entradas · ${m(yearOut)} em saídas · ${m(yearIn - yearOut)} de saldo acumulado do ano.</p>` : ""}`;
}

export function bind(root, ctx) {
  if (welcome.show(ctx)) welcome.bind(root, ctx);
}
