import * as finance from "../finance.js";
import { askLink, html, iconTile, raw, shiftMonth, today } from "../ui.js";
import { chart } from "../charts.js";
import * as welcome from "./welcome.js";
import { stageDraft } from "./advisor.js";

export function monthNav(month) {
  return html`<div class="monthnav">
    <a href="?mes=${shiftMonth(month, -1)}" aria-label="Mês anterior">‹</a>
    <strong>${finance.monthLabel(month)}</strong>
    <a href="?mes=${shiftMonth(month, 1)}" aria-label="Próximo mês">›</a>
  </div>`;
}

// pageHead é o título da tela: a frase do design, com o "kicker" acima e a linha
// de apoio (sub) embaixo quando houver; a navegação de mês e os botões à direita.
export function pageHead(title, month, { kicker = "", actions = "", sub = "" } = {}) {
  return html`<div class="page-head">
    <div class="title">${kicker ? html`<span class="kicker">${kicker}</span>` : ""}<h1>${title}</h1>${sub ? html`<p class="sub">${sub}</p>` : ""}</div>
    ${month || actions ? html`<div class="actions">${month ? monthNav(month) : ""}${actions}</div>` : ""}
  </div>`;
}

export const PLUS = raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14"/></svg>');


const ARROW = "↗";
const shortDate = (date) => `${date.slice(8, 10)}/${date.slice(5, 7)}`;

export function metric(label, value, note, { primary = false, href = "", cls = "", extra = "" } = {}) {
  const body = html`<span class="metric-top">${label}${href ? html`<span aria-hidden="true">${ARROW}</span>` : ""}</span><strong class="${cls}">${value}</strong><small>${note}</small>`;
  return href ? html`<a class="card metric ${primary ? "primary" : ""} ${extra}" href="${href}">${body}</a>` : html`<div class="card metric ${primary ? "primary" : ""} ${extra}">${body}</div>`;
}

// spendingRing: a rosca das saídas do mês por categoria (Painel e Análise); v é o finance.dashboard do mês.
const DONUT = ["accent", "lilac", "desejos", "pink", "faint"];
export function spendingRing(v) {
  const m = finance.money;
  const top = v.categories.slice(0, 5);
  return top.length ? html`<div class="ringbox">
    ${chart({
      key: "categorias", type: "doughnut", height: 150, size: 150, cutout: "72%",
      labels: top.map((c) => c.name),
      series: [{ name: "Saídas", data: top.map((c) => c.spentCents), colors: top.map((_, i) => DONUT[i]) }],
      center: { value: m(v.outCents), text: "em saídas" },
    }, `Saídas do mês por categoria: ${top.map((c) => `${c.name} ${m(c.spentCents)}`).join(", ")}.`)}
    <ul class="catlegend">${top.map((c, i) => html`<li><i class="sq" style="--c: var(--${DONUT[i]})"></i><span>${c.name}</span><b class="num">${c.sharePct}%</b></li>`)}</ul>
  </div>` : html`<p class="empty">Nenhuma saída neste mês. <a href="/lancamentos?novo=1">Lançar um gasto</a></p>`;
}

