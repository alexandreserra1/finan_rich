// Dívidas (visual Esmeralda): o diagnóstico (onde está o problema e onde dá
// para guardar), o plano para sair delas com o valor a mais (campo e controle
// deslizante, recalcula na hora), o gráfico, a ordem para quitar, o calendário
// e a lista com edição em janela. As contas são de finance.Debts e
// finance.Diagnose (Go): aqui só desenha.
import * as finance from "../finance.js";
import { centsInput, closeModal, html, openModal, raw, ratePct, shiftMonth, toast, today } from "../ui.js";
import { PLUS, pageHead } from "./dashboard.js";
import { chart } from "../charts.js";

// horizon é até onde o gráfico vai: o fim do plano, ou do "só o mínimo" se ele
// acabar; se a dívida só cresce, um pouco além do plano (senão a linha do plano some).
function horizon(v) {
  const plan = v.plan.balanceCents.length - 1, min = v.minOnly.balanceCents.length - 1;
  return v.minOnly.ok ? Math.max(plan, min) : Math.min(min, Math.max(plan * 2, 24));
}

const whenLabel = (months) => finance.monthLabel(shiftMonth(today().slice(0, 7), months));
const SLIDER_MAX = 150000; // R$ 1.500, como no design; acima disso, só pelo campo

// duration: 8 → "8 meses"; 18 → "1 ano e 6 meses"; 121 → "10 anos e 1 mês".
export function duration(months) {
  const y = Math.floor(months / 12), m = months % 12;
  const part = (n, one, many) => `${n} ${n === 1 ? one : many}`;
  if (!y) return part(m, "mês", "meses");
  return m ? `${part(y, "ano", "anos")} e ${part(m, "mês", "meses")}` : part(y, "ano", "anos");
}

