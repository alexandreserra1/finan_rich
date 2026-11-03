// Contas e cartões: cadastro, saldo de cada um, fatura atual e próxima do
// cartão (pelo dia de fechamento) e o patrimônio mês a mês. As contas são de
// finance.Accounts e finance.NetWorth (Go, pelo WebAssembly); aqui só desenha.
import * as finance from "../finance.js";
import { centsInput, closeModal, html, openModal, raw, tilt, today, toast } from "../ui.js";
import { PLUS, metric, pageHead } from "./dashboard.js";
import { chart } from "../charts.js";

export const ACCOUNT_KINDS = { corrente: "Conta corrente", poupanca: "Poupança", carteira: "Carteira", cartao: "Cartão de crédito" };
const SHORT = { corrente: "Corrente", poupanca: "Poupança", carteira: "Carteira", cartao: "Cartão" };

const svg = (d) => raw(`<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="${d}"/></svg>`);
const ICON = {
  corrente: svg("M3 10l9-6 9 6M5 10v8M9.5 10v8M14.5 10v8M19 10v8M3 21h18"),
  poupanca: svg("M19 7c-1-2-3.5-3-7-3-5 0-8 2.5-8 6.5 0 2 1 3.7 2.5 4.8V19h3v-2h5v2h3v-3.2c1-.7 1.8-1.7 2.2-2.8H21v-4h-1.8c-.3-.7-.7-1.4-1.2-2zM15 9h.01"),
  carteira: svg("M20 7H5a2 2 0 0 1 0-4h13v4M3 5v14a2 2 0 0 0 2 2h15V7M16 14h.01"),
  chevron: svg("M9 18l6-6-6-6"),
  pencil: svg("M4 20h4L19 9l-4-4L4 16v4zM14 6l4 4"),
  close: svg("M18 6L6 18M6 6l12 12"),
  swap: svg("M7 7h13M16 3l4 4-4 4M17 17H4M8 13l-4 4 4 4"),
  trash: svg("M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6"),
};

const CHIP = raw('<svg class="rc-chip" viewBox="0 0 52 40" aria-hidden="true"><rect x="1" y="1" width="50" height="38" rx="8" fill="#dac994" stroke="#b4a16a"/><path d="M18 1v11H1m50 0H34V1M18 39V28H1m50 0H34v11M18 12h16v16H18zM1 20h17m16 0h17" fill="none" stroke="#998752"/></svg>');

const ddmm = (date) => `${date.slice(8, 10)}/${date.slice(5, 7)}`;
// signedInput mostra centavos com sinal no campo ("-350,00").
const signedInput = (c) => (c < 0 ? "-" : "") + centsInput(Math.abs(c));

// invoiceBlock é uma fatura; a fechada e ainda não paga ganha o "Pagar fatura".
function invoiceBlock(title, inv, m, cardId) {
  const due = inv.closed && !inv.paid && inv.totalCents > inv.paidCents;
  return html`<div class="invoice ${inv.closed ? "closed" : ""} ${inv.paid ? "paid" : ""}">
    <div class="invoice-top"><span>${title} · <strong>${inv.label}</strong></span>
      <span class="goal-chip ${inv.paid ? "pos" : inv.closed ? "warn" : "plain"}">${inv.paid ? "Paga" : inv.closed ? "Fechada · pagar" : "Aberta"}</span></div>
    <strong class="invoice-total">${m(inv.totalCents)}</strong>
    <span class="small muted">${inv.closed ? "fechou" : "fecha"} ${ddmm(inv.closingDate)} · vence ${ddmm(inv.dueDate)}${inv.paidCents && !inv.paid ? ` · pago ${m(inv.paidCents)}` : ""}</span>
    ${due ? html`<button type="button" class="small-btn invoice-pay" data-pay-invoice="${cardId}" data-amount="${inv.totalCents - inv.paidCents}" data-label="${inv.label}">Pagar fatura</button>` : ""}
    ${inv.items.length ? html`<details class="invoice-items"><summary>${inv.items.length} ${inv.items.length === 1 ? "lançamento" : "lançamentos"}${ICON.chevron}</summary>
      <ul class="plain">${inv.items.map((t) => html`<li class="row"><span>${t.description} <small class="muted">${ddmm(t.date)}</small></span>
        <span class="${t.kind === "entrada" ? "pos" : ""}">${t.kind === "entrada" ? "−" : ""}${m(t.amountCents)}</span></li>`)}</ul>
    </details>` : ""}
  </div>`;
}

