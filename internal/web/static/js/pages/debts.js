// Dívidas: o plano para sair delas (o que pagar primeiro, em quanto tempo e
// quanto de juros se economiza), o gráfico e o cadastro. As contas são de
// finance.Debts (Go): aqui só desenha.
import * as finance from "../finance.js";
import { addPanel, centsInput, html, raw, ratePct, shiftMonth, today } from "../ui.js";
import { pageHead } from "./dashboard.js";

const W = 600, H = 220, PAD = { l: 8, r: 8, t: 12, b: 24 };

// horizon é até onde o gráfico vai: o fim do plano, ou do "só o mínimo" se ele
// acabar; se a dívida só cresce, um pouco além do plano (senão a linha do plano some).
function horizon(v) {
  const plan = v.plan.balanceCents.length - 1, min = v.minOnly.balanceCents.length - 1;
  return v.minOnly.ok ? Math.max(plan, min) : Math.min(min, Math.max(plan * 2, 24));
}

const whenLabel = (months) => finance.monthLabel(shiftMonth(today().slice(0, 7), months));

// duration: 8 → "8 meses"; 18 → "1 ano e 6 meses"; 121 → "10 anos e 1 mês".
export function duration(months) {
  const y = Math.floor(months / 12), m = months % 12;
  const part = (n, one, many) => `${n} ${n === 1 ? one : many}`;
  if (!y) return part(m, "mês", "meses");
  return m ? `${part(y, "ano", "anos")} e ${part(m, "mês", "meses")}` : part(y, "ano", "anos");
}

// calendarTable: mês a mês, quanto pagar em cada dívida seguindo o plano, o
// saldo e o saldo pagando só o mínimo; uma linha por ano separa os meses.
function calendarTable(v) {
  const m = finance.money;
  const done = new Map(v.plan.paidOff.map((p) => [`${p.month}|${p.name}`, true]));
  let year = "";
  const rows = v.plan.calendar.map((month, i) => {
    const key = shiftMonth(today().slice(0, 7), i + 1);
    const head = key.slice(0, 4) !== year ? ((year = key.slice(0, 4)), html`<tr class="year-row"><th colspan="${v.plan.names.length + 4}">${year}</th></tr>`) : "";
    const total = month.payments.reduce((a, b) => a + b, 0);
    return html`${head}<tr><td>${finance.monthLabel(key).replace(/ de \d{4}$/, "")}</td>
      ${month.payments.map((c, j) => html`<td class="num">${c ? m(c) : "—"}${done.has(`${i + 1}|${v.plan.names[j]}`) ? html` <span class="tag ok" title="quitada neste mês">quitada</span>` : ""}</td>`)}
      <td class="num"><strong>${m(total)}</strong></td><td class="num">${m(v.plan.balanceCents[i + 1])}</td>
      <td class="num muted">${i + 1 < v.minOnly.balanceCents.length ? m(v.minOnly.balanceCents[i + 1]) : "—"}</td></tr>`;
  });
  return html`<details class="calendar"><summary>Calendário mês a mês (${duration(v.plan.months)})</summary>
    <div class="table-wrap"><table>
      <thead><tr><th>Mês</th>${v.plan.names.map((n) => html`<th class="num">${n}</th>`)}<th class="num">Pagar no mês</th><th class="num">Saldo</th><th class="num">Só o mínimo</th></tr></thead>
      <tbody>${rows}</tbody>
    </table></div>
  </details>`;
}