const ICON = {
  check: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6L9 17l-5-5"/></svg>'),
  close: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6L6 18M6 6l12 12"/></svg>'),
  chat: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>'),
};

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
// o mínimo (tracejada). Passar o mouse mostra o mês, com a linha vertical.
function debtChart(v) {
  const m = finance.money;
  const n = Math.max(1, horizon(v));
  const label = (i) => (i ? whenLabel(i) : "Hoje");
  const upTo = (s) => Array.from({ length: n + 1 }, (_, i) => (i < s.length ? s[i] : null));
  const plan = upTo(v.plan.balanceCents), min = upTo(v.minOnly.balanceCents);
  return html`<div class="debt-chart">${chart({
    key: "dividas", type: "line", height: 220, dashed: true, cursor: true, stripInline: true,
    labels: Array.from({ length: n + 1 }, (_, i) => (i ? `${label(i).slice(0, 3)}/${label(i).slice(-2)}` : "hoje")),
    series: [
      { name: "Seguindo o plano", data: plan, color: "accent", fill: true, width: 2.5 },
      { name: "Só o mínimo", data: min, color: "neg", dash: true },
    ],
    strip: { index: 0, render: (i) => html`<b>${label(i)}</b> · plano ${plan[i] != null ? m(plan[i]) : "—"} · mínimo ${min[i] != null ? m(min[i]) : "—"}` },
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
      <div class="panel-head"><h2>Seu plano</h2><span class="faint small">Método avalanche: o extra vai para a de maior juros</span></div>
      ${v.plan.ok
        ? html`<p class="plan-lead">${v.debts.length > 1 ? html`Pague primeiro <b>${first}</b>. ` : ""}Pagando <b class="num">${m(monthly)}</b> por mês, você ${v.debts.length > 1 ? "sai das dívidas" : html`quita <b>${first}</b>`} em
            <b class="accent">${duration(v.plan.months)}</b> (${whenLabel(v.plan.months)}) e paga ${m(v.plan.interestCents)} de juros.</p>
          ${v.plan.paidOff.length > 1 ? html`<p class="small muted">${v.plan.paidOff.length} dívidas: a primeira acaba em ${whenLabel(v.plan.paidOff[0].month)}, a última em ${whenLabel(v.plan.months)}.</p>` : ""}
          ${v.minOnly.ok
            ? saved > 0 ? html`<p class="notice ok compare"><span>Só no mínimo seriam <b>${duration(v.minOnly.months)}</b> e ${m(v.minOnly.interestCents)} de juros.</span>
                <span class="pos"><b>Você economiza ${m(saved)}</b>${v.minOnly.months > v.plan.months ? ` e sai ${duration(v.minOnly.months - v.plan.months)} antes` : ""}.</span></p>` : ""
            : html`<p class="notice warn">Só no mínimo as dívidas não acabam: os juros passam do que você paga.</p>`}`
        : html`<p class="notice warn">Com ${m(monthly)} por mês as dívidas não acabam: os juros passam do que você paga. Aumente o valor abaixo, ou converse com o banco sobre trocar por uma dívida mais barata.</p>`}
      <form data-form="extra" class="extra-form">
        <label class="field"><span>Quanto dá para pagar <b>a mais</b> por mês</span>
          <span class="money-field"><span>R$</span><input name="extra" inputmode="decimal" value="${c.typed ? centsInput(c.typed) : ""}" placeholder="0,00" autocomplete="off"></span></label>
        <input type="range" name="slider" min="0" max="${SLIDER_MAX / 100}" step="50" value="${Math.min(c.typed, SLIDER_MAX) / 100}" aria-label="Valor a mais por mês, em reais">
      </form>
      ${c.cuts ? html`<p class="small muted">Mais ${m(c.cuts)} dos cortes marcados.</p>` : ""}
      ${debtChart(v)}
      ${v.plan.paidOff.length > 1 ? html`<h3 class="order-title">Ordem para quitar</h3>
        <ol class="paid-off">${v.plan.paidOff.map((p) => html`<li><strong>${p.name}</strong><span class="faint small">zera em ${whenLabel(p.month)}</span></li>`)}</ol>` : ""}
      ${v.plan.ok ? calendarTable(v) : ""}
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
      ${diag.costliest ? html`<li class="neg-dot"><span><b>${diag.costliest.name}</b> é a dívida mais cara: ${m(diag.costliest.monthlyInterestCents)} de juros por mês (${ratePct(diag.costliest.monthlyRateBp)} ao mês).</span></li>` : ""}
      ${diag.topCategory ? html`<li class="warn-dot"><span>O que mais pesou em ${diag.monthLabel}: <b>${diag.topCategory.name}</b>, ${m(diag.topCategory.cents)} (${diag.topCategory.sharePct}% dos gastos).</span></li>` : ""}
    </ul>
    ${diag.levers.length ? html`<div class="levers-head"><h3>Onde dá para guardar</h3>
        <span class="faint small">Marque o que dá para fazer: o valor vai para as dívidas e o plano se refaz.</span></div>
      <ul class="levers">${diag.levers.map((l) => html`<li><label class="lever ${c.ids.has(l.id) ? "on" : ""}">
        <input type="checkbox" data-lever="${l.id}" ${c.ids.has(l.id) ? "checked" : ""}>
        <span class="lever-body">
          <span class="lever-top"><strong>${l.title}</strong><span class="num">${m(l.monthlyCents)}/mês</span></span>
          <span class="muted small">${l.text}</span>
          ${l.ends ? html`<span class="pos small">Sozinho, já faz as dívidas terem fim.</span>`
            : l.monthsSooner > 0 ? html`<span class="pos small">Sozinho: sai ${duration(l.monthsSooner)} antes e paga ${m(l.interestSavedCents)} a menos de juros.</span>` : ""}
        </span>
      </label></li>`)}</ul>
      ${c.cuts ? html`<p class="notice ok">Com o que você marcou, vão <b>${m(c.cuts)}</b> a mais por mês para as dívidas${base?.ok && v.plan.ok && base.months > v.plan.months ? `: sai ${duration(base.months - v.plan.months)} antes e paga ${m(base.interestCents - v.plan.interestCents)} a menos de juros` : !base?.ok && v.plan.ok ? ": com isso as dívidas passam a ter fim" : ""}.</p>` : ""}` : ""}
  </section>`;
}

function debtForm() {
  return html`<div class="modal" id="debt-modal" hidden>
    <div class="modal-scrim" data-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="debt-title">
      <form data-form="debt" class="modal-form" novalidate>
        <div class="modal-head"><h2 id="debt-title">Nova dívida</h2><button type="button" class="icon-btn" data-close aria-label="Fechar">${ICON.close}</button></div>
        <label class="field">Nome <input name="name" placeholder="Ex.: Cartão Nubank" autocomplete="off" autofocus>
          <span class="field-error" data-error="name" hidden></span></label>
        <div class="field-grid">
          <label class="field">Credor <input name="creditor" placeholder="Banco ou loja" autocomplete="off"></label>
          <label class="field">Saldo devedor (R$) <input name="balance" inputmode="decimal" placeholder="0,00" autocomplete="off">
            <span class="field-error" data-error="balance" hidden></span></label>
          <label class="field">Juros ao mês (%) <input name="rate" inputmode="decimal" placeholder="Ex.: 12,5" autocomplete="off"></label>
          <label class="field">Parcela mínima (R$) <input name="min_payment" inputmode="decimal" placeholder="0,00" autocomplete="off"></label>
          <label class="field">Dia do vencimento <input type="number" name="due_day" min="1" max="31"></label>
        </div>
        <div class="modal-foot">
          <button type="button" class="ghost danger edit-only" data-action="delete-editing">Apagar</button>
          <span class="spacer"></span>
          <button type="button" class="secondary" data-close>Cancelar</button>
          <button type="submit">Salvar</button>
        </div>
      </form>
    </div>
  </div>`;
}