// netWorthPanel é o patrimônio dos últimos 12 meses (também usado na Análise).
export function netWorthPanel(ledger) {
  if (!ledger.accounts.some((a) => !a.deleted)) {
    return html`<section class="panel"><h2>Patrimônio</h2>
      <p class="empty">Cadastre suas <a href="/contas">contas e cartões</a> para ver o patrimônio mês a mês.</p></section>`;
  }
  const m = finance.money;
  const points = finance.netWorth(ledger, today());
  const last = points[points.length - 1];
  const diff = last.netCents - points[0].netCents;
  const strip = (i) => {
    const p = points[i];
    return html`<b>${p.label}</b> · contas ${m(p.accountsCents)} · cartões ${m(p.cardsCents)} · dívidas ${m(p.debtsCents)} · <b class="${p.netCents < 0 ? "neg" : ""}">${m(p.netCents)}</b>`;
  };
  return html`<section class="panel networth">
    <div class="chart-head">
      <div><h2>Patrimônio · últimos 12 meses</h2>
        <p class="small muted">${diff >= 0 ? "Subiu" : "Caiu"} ${m(Math.abs(diff))} em 12 meses. Dívidas entram pelo saldo de hoje em todos os meses.</p></div>
      <strong class="${last.netCents < 0 ? "neg" : "pos"}">${m(last.netCents)}</strong>
    </div>
    <div data-nw-chart>${chart({
      key: "patrimonio", type: "line", height: 220, cursor: true,
      labels: points.map((p) => p.label.slice(0, 3)),
      series: [{ name: "Patrimônio", data: points.map((p) => p.netCents), color: last.netCents < 0 ? "neg" : "accent", fill: true, width: 2.5 }],
      legend: false,
      strip: { index: points.length - 1, render: strip },
    }, `Patrimônio nos últimos 12 meses: de ${m(points[0].netCents)} para ${m(last.netCents)}`)}</div>
    <div class="table-wrap" data-nw-table hidden><table>
      <thead><tr><th>Mês</th><th class="num">Contas</th><th class="num">Cartões</th><th class="num">Dívidas</th><th class="num">Patrimônio</th></tr></thead>
      <tbody>${points.map((p) => html`<tr><td>${p.label}</td><td class="num">${m(p.accountsCents)}</td><td class="num">${m(p.cardsCents)}</td>
        <td class="num">${m(p.debtsCents)}</td><td class="num ${p.netCents < 0 ? "neg" : "pos"}">${m(p.netCents)}</td></tr>`)}</tbody>
    </table></div>
    <div class="nw-foot"><span class="small muted">Só lançamentos com conta mexem nos saldos.</span>
      <button type="button" class="ghost small-btn" data-nw-toggle aria-pressed="false">Ver em tabela</button></div>
  </section>`;
}

// bindNetWorth liga o "Ver em tabela / Ver gráfico" (Contas e Análise).
export function bindNetWorth(root) {
  const b = root.querySelector("[data-nw-toggle]");
  b?.addEventListener("click", () => {
    const table = b.getAttribute("aria-pressed") !== "true";
    b.setAttribute("aria-pressed", String(table));
    b.textContent = table ? "Ver gráfico" : "Ver em tabela";
    root.querySelector("[data-nw-table]").hidden = !table;
    root.querySelector("[data-nw-chart]").hidden = table;
  });
}