// debtChart: saldo total mês a mês, seguindo o plano (linha cheia) e pagando só
// o mínimo (tracejada). Mesmo desenho do gráfico da Análise.
function debtChart(v) {
  const m = finance.money;
  const n = Math.max(1, horizon(v));
  const plan = v.plan.balanceCents.slice(0, n + 1), min = v.minOnly.balanceCents.slice(0, n + 1);
  const top = Math.max(1, ...plan, ...min);
  const x = (i) => PAD.l + (i / n) * (W - PAD.l - PAD.r);
  const y = (c) => H - PAD.b - (c / top) * (H - PAD.t - PAD.b);
  const path = (s) => s.map((c, i) => `${i ? "L" : "M"}${x(i).toFixed(1)},${y(c).toFixed(1)}`).join("");
  const tick = (i, anchor, label) => `<text x="${x(i)}" y="${H - 6}" text-anchor="${anchor}">${label}</text>`;
  const svg = `<svg viewBox="0 0 ${W} ${H}" role="img" aria-label="Saldo das dívidas mês a mês: seguindo o plano ${v.plan.ok ? `zera em ${v.plan.months} meses` : "não zera"}; só no mínimo ${v.minOnly.ok ? `zera em ${v.minOnly.months} meses` : "não zera"}.">
    <line class="axis" x1="${PAD.l}" x2="${W - PAD.r}" y1="${H - PAD.b}" y2="${H - PAD.b}"/>
    <g class="ticks">${tick(0, "start", "hoje")}${tick(n, "end", `${n} meses`)}</g>
    <path class="line last" d="${path(min)}"/>
    <path class="line now" d="${path(plan)}"/>
    <line class="cross" y1="${PAD.t}" y2="${H - PAD.b}" x1="-10" x2="-10"/>
  </svg>`;
  return html`
    <div class="pace-chart debt-chart" data-months="${n}">
      <p class="legend">
        <span><span class="key now"></span>Seguindo o plano</span>
        <span><span class="key last"></span>Só o mínimo</span>
      </p>
      ${raw(svg)}
      <p class="chart-tip small" aria-live="polite" hidden></p>
    </div>
`;
}

// planPanel diz em poucas linhas o que fazer, com o valor que dá para pagar a mais.
function planPanel(v) {
  const m = finance.money;
  const monthly = v.totalMinCents + v.extraCents;
  const first = v.plan.paidOff[0]?.name || v.avalanche[0];
  const saved = v.minOnly.interestCents - v.plan.interestCents;
  return html`
    <section class="panel debt-plan">
      <h2>Seu plano</h2>
      ${v.plan.ok
        ? html`<p class="plan-lead">${v.debts.length > 1 ? html`Pague primeiro <strong>${first}</strong>. ` : ""}Pagando <strong>${m(monthly)}</strong> por mês, você ${v.debts.length > 1 ? "sai das dívidas" : html`quita <strong>${first}</strong>`} em
            <strong>${duration(v.plan.months)}</strong> (${whenLabel(v.plan.months)}) e paga ${m(v.plan.interestCents)} de juros.</p>
          ${v.plan.paidOff.length > 1 ? html`<p class="small muted">${v.plan.paidOff.length} dívidas: a primeira acaba em ${whenLabel(v.plan.paidOff[0].month)}, a última em ${whenLabel(v.plan.months)}.</p>` : ""}
          ${v.minOnly.ok
            ? saved > 0 ? html`<p class="notice ok">Só no mínimo seriam ${duration(v.minOnly.months)} e ${m(v.minOnly.interestCents)} de juros: você economiza <strong>${m(saved)}</strong>${v.minOnly.months > v.plan.months ? ` e sai ${duration(v.minOnly.months - v.plan.months)} antes` : ""}.</p>` : ""
            : html`<p class="notice warn">Só no mínimo as dívidas não acabam: os juros passam do que você paga.</p>`}`
        : html`<p class="notice warn">Com ${m(monthly)} por mês as dívidas não acabam: os juros passam do que você paga. Aumente o valor abaixo, ou converse com o banco sobre trocar por uma dívida mais barata.</p>`}
      <form data-form="extra" class="extra-form">
        <label><span>Quanto dá para pagar <strong>a mais</strong> por mês (R$)</span>
          <input name="extra" inputmode="decimal" value="${v.extraCents ? centsInput(v.extraCents) : ""}" placeholder="0,00"></label>
        <button type="submit" class="secondary">Recalcular</button>
      </form>
      ${debtChart(v)}
      ${v.plan.ok ? calendarTable(v) : ""}
      ${v.plan.paidOff.length > 1 ? html`<h3>Ordem para quitar</h3>
        <ol class="paid-off">${v.plan.paidOff.map((p) => html`<li><strong>${p.name}</strong> <span class="muted">zera em ${whenLabel(p.month)}</span></li>`)}</ol>` : ""}
      <details><summary>Por que essa ordem?</summary>
        <p class="small muted">Você paga o mínimo de todas e todo o resto vai para a de <strong>maior juros</strong> (método avalanche): é o que mais economiza. Quando uma acaba, o que você pagava nela vai para a próxima.</p>
      </details>
    </section>`;
}

