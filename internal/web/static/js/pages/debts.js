// Dívidas: o plano para sair delas (o que pagar primeiro, em quanto tempo e
// quanto de juros se economiza), o gráfico e o cadastro. As contas são de
// finance.Debts (Go): aqui só desenha.
import * as finance from "../finance.js";
import { addPanel, centsInput, html, ratePct, shiftMonth, today } from "../ui.js";
import { pageHead } from "./dashboard.js";
import { chart } from "../charts.js";

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

// debtChart: saldo total mês a mês, seguindo o plano (área cheia) e pagando só
// o mínimo (tracejada). Passar o dedo mostra o saldo daquele mês.
function debtChart(v) {
  const n = Math.max(1, horizon(v));
  const label = (i) => (i ? whenLabel(i) : "Hoje");
  const upTo = (s) => Array.from({ length: n + 1 }, (_, i) => (i < s.length ? s[i] : null));
  return html`<div class="debt-chart">${chart({
    key: "dividas", type: "line", height: 260,
    labels: Array.from({ length: n + 1 }, (_, i) => (i ? `${label(i).slice(0, 3)}/${label(i).slice(-2)}` : "hoje")),
    tip: label,
    series: [
      { name: "Seguindo o plano", data: upTo(v.plan.balanceCents), color: "accent", fill: true },
      { name: "Só o mínimo", data: upTo(v.minOnly.balanceCents), color: "neg", dash: true },
    ],
  }, `Saldo das dívidas mês a mês: seguindo o plano ${v.plan.ok ? `zera em ${v.plan.months} meses` : "não zera"}; só no mínimo ${v.minOnly.ok ? `zera em ${v.minOnly.months} meses` : "não zera"}.`)}</div>`;
}

// planPanel diz em poucas linhas o que fazer, com o valor que dá para pagar a mais.
function planPanel(v, c) {
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
          <input name="extra" inputmode="decimal" value="${c.typed ? centsInput(c.typed) : ""}" placeholder="0,00"></label>
        ${c.cuts ? html`<p class="small muted">Mais ${m(c.cuts)} dos cortes marcados acima.</p>` : ""}
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

// O que vai a mais por mês: o valor digitado mais os cortes marcados no diagnóstico.
function chosen(ledger, params) {
  const typed = Math.max(0, Number(params.get("extra")) || 0);
  const ids = new Set((params.get("cortes") || "").split(",").filter(Boolean));
  const diag = finance.diagnose(ledger, today());
  const cuts = diag.levers.filter((l) => ids.has(l.id)).reduce((sum, l) => sum + l.monthlyCents, 0);
  return { typed, ids, diag, cuts, extra: typed + cuts };
}

// planURL monta o endereço com o valor a mais e os cortes, para recarregar o plano.
const planURL = (typed, ids) => {
  const q = new URLSearchParams();
  if (typed > 0) q.set("extra", typed);
  if (ids.size) q.set("cortes", [...ids].join(","));
  return `/dividas${q.size ? `?${q}` : ""}`;
};

// diagnosisPanel: o que mais custa e onde dá para guardar, com o efeito de cada
// corte no plano. Tudo calculado pelas regras (finance.Diagnose), sem IA.
function diagnosisPanel(c, v, ledger) {
  const m = finance.money;
  const { diag } = c;
  if (!diag.costliest && !diag.levers.length) return "";
  const base = c.cuts ? finance.debts(ledger, c.typed).plan : null;
  return html`<section class="panel diagnosis">
    <h2>Onde está o problema</h2>
    <ul class="diag-facts">
      ${diag.costliest ? html`<li><strong>${diag.costliest.name}</strong> é a dívida mais cara: ${m(diag.costliest.monthlyInterestCents)} de juros por mês (${ratePct(diag.costliest.monthlyRateBp)} ao mês).</li>` : ""}
      ${diag.topCategory ? html`<li>O que mais pesou em ${diag.monthLabel}: <strong>${diag.topCategory.name}</strong>, ${m(diag.topCategory.cents)} (${diag.topCategory.sharePct}% dos gastos).</li>` : ""}
    </ul>
    ${diag.levers.length ? html`<h3>Onde dá para guardar</h3>
      <p class="small muted">Marque o que dá para fazer: o valor vai para as dívidas e o plano abaixo se refaz.</p>
      <ul class="levers">${diag.levers.map((l) => html`<li><label class="check">
        <input type="checkbox" data-lever="${l.id}" ${c.ids.has(l.id) ? "checked" : ""}>
        <span><strong>${l.title}</strong> · ${m(l.monthlyCents)} por mês
          <small class="muted">${l.text}</small>
          ${l.ends ? html`<small class="pos">Sozinho, já faz as dívidas terem fim.</small>`
            : l.monthsSooner > 0 ? html`<small class="pos">Sozinho: sai ${duration(l.monthsSooner)} antes e paga ${m(l.interestSavedCents)} a menos de juros.</small>` : ""}</span>
      </label></li>`)}</ul>
      ${c.cuts ? html`<p class="notice ok">Com o que você marcou, vão <strong>${m(c.cuts)}</strong> a mais por mês para as dívidas${base?.ok && v.plan.ok && base.months > v.plan.months ? `: sai ${duration(base.months - v.plan.months)} antes e paga ${m(base.interestCents - v.plan.interestCents)} a menos de juros` : !base?.ok && v.plan.ok ? ": com isso as dívidas passam a ter fim" : ""}.</p>` : ""}` : ""}
  </section>`;
}

export function render({ ledger, params }) {
  const c = chosen(ledger, params);
  const extra = c.extra;
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

    ${v.debts.length ? diagnosisPanel(c, v, ledger) : ""}
    ${v.debts.length ? planPanel(v, c) : ""}
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
      ctx.go(planURL(cents, chosen(ctx.ledger, ctx.params).ids));
    } catch (err) {
      ctx.error(err);
    }
  });
  root.querySelectorAll("[data-lever]").forEach((box) => box.addEventListener("change", () => {
    const c = chosen(ctx.ledger, ctx.params);
    if (box.checked) c.ids.add(box.dataset.lever);
    else c.ids.delete(box.dataset.lever);
    ctx.go(planURL(c.typed, c.ids));
  }));
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