// transferForm: mandar dinheiro de uma conta para outra (também é o "Pagar fatura").
function transferForm(accounts) {
  const opts = (sel) => accounts.map((a) => html`<option value="${a.id}" ${a.id === sel ? "selected" : ""}>${a.name}</option>`);
  return html`<div class="modal" id="transfer-modal" hidden>
    <div class="modal-scrim" data-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="transfer-title">
      <form data-form="transfer" class="modal-form" novalidate>
        <div class="modal-head"><h2 id="transfer-title">Transferir entre contas</h2><button type="button" class="icon-btn" data-close aria-label="Fechar">${ICON.close}</button></div>
        <label class="field">Valor
          <span class="amount-field"><span>R$</span><input name="amount" inputmode="decimal" placeholder="0,00" autocomplete="off" autofocus></span>
          <span class="field-error" data-error="amount" hidden></span></label>
        <div class="field-grid">
          <label class="field">De <select name="from">${opts(accounts.find((a) => a.kind !== "cartao")?.id)}</select></label>
          <label class="field">Para <select name="to">${opts(accounts.find((a) => a.kind === "cartao")?.id)}</select></label>
          <label class="field">Data <input type="date" name="date" value="${today()}"></label>
          <label class="field">Descrição <input name="description" maxlength="120" placeholder="Ex.: Pagamento da fatura" autocomplete="off"></label>
        </div>
        <span class="field-error" data-error="to" hidden></span>
        <p class="small muted">Transferência não é gasto nem entrada: só muda o saldo das duas contas. No cartão, pagar a fatura tira o valor do "em aberto" (o gasto já contou na compra).</p>
        <div class="modal-foot"><span class="spacer"></span><button type="button" class="secondary" data-close>Cancelar</button><button type="submit">Transferir</button></div>
      </form>
    </div>
  </div>`;
}

function accountForm() {
  return html`<div class="modal" id="account-modal" hidden>
    <div class="modal-scrim" data-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="account-title">
      <form data-form="account" class="modal-form" novalidate>
        <div class="modal-head"><h2 id="account-title">Nova conta ou cartão</h2><button type="button" class="icon-btn" data-close aria-label="Fechar">${ICON.close}</button></div>
        <div class="segmented big kinds" role="radiogroup" aria-label="Tipo">
          ${Object.keys(ACCOUNT_KINDS).map((k) => html`<label><input type="radio" name="kind" value="${k}" ${k === "corrente" ? "checked" : ""}><span>${SHORT[k]}</span></label>`)}
        </div>
        <label class="field">Nome <input name="name" maxlength="60" placeholder="Ex.: Nubank, Itaú, Carteira, Cartão Nubank" autocomplete="off" autofocus>
          <span class="field-error" data-error="name" hidden></span></label>
        <label class="field"><span data-initial-label>Saldo inicial (R$)</span>
          <input name="initial" inputmode="decimal" placeholder="0,00" autocomplete="off">
          <span class="field-error" data-error="initial" hidden></span></label>
        <div class="field-grid" data-card hidden>
          <label class="field">Dia do fechamento <input type="number" name="closing_day" min="1" max="31">
            <span class="field-error" data-error="closing_day" hidden></span></label>
          <label class="field">Dia do vencimento <input type="number" name="due_day" min="1" max="31">
            <span class="field-error" data-error="due_day" hidden></span></label>
        </div>
        <p class="small muted" data-hint></p>
        <div class="modal-foot">
          <button type="button" class="ghost danger edit-only" data-action="delete-editing">Apagar</button>
          <span class="spacer"></span>
          <button type="button" class="secondary" data-close>Cancelar</button>
          <button type="submit">Salvar conta</button>
        </div>
      </form>
    </div>
  </div>`;
}