export function render({ ledger, params }) {
  const c = chosen(ledger, params);
  const v = finance.debts(ledger, c.extra);
  const m = finance.money;
  const cards = ledger.accounts.some((a) => a.kind === "cartao" && !a.deleted) ? finance.accounts(ledger, today()).cardsOpenCents : 0;
  const income = finance.dashboard(ledger, today().slice(0, 7)).inCents;
  const newButton = html`<button type="button" data-action="new-debt">${PLUS}Nova dívida</button>`;

  if (!v.debts.length) {
    return html`${pageHead("Dívidas", null, { kicker: "Plano para sair delas", actions: newButton })}
      <section class="panel empty-state debt-empty">
        <span class="ok-tile" aria-hidden="true">${ICON.check}</span>
        <strong>Nenhuma dívida cadastrada. Ótimo sinal!</strong>
        <span class="muted small">Tem cartão no rotativo, empréstimo ou cheque especial? Cadastrando, o app mostra quanto de juros ela gera e qual pagar primeiro.</span>
        <button type="button" class="secondary" data-action="new-debt">Cadastrar dívida</button>
      </section>
      ${cards > 0 ? cardsNotice(cards, m) : ""}
      ${debtForm()}`;
  }

  const diagnosis = diagnosisPanel(c, v, ledger);
  return html`
    ${pageHead("Dívidas", null, { kicker: "Plano para sair delas", actions: newButton })}
    <section class="cards debt-kpis">
      <div class="card"><span>Total devido</span><strong>${m(v.totalCents)}</strong><span class="faint small">${v.debts.length} ${v.debts.length === 1 ? "dívida ativa" : "dívidas ativas"}</span></div>
      <div class="card"><span>Parcelas mínimas / mês</span><strong>${m(v.totalMinCents)}</strong>
        <span class="faint small">${income ? `${Math.round((v.totalMinCents * 100) / income)}% da sua renda` : "lance o salário para ver quanto da renda vai para elas"}</span></div>
      <div class="card interest"><span>Juros gerados / mês</span><strong class="neg">${m(v.monthlyInterestCents)}</strong><span class="faint small">dinheiro que não abate nada</span></div>
    </section>

    <section class="debt-row ${diagnosis ? "" : "solo"}">
      ${diagnosis}
      ${planPanel(v, c)}
    </section>
    ${cards > 0 ? cardsNotice(cards, m) : ""}

    <section class="panel debt-list">
      <div class="panel-head"><h2>Suas dívidas</h2><span class="faint small">Toque numa dívida para editar</span></div>
      <div class="table-wrap"><table class="debt-table">
        <thead><tr><th>Dívida</th><th class="num">Saldo</th><th class="num">Juros a.m.</th><th class="num">Mínimo</th><th>Só pagando o mínimo</th></tr></thead>
        <tbody>
          ${v.debts.map((d) => html`<tr class="debt-row-item" data-edit="${d.id}" tabindex="0">
            <td class="primary"><strong>${d.name}</strong><span class="faint small">${[d.creditor, d.dueDay ? `vence dia ${d.dueDay}` : ""].filter(Boolean).join(" · ")}</span></td>
            <td data-label="Saldo" class="num strong">${m(d.balanceCents)}</td>
            <td data-label="Juros a.m." class="num ${d.monthlyRateBp > 800 ? "neg strong" : ""}">${ratePct(d.monthlyRateBp)}</td>
            <td data-label="Mínimo" class="num">${m(d.minPaymentCents)}</td>
            <td data-label="Só o mínimo">${d.payable
              ? `${duration(d.months)} · ${m(d.interestCents)} de juros`
              : d.minPaymentCents ? html`<span class="neg">Não cobre os juros: a dívida cresce</span>` : html`<span class="muted">informe o pagamento</span>`}</td>
          </tr>`)}
        </tbody>
      </table></div>
    </section>

    <a class="btn secondary" href="/chat">${ICON.chat}Tirar dúvidas com o consultor IA</a>
    ${debtForm()}`;
}

const cardsNotice = (cards, m) => html`<p class="notice warn">Faturas abertas nos cartões: <b>${m(cards)}</b> (veja em <a href="/contas">Contas</a>). Pague a fatura inteira para não cair no rotativo, o juro mais caro do país.</p>`;