export function render({ ledger, params }) {
  const extra = Math.max(0, Number(params.get("extra")) || 0);
  const v = finance.debts(ledger, extra);
  const m = finance.money;
  const cards = ledger.accounts.some((a) => a.kind === "cartao" && !a.deleted) ? finance.accounts(ledger, today()).cardsOpenCents : 0;
  const editing = ledger.debts.find((d) => d.id === params.get("editar") && !d.deleted);
  const e = editing || {};

  return html`
    ${pageHead("Dívidas")}
    <section class="cards">
      <div class="card"><span class="muted">Total devido</span><strong class="neg">${m(v.totalCents)}</strong></div>
      <div class="card"><span class="muted">Parcelas mínimas / mês</span><strong>${m(v.totalMinCents)}</strong></div>
      <div class="card"><span class="muted">Juros gerados / mês</span><strong class="neg">${m(v.monthlyInterestCents)}</strong></div>
    </section>

    ${v.debts.length ? planPanel(v) : ""}
    ${cards > 0 ? html`<p class="notice">Faturas abertas nos cartões: <strong>${m(cards)}</strong> (veja em <a href="/contas">Contas</a>). Pague a fatura inteira para não cair no rotativo, o juro mais caro do país.</p>` : ""}

    ${addPanel("debt", editing ? "Editar dívida" : "+ Nova dívida", html`
      <form data-form="debt" class="grid-form">
        <label>Nome <input name="name" required value="${e.name || ""}" placeholder="Ex.: Cartão Nubank"></label>
        <label>Credor <input name="creditor" value="${e.creditor || ""}" placeholder="Banco/loja"></label>
        <label class="half">Saldo devedor (R$) <input name="balance" required inputmode="decimal" value="${editing ? centsInput(e.balanceCents) : ""}"></label>
        <label class="half">Juros ao mês (%) <input name="rate" inputmode="decimal" value="${editing ? centsInput(e.monthlyRateBp) : ""}" placeholder="Ex.: 12,5"></label>
        <label class="half">Parcela mínima (R$) <input name="min_payment" inputmode="decimal" value="${editing ? centsInput(e.minPaymentCents) : ""}"></label>
        <label class="half">Dia do vencimento <input type="number" name="due_day" min="1" max="31" value="${e.dueDay || ""}"></label>
        <div class="form-actions">
          <button type="submit">Salvar</button>
          ${editing ? html`<a href="/dividas">Cancelar</a>` : ""}
        </div>
      </form>`, !!editing || !v.debts.length)}

    <section class="panel">
      <h2>Suas dívidas</h2>
      ${v.debts.length ? html`
        <div class="table-wrap"><table class="cards-mobile">
          <thead><tr><th>Dívida</th><th class="num">Saldo</th><th class="num">Juros a.m.</th><th class="num">Mínimo</th><th>Só pagando o mínimo</th><th></th></tr></thead>
          <tbody>
            ${v.debts.map((d) => html`<tr>
              <td class="primary"><strong>${d.name}</strong>${d.creditor ? html`<br><small class="muted">${d.creditor}</small>` : ""}${d.dueDay ? html`<br><small class="muted">vence dia ${d.dueDay}</small>` : ""}</td>
              <td data-label="Saldo" class="num">${m(d.balanceCents)}</td>
              <td data-label="Juros a.m." class="num">${ratePct(d.monthlyRateBp)}</td>
              <td data-label="Mínimo" class="num">${m(d.minPaymentCents)}</td>
              <td data-label="Só o mínimo">${d.payable
                ? `${duration(d.months)} · ${m(d.interestCents)} de juros`
                : d.minPaymentCents ? html`<span class="neg">não cobre os juros — a dívida cresce</span>` : html`<span class="muted">informe o pagamento</span>`}</td>
              <td class="num nowrap actions">
                <a href="/dividas?editar=${d.id}">editar</a>
                <button class="link" type="button" data-delete="${d.id}">apagar</button>
              </td>
            </tr>`)}
          </tbody>
        </table></div>`
        : html`<p class="empty">Nenhuma dívida cadastrada. Ótimo sinal!</p>`}
    </section>

    ${v.debts.length ? html`<p><a class="button secondary-link" href="/chat">Tirar dúvidas com o consultor IA</a></p>` : ""}`;
}