export function render({ ledger }) {
  const m = finance.money;
  const v = finance.accounts(ledger, today());
  const newButton = html`<button type="button" data-action="new-account">${PLUS}Nova conta ou cartão</button>`;
  const head = pageHead("Um lugar para cada conta.", null, { sub: "Saldos, cartões e uma visão do seu patrimônio.", actions: newButton });
  if (!v.accounts.length) {
    return html`${head}
      <section class="acct-empty">
        <div class="panel">
          <strong>Onde o seu dinheiro fica?</strong>
          <span class="muted">Cadastre o banco, a poupança, a carteira e os cartões. Ao lançar, escolha a conta: o app soma o saldo de cada uma, monta a fatura do cartão pelo dia de fechamento e mostra o seu patrimônio mês a mês.</span>
          <div><button type="button" data-action="new-account">Cadastrar a primeira conta</button></div>
        </div>
        <div class="acct-empty-art" aria-hidden="true"><span class="card-tile"></span><span class="card-tile back"></span></div>
      </section>
      ${accountForm()}`;
  }
  const cards = v.accounts.filter((a) => a.kind === "cartao");
  const plain = v.accounts.filter((a) => a.kind !== "cartao");
  const net = finance.netWorth(ledger, today()).at(-1).netCents;
  const names = Object.fromEntries(ledger.accounts.map((a) => [a.id, a.name]));
  const transfers = (ledger.transfers || []).filter((t) => !t.deleted).sort((a, b) => b.date.localeCompare(a.date) || b.updatedAt - a.updatedAt).slice(0, 8);
  return html`${head}
    <section class="cards metricgrid three acct-kpis">
      ${metric("Saldo das contas", m(v.accountsCents), `${plain.length} ${plain.length === 1 ? "conta" : "contas"}, sem os cartões`, { cls: v.accountsCents < 0 ? "neg" : "" })}
      ${metric("Cartões em aberto", m(v.cardsOpenCents), `${cards.length} ${cards.length === 1 ? "cartão" : "cartões"}`)}
      ${metric("Patrimônio líquido", m(net), "Contas − cartões − dívidas", { primary: true })}
    </section>

    ${cards.map((a) => html`<section class="credit-account account">
      <div class="credit-stage">
        <button type="button" class="rich-card card-tile" data-tilt data-edit="${a.id}" aria-label="Editar ${a.name}">
          <span class="card-orbits" aria-hidden="true"></span>
          <span class="rc-top"><span class="rc-logo">rich<span>✳</span></span><span class="rc-tag">${a.name}</span></span>
          <span class="rc-chiprow" aria-hidden="true">${CHIP}</span>
          <span class="rc-number" aria-hidden="true"><span>••••</span><span>••••</span><span>••••</span><span>••••</span></span>
          <span class="rc-bottom"><span class="card-tile-open">${a.balanceCents < 0 ? `${m(-a.balanceCents)} em aberto` : a.balanceCents > 0 ? `${m(a.balanceCents)} de crédito` : "Nada em aberto"}</span><span class="rc-mark" aria-hidden="true">✳</span></span>
        </button>
      </div>
      <div class="credit-info">
        <div class="row"><span class="eyebrow">Sua carteira · cartão</span><button type="button" class="iconbtn" data-edit="${a.id}" aria-label="Editar ${a.name}">${ICON.pencil}</button></div>
        <h2>${a.name}</h2>
        <span class="small muted">Valor em aberto</span>
        <strong class="numberbig">${m(Math.max(0, -a.balanceCents))}</strong>
        <div class="card-dates"><div><small>FECHAMENTO</small><b>Dia ${a.closingDay}</b></div><div><small>VENCIMENTO</small><b>Dia ${a.dueDay}</b></div></div>
        ${a.current ? html`<div class="invoices">${invoiceBlock("Fatura atual", a.current, m, a.id)}${invoiceBlock("Próxima", a.next, m, a.id)}</div>` : ""}
        <p class="faint tiny">Cartão ilustrativo: a Rich não emite cartões. O valor vem dos lançamentos feitos neste cartão. Pague a fatura inteira para não cair no rotativo; se não der, fale com o Otávio no Consultor.</p>
      </div>
    </section>`)}

    ${plain.length ? html`<section class="acct-grid">${plain.map((a) => html`<button type="button" class="account acct-card" data-edit="${a.id}">
      <span class="row"><span class="badge">${ACCOUNT_KINDS[a.kind]}</span><span class="acct-pencil">${ICON.pencil}</span></span>
      <span class="bank-symbol">${ICON[a.kind]}</span>
      <strong class="acct-name">${a.name}</strong>
      <strong class="numberbig ${a.balanceCents < 0 ? "neg" : ""}">${m(a.balanceCents)}</strong>
      <span class="small muted">Saldo inicial: ${m(ledger.accounts.find((x) => x.id === a.id)?.initialCents || 0)}</span>
      <span class="account-foot"><span>Saldo pelos lançamentos</span><span aria-hidden="true">↗</span></span>
    </button>`)}</section>` : ""}

    ${v.accounts.length > 1 ? html`<section class="panel transfers">
      <div class="panel-head"><div><h2>Transferências</h2><p class="faint small">Entre as suas contas: não contam como gasto nem como entrada.</p></div>
        <button type="button" class="secondary" data-action="new-transfer">${ICON.swap}Transferir</button></div>
      ${transfers.length ? html`<ul class="plain transfer-list">${transfers.map((t) => html`<li class="row">
        <span><strong>${t.description || "Transferência"}</strong><small class="muted">${ddmm(t.date)} · ${names[t.fromAccountId] || "conta apagada"} → ${names[t.toAccountId] || "conta apagada"}</small></span>
        <span class="row"><strong class="num">${m(t.amountCents)}</strong><button type="button" class="icon-btn danger" data-delete-transfer="${t.id}" aria-label="Apagar a transferência de ${m(t.amountCents)}">${ICON.trash}</button></span>
      </li>`)}</ul>` : html`<p class="empty">Nenhuma transferência ainda. Use para pagar a fatura do cartão ou mandar dinheiro para a poupança.</p>`}
    </section>` : ""}

    ${netWorthPanel(ledger)}
    ${accountForm()}
    ${v.accounts.length > 1 ? transferForm(v.accounts) : ""}`;
}