export function bind(root, ctx) {
  bindExtra(root, ctx);
  root.querySelectorAll("[data-lever]").forEach((box) => box.addEventListener("change", () => {
    const c = chosen(ctx.ledger, ctx.params);
    if (box.checked) c.ids.add(box.dataset.lever);
    else c.ids.delete(box.dataset.lever);
    ctx.go(planURL(c.typed, c.ids));
  }));

  const modal = root.querySelector("#debt-modal");
  const form = root.querySelector('[data-form="debt"]');
  const f = form.elements;
  let editing = null;
  const open = (d, trigger) => {
    editing = d || null;
    form.reset();
    form.querySelectorAll("[data-error]").forEach((el) => { el.hidden = true; });
    modal.classList.toggle("editing", !!d);
    form.querySelector("#debt-title").textContent = d ? "Editar dívida" : "Nova dívida";
    if (d) {
      f.name.value = d.name;
      f.creditor.value = d.creditor || "";
      f.balance.value = centsInput(d.balanceCents);
      f.rate.value = d.monthlyRateBp ? centsInput(d.monthlyRateBp) : "";
      f.min_payment.value = d.minPaymentCents ? centsInput(d.minPaymentCents) : "";
      f.due_day.value = d.dueDay || "";
    }
    openModal(modal, trigger);
  };
  root.querySelectorAll('[data-action="new-debt"]').forEach((b) => b.addEventListener("click", () => open(null, b)));
  root.querySelectorAll("tr[data-edit]").forEach((row) => {
    const edit = () => open(ctx.ledger.debts.find((x) => x.id === row.dataset.edit), row);
    row.addEventListener("click", edit);
    row.addEventListener("keydown", (e) => { if (e.key === "Enter") edit(); });
  });
  const wanted = ctx.ledger.debts.find((d) => d.id === ctx.params.get("editar") && !d.deleted);
  if (wanted) open(wanted);

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const errors = {};
    const parse = (input, label) => {
      if (!input.value.trim()) return 0;
      try { return finance.parseMoney(input.value); } catch (err) { throw new Error(`${label}: ${err.message}`); }
    };
    if (!f.name.value.trim()) errors.name = "Dê um nome à dívida.";
    let balance = 0;
    try { balance = parse(f.balance, "saldo devedor"); } catch {}
    if (!(balance > 0)) errors.balance = "Valor inválido. Use o formato 1.234,56.";
    form.querySelectorAll("[data-error]").forEach((el) => {
      el.hidden = !errors[el.dataset.error];
      el.textContent = errors[el.dataset.error] || "";
      f[el.dataset.error]?.setAttribute("aria-invalid", errors[el.dataset.error] ? "true" : "false");
    });
    if (Object.keys(errors).length) return;
    try {
      await ctx.save("debt", {
        ...(editing ? { id: editing.id } : {}),
        name: f.name.value.trim(), creditor: f.creditor.value.trim(), balanceCents: balance,
        monthlyRateBp: parse(f.rate, "juros"), // "12,5" → 1250 pontos-base, mesmo parser do dinheiro
        minPaymentCents: parse(f.min_payment, "parcela"),
        dueDay: Number(f.due_day.value) || 0,
      }, "Dívida salva.");
      if (ctx.params.get("editar")) ctx.go("/dividas");
    } catch (err) {
      ctx.error(err);
    }
  });

  form.querySelector('[data-action="delete-editing"]').addEventListener("click", async () => {
    if (!editing) return;
    const d = editing;
    closeModal(modal);
    await ctx.remove("debt", d);
    toast(`${d.name} apagada.`, () => ctx.save("debt", { ...d, deleted: false }, "Dívida de volta."));
  });
}

// bindExtra: o valor a mais recalcula o plano enquanto a pessoa digita ou
// arrasta (o endereço guarda o valor, sem encher o histórico).
function bindExtra(root, ctx) {
  const form = root.querySelector('[data-form="extra"]');
  if (!form) return;
  const { extra, slider } = form.elements;
  let timer;
  const apply = (cents, focusName) => {
    const url = planURL(cents, chosen(ctx.ledger, ctx.params).ids);
    if (url === location.pathname + location.search) return;
    const caret = extra.selectionStart;
    history.replaceState(null, "", url);
    ctx.rerender();
    const again = root.querySelector(`[data-form="extra"] [name="${focusName}"]`);
    again?.focus();
    if (focusName === "extra" && again) again.setSelectionRange(caret, caret);
  };
  form.addEventListener("submit", (e) => e.preventDefault());
  extra.addEventListener("input", () => {
    clearTimeout(timer);
    timer = setTimeout(() => {
      let cents = 0;
      try { cents = extra.value.trim() ? finance.parseMoney(extra.value) : 0; } catch { return; } // ainda digitando
      apply(Math.max(0, cents), "extra");
    }, 350);
  });
  slider.addEventListener("input", () => {
    extra.value = slider.value === "0" ? "" : centsInput(Number(slider.value) * 100);
    apply(Number(slider.value) * 100, "slider");
  });
}