export function render(ctx) {
  if (welcome.show(ctx)) return welcome.render(ctx);
  const { ledger, month } = ctx;
  const day = today();
  const current = month === day.slice(0, 7);
  const summary = finance.dashboard(ledger, month);
  const home = current ? finance.home(ledger, day) : null;
  const money = finance.money;
  const categories = new Map(ledger.categories.map((category) => [category.id, category]));
  const recent = ledger.transactions.filter((transaction) => !transaction.deleted && transaction.date.startsWith(month) && transaction.date <= day)
    .sort((first, second) => second.date.localeCompare(first.date) || second.updatedAt - first.updatedAt).slice(0, 5);
  const goals = current ? finance.goals(ledger, day).filter((goal) => !goal.done).slice(0, 1) : [];

  return html`
    ${pageHead("Seu dinheiro, sem complicar.", month, { kicker: "Início", sub: "O essencial primeiro. Os detalhes ficam a um toque." })}
    <section class="home-money" data-section="money-now" aria-labelledby="home-money-title">
      <h2 id="home-money-title">${current ? "Seu dinheiro agora" : finance.monthLabel(month)}</h2>
      <div class="cards has-hero dash-kpis metricgrid three">
        ${metric(home?.hasAccounts ? "Saldo das contas cadastradas" : "Saldo do mês", money(home?.hasAccounts ? home.accountsCents : summary.balanceCents),
          home?.hasAccounts ? `Saldo do mês: ${money(summary.balanceCents)}. Não é dinheiro livre.` : "Entradas menos saídas registradas. Não é saldo bancário.",
          { primary: true, href: home?.hasAccounts ? "/contas" : "/lancamentos" })}
        ${metric("Entradas do mês", money(summary.inCents), "O que entrou nos registros deste mês", { href: "/lancamentos?tipo=entrada" })}
        ${metric("Saídas do mês", money(summary.outCents), "O que saiu nos registros deste mês", { href: "/lancamentos?tipo=saida" })}
      </div>
      ${home ? html`<div class="panel home-estimate">
        <div><h3>Estimativa de orçamento</h3>
          ${home.estimateAvailable ? html`<strong class="home-estimate-value ${home.freeCents < 0 ? "neg" : ""}">${money(home.freeCents)}</strong>
            <p>${home.freeCents < 0 ? "O orçamento estimado já ficou negativo." : html`Referência por dia: <b>${money(home.perDayCents)}</b> até o fim do mês.`}</p>`
            : html`<p class="home-estimate-unavailable">Sem estimativa por enquanto</p><p>${home.estimateReason}</p>`}
          <p class="small muted">Estimativa com os registros disponíveis. Dados que faltam mudam o resultado.</p>
        </div>
        <details class="home-explanation"><summary>Como calculamos</summary>
          <p>O saldo do mês compara entradas e saídas. O saldo das contas considera apenas as contas cadastradas e seus movimentos.</p>
          <p>A estimativa de orçamento usa a renda registrada no mês, desconta a reserva planejada, os gastos e as recorrências previstas. Não é saldo disponível nem autorização para gastar.</p>
          <p>Sem renda registrada, com saldo das contas negativo, dívida ou cartão em aberto, a referência fica indisponível até você conferir os compromissos. Não somamos nem descontamos a mesma obrigação de novo para inventar um saldo livre.</p>
          <a href="/analise">Conferir a análise</a> · <a href="/contas">Conferir contas e cartões</a>
        </details>
      </div>` : html`<p class="tip info">Este é um mês do histórico. Estimativas e próximos vencimentos aparecem no mês atual.</p>`}
    </section>

    ${home ? html`<section class="panel home-week" data-section="week" aria-labelledby="home-week-title">
      <div class="panel-head"><div><h2 id="home-week-title">Nesta semana</h2><p class="small muted">Últimos 7 dias, de ${shortDate(home.weekFrom)} a ${shortDate(home.weekThrough)}: entraram ${money(home.weekInCents)} e saíram ${money(home.weekOutCents)}.</p></div></div>
      <h3>Próximos 7 dias</h3>
      <p class="small muted">Compromissos previstos até ${shortDate(home.dueThrough)}. Confira se as parcelas e recorrências já foram pagas.</p>
      ${home.due.length ? html`<ul class="deadlines">${home.due.map((due) => html`<li><a href="${due.link}"><span class="day">${shortDate(due.date)}</span><span class="dl-text"><b>${due.name}</b><small>Previsão cadastrada</small></span><b class="num">${money(due.cents)}</b></a></li>`)}</ul>`
        : html`<p class="small">Nenhum compromisso cadastrado para esses dias. Isso não significa que não existam contas a pagar.</p>`}
      <a href="/contas">Conferir contas e cartões ${ARROW}</a>
    </section>` : ""}

    ${home?.insights.length ? html`<section class="panel home-insights" id="time" data-section="insights" aria-labelledby="home-insights-title">
      <div class="panel-head"><div><h2 id="home-insights-title">O que merece sua atenção</h2><p class="small muted">Até três achados calculados no app, sem precisar perguntar à IA.</p></div><a href="/analise">Ver análise ${ARROW}</a></div>
      <div id="atencao" class="home-insight-list">${home.insights.map((insight) => html`<article class="brief home-insight" data-insight>
        <a class="brief-main" href="${insight.link}"><strong>${insight.agent.name}</strong><h3>${insight.title}</h3><ul>${insight.lines.map((line) => html`<li>${line}</li>`)}</ul><span class="small">Ver detalhes ${ARROW}</span></a>
        ${askLink(insight, insight.title)}
      </article>`)}</div>
    </section>` : ""}

    <section class="panel home-entry" aria-labelledby="home-entry-title">
      <div><h2 id="home-entry-title">Registrar sem complicar</h2><p class="small muted">Use o formulário ou prepare uma mensagem para o Consultor.</p></div>
      <div class="home-actions">
        <a class="btn" href="/lancamentos?novo=1">+ Registrar</a>
        <a class="btn secondary" href="/importar">Importar e revisar</a>
      </div>
      <form data-form="quick-entry" autocomplete="off" class="home-quick">
        <label for="home-message">Conte o que aconteceu</label>
        <div class="home-quick-row"><input id="home-message" name="message" maxlength="500" required placeholder="Ex.: gastei 32 no almoço" aria-describedby="home-entry-help"><button type="submit">Preparar no Consultor</button></div>
        <p id="home-entry-help" class="small muted">Você revisa a mensagem antes de enviar. Lançamentos só entram após sua confirmação. Sem IA? Use “+ Registrar”.</p>
      </form>
    </section>

    <section class="panel recent">
      <div class="panel-head"><h2>Últimos movimentos</h2><a href="/lancamentos">Ver todos ${ARROW}</a></div>
      ${recent.length ? html`<div class="table-wrap"><table class="tx-table"><thead><tr><th>Descrição / categoria</th><th>Data</th><th class="num">Valor</th></tr></thead><tbody>
        ${recent.map((transaction) => {
          const category = categories.get(transaction.categoryId);
          return html`<tr><td><a class="txnlabel" href="/lancamentos?${new URLSearchParams({ mes: transaction.date.slice(0, 7), editar: transaction.id })}">${iconTile(category, transaction.kind)}<span><b>${transaction.description}</b><small>${category?.name || "Sem categoria"}</small></span></a></td>
            <td class="nowrap">${shortDate(transaction.date)}</td><td class="num nowrap ${transaction.kind === "entrada" ? "pos" : ""}">${transaction.kind === "entrada" ? "+" : "−"} ${money(transaction.amountCents)}</td></tr>`;
        })}</tbody></table></div>` : html`<p class="small">Os movimentos aparecem aqui quando você registrar ou importar.</p>`}
    </section>

    ${goals.length ? html`<section class="panel home-goal"><div class="panel-head"><h2>Seu próximo objetivo</h2><a href="/metas">Ver metas ${ARROW}</a></div>
      <div class="goal-rows">${goals.map((goal) => html`<a class="goalrow" href="/metas"><span class="row"><b>✳ ${goal.name}</b><span class="small num">${goal.pct}%</span></span><span class="progress"><i style="width: ${Math.min(goal.pct, 100)}%"></i></span><span class="row small num"><span>${money(goal.progressCents)}</span><span>de ${money(goal.targetCents)}</span></span></a>`)}</div>
    </section>` : ""}
  `;
}

export function bind(root, ctx) {
  if (welcome.show(ctx)) {
    welcome.bind(root, ctx);
    return;
  }
  root.querySelector('[data-form="quick-entry"]')?.addEventListener("submit", (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    if (!stageDraft(form.elements.message.value)) {
      ctx.error(new Error("Você já tem uma mensagem ou resposta em andamento no Consultor. Termine ou limpe a mensagem antes de preparar outra."));
      return;
    }
    form.reset();
    ctx.go("/chat");
  });
}