export function bind(root, ctx) {
  bindNetWorth(root);
  root.querySelectorAll("[data-tilt]").forEach(tilt);
  const modal = root.querySelector("#account-modal");
  const form = root.querySelector('[data-form="account"]');
  const f = form.elements;
  let editing = null;
  const kind = () => form.querySelector("input[name=kind]:checked").value;
  const toggle = () => {
    const card = kind() === "cartao";
    form.querySelector("[data-card]").hidden = !card;
    form.querySelector("[data-initial-label]").textContent = card ? "Em aberto hoje (R$)" : "Saldo inicial (R$)";
    form.querySelector("[data-hint]").textContent = card
      ? "A compra feita depois do fechamento cai na fatura do mês seguinte."
      : "Saldo = saldo inicial + entradas − saídas lançadas na conta. Aceita negativo (-350,00).";
  };
  form.querySelectorAll("input[name=kind]").forEach((r) => r.addEventListener("change", toggle));

  const open = (a, trigger) => {
    editing = a || null;
    form.reset();
    form.querySelectorAll("[data-error]").forEach((el) => { el.hidden = true; });
    modal.classList.toggle("editing", !!a);
    form.querySelector("#account-title").textContent = a ? "Editar conta" : "Nova conta ou cartão";
    form.querySelector(`input[name=kind][value=${a?.kind || "corrente"}]`).checked = true;
    if (a) {
      f.name.value = a.name;
      f.initial.value = a.initialCents ? signedInput(a.kind === "cartao" ? -a.initialCents : a.initialCents) : "";
      f.closing_day.value = a.closingDay || "";
      f.due_day.value = a.dueDay || "";
    }
    toggle();
    openModal(modal, trigger);
  };
  root.querySelectorAll('[data-action="new-account"]').forEach((b) => b.addEventListener("click", () => open(null, b)));
  root.querySelectorAll("[data-edit]").forEach((b) => b.addEventListener("click", () => open(ctx.ledger.accounts.find((a) => a.id === b.dataset.edit), b)));
  const wanted = ctx.ledger.accounts.find((a) => a.id === ctx.params.get("editar") && !a.deleted);
  if (wanted) open(wanted);

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const card = kind() === "cartao";
    const errors = {};
    if (!f.name.value.trim()) errors.name = "Dê um nome à conta.";
    // O campo aceita sinal ("-350,00": conta no negativo); a regra de valor é a do Go.
    const text = f.initial.value.trim();
    let value = 0;
    try { value = text ? finance.parseMoney(text.replace(/^-/, "")) * (text.startsWith("-") ? -1 : 1) : 0; } catch { errors.initial = "Valor inválido. Use o formato 1.234,56."; }
    const day = (input) => Number(input.value);
    if (card) {
      if (!(day(f.closing_day) >= 1 && day(f.closing_day) <= 31)) errors.closing_day = "Dia de 1 a 31.";
      if (!(day(f.due_day) >= 1 && day(f.due_day) <= 31)) errors.due_day = "Dia de 1 a 31.";
    }
    form.querySelectorAll("[data-error]").forEach((el) => {
      el.hidden = !errors[el.dataset.error];
      el.textContent = errors[el.dataset.error] || "";
      f[el.dataset.error]?.setAttribute("aria-invalid", errors[el.dataset.error] ? "true" : "false");
    });
    if (Object.keys(errors).length) return;
    try {
      await ctx.save("account", {
        ...(editing ? { id: editing.id } : {}),
        name: f.name.value.trim(), kind: kind(), initialCents: card ? -value : value,
        closingDay: card ? day(f.closing_day) : undefined, dueDay: card ? day(f.due_day) : undefined,
      }, "Conta salva.");
      if (ctx.params.get("editar")) ctx.go("/contas");
    } catch (err) {
      ctx.error(err);
    }
  });

  form.querySelector('[data-action="delete-editing"]').addEventListener("click", async () => {
    if (!editing) return;
    const a = editing;
    closeModal(modal);
    await ctx.remove("account", a);
    toast("Os lançamentos continuam, só sem conta.", () => ctx.save("account", { ...a, deleted: false }, "Conta de volta."));
  });
  bindTransfers(root, ctx);
}