export function bind(root, ctx) {
  root.querySelector('[data-form="extra"]')?.addEventListener("submit", (e) => {
    e.preventDefault();
    try {
      const raw = e.target.elements.extra.value.trim();
      const cents = raw ? finance.parseMoney(raw) : 0;
      ctx.go(cents > 0 ? `/dividas?extra=${cents}` : "/dividas");
    } catch (err) {
      ctx.error(err);
    }
  });
  bindChart(root, ctx);
  const form = root.querySelector('[data-form="debt"]');
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = form.elements;
    const parse = (input, label) => {
      if (!input.value.trim()) return 0;
      try { return finance.parseMoney(input.value); } catch (err) { throw new Error(`${label}: ${err.message}`); }
    };
    try {
      const editing = ctx.ledger.debts.find((d) => d.id === ctx.params.get("editar"));
      await ctx.save("debt", {
        ...(editing ? { id: editing.id } : {}),
        name: f.name.value.trim(), creditor: f.creditor.value.trim(),
        balanceCents: parse(f.balance, "saldo devedor"),
        monthlyRateBp: parse(f.rate, "juros"), // "12,5" → 1250 pontos-base, mesmo parser do dinheiro
        minPaymentCents: parse(f.min_payment, "parcela"),
        dueDay: Number(f.due_day.value) || 0,
      }, "Dívida salva.");
      if (editing) ctx.go("/dividas");
    } catch (err) {
      ctx.error(err);
    }
  });

  root.querySelectorAll("[data-delete]").forEach((btn) => btn.addEventListener("click", async () => {
    const d = ctx.ledger.debts.find((x) => x.id === btn.dataset.delete);
    if (!confirm(`Apagar ${d.name}?`)) return;
    await ctx.remove("debt", d, "Dívida apagada.");
  }));
}

// bindChart: passar o dedo ou o mouse no gráfico mostra o saldo daquele mês.
function bindChart(root, ctx) {
  const box = root.querySelector(".debt-chart");
  if (!box) return;
  const v = finance.debts(ctx.ledger, Math.max(0, Number(ctx.params.get("extra")) || 0));
  const svg = box.querySelector("svg"), tip = box.querySelector(".chart-tip"), cross = box.querySelector(".cross");
  const n = Number(box.dataset.months);
  const at = (s, i) => (i < s.length ? finance.money(s[i]) : "—");
  const show = (e) => {
    const r = svg.getBoundingClientRect();
    const i = Math.min(n, Math.max(0, Math.round(((((e.clientX - r.left) / r.width) * W - PAD.l) / (W - PAD.l - PAD.r)) * n)));
    const cx = PAD.l + (i / n) * (W - PAD.l - PAD.r);
    cross.setAttribute("x1", cx);
    cross.setAttribute("x2", cx);
    tip.textContent = `${i ? whenLabel(i) : "Hoje"}: plano ${at(v.plan.balanceCents, i)} · só o mínimo ${at(v.minOnly.balanceCents, i)}`;
    tip.hidden = false;
  };
  svg.addEventListener("pointermove", show);
  svg.addEventListener("pointerdown", show);
  svg.addEventListener("pointerleave", () => { tip.hidden = true; cross.setAttribute("x1", -10); cross.setAttribute("x2", -10); });
}