// bindTransfers: a janela de transferir (aberta pelo "Transferir" ou pelo
// "Pagar fatura", já com o cartão e o valor) e o apagar com "Desfazer".
function bindTransfers(root, ctx) {
  const modal = root.querySelector("#transfer-modal");
  if (!modal) return;
  const form = modal.querySelector('[data-form="transfer"]');
  const f = form.elements;
  const open = (trigger, preset = {}) => {
    form.reset();
    form.querySelectorAll("[data-error]").forEach((el) => { el.hidden = true; });
    f.date.value = today();
    if (preset.to) f.to.value = preset.to;
    if (preset.amountCents) f.amount.value = centsInput(preset.amountCents);
    if (preset.description) f.description.value = preset.description;
    if (f.from.value === f.to.value) f.from.value = [...f.from.options].find((o) => o.value !== f.to.value)?.value || "";
    form.querySelector("#transfer-title").textContent = preset.title || "Transferir entre contas";
    openModal(modal, trigger);
  };
  root.querySelectorAll('[data-action="new-transfer"]').forEach((b) => b.addEventListener("click", () => open(b)));
  root.querySelectorAll("[data-pay-invoice]").forEach((b) => b.addEventListener("click", () => open(b, {
    to: b.dataset.payInvoice, amountCents: Number(b.dataset.amount), description: `Pagamento da fatura de ${b.dataset.label}`, title: "Pagar fatura",
  })));
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const errors = {};
    let cents = 0;
    try { cents = finance.parseMoney(f.amount.value); } catch {}
    if (!(cents > 0)) errors.amount = "Valor inválido. Use o formato 1.234,56.";
    if (f.from.value === f.to.value) errors.to = "Escolha contas diferentes para a origem e o destino.";
    form.querySelectorAll("[data-error]").forEach((el) => { el.hidden = !errors[el.dataset.error]; el.textContent = errors[el.dataset.error] || ""; });
    if (Object.keys(errors).length) return;
    try {
      closeModal(modal);
      await ctx.save("transfer", { fromAccountId: f.from.value, toAccountId: f.to.value, amountCents: cents, date: f.date.value || today(),
        description: f.description.value.trim() || undefined }, `Transferência de ${finance.money(cents)} salva.`);
    } catch (err) {
      ctx.error(err);
    }
  });
  root.querySelectorAll("[data-delete-transfer]").forEach((b) => b.addEventListener("click", async () => {
    const t = ctx.ledger.transfers.find((x) => x.id === b.dataset.deleteTransfer);
    await ctx.remove("transfer", t);
    toast("Transferência apagada.", () => ctx.save("transfer", { ...t, deleted: false }, "Transferência de volta."));
  }));
}
