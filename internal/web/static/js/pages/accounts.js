// Contas e cartões: cadastro, saldo de cada um, fatura atual e próxima do
// cartão (pelo dia de fechamento) e o patrimônio mês a mês. As contas são de
// finance.Accounts e finance.NetWorth (Go, pelo WebAssembly); aqui só desenha.
import * as finance from "../finance.js";
import { centsInput, closeModal, html, openModal, raw, today, toast } from "../ui.js";
import { PLUS, pageHead } from "./dashboard.js";
import { chart } from "../charts.js";

export const ACCOUNT_KINDS = { corrente: "Conta corrente", poupanca: "Poupança", carteira: "Carteira", cartao: "Cartão de crédito" };
const SHORT = { corrente: "Corrente", poupanca: "Poupança", carteira: "Carteira", cartao: "Cartão" };

const svg = (d) => raw(`<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="${d}"/></svg>`);
const ICON = {
  corrente: svg("M3 10l9-6 9 6M5 10v8M9.5 10v8M14.5 10v8M19 10v8M3 21h18"),
  poupanca: svg("M19 7c-1-2-3.5-3-7-3-5 0-8 2.5-8 6.5 0 2 1 3.7 2.5 4.8V19h3v-2h5v2h3v-3.2c1-.7 1.8-1.7 2.2-2.8H21v-4h-1.8c-.3-.7-.7-1.4-1.2-2zM15 9h.01"),
  carteira: svg("M20 7H5a2 2 0 0 1 0-4h13v4M3 5v14a2 2 0 0 0 2 2h15V7M16 14h.01"),
  chevron: svg("M9 18l6-6-6-6"),
  close: svg("M18 6L6 18M6 6l12 12"),
};

const ddmm = (date) => `${date.slice(8, 10)}/${date.slice(5, 7)}`;
// signedInput mostra centavos com sinal no campo ("-350,00").
const signedInput = (c) => (c < 0 ? "-" : "") + centsInput(Math.abs(c));

function invoiceBlock(title, inv, m) {
  return html`<div class="invoice ${inv.closed ? "closed" : ""}">
    <div class="invoice-top"><span>${title} · <strong>${inv.label}</strong></span>
      <span class="goal-chip ${inv.closed ? "warn" : "plain"}">${inv.closed ? "Fechada · pagar" : "Aberta"}</span></div>
    <strong class="invoice-total">${m(inv.totalCents)}</strong>
    <span class="small muted">${inv.closed ? "fechou" : "fecha"} ${ddmm(inv.closingDate)} · vence ${ddmm(inv.dueDate)}</span>
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
  const head = pageHead("Contas e cartões", null, { kicker: "Saldos, faturas e patrimônio", actions: newButton });
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
  const biggest = Math.max(1, ...plain.map((a) => Math.abs(a.balanceCents)));
  const net = finance.netWorth(ledger, today()).at(-1).netCents;
  return html`${head}
    <section class="cards acct-kpis">
      <div class="card"><span>Nas contas</span><strong class="${v.accountsCents < 0 ? "neg" : ""}">${m(v.accountsCents)}</strong>
        <span class="faint small">${plain.length} ${plain.length === 1 ? "conta" : "contas"}, sem os cartões</span></div>
      <div class="card"><span>Em aberto nos cartões</span><strong class="${v.cardsOpenCents > 0 ? "neg" : ""}">${m(v.cardsOpenCents)}</strong>
        <span class="faint small">${cards.length} ${cards.length === 1 ? "cartão" : "cartões"}</span></div>
      <div class="card hero"><span>Patrimônio</span><strong class="${net < 0 ? "neg" : ""}">${m(net)}</strong>
        <span class="hero-label">contas − cartões em aberto − dívidas</span></div>
    </section>

    ${cards.length ? html`<section class="panel">
      <div class="panel-head"><h2>Cartões</h2></div>
      <div class="card-list">${cards.map((a) => html`<div class="account card-row">
        <button type="button" class="card-tile" data-edit="${a.id}" aria-label="Editar ${a.name}">
          <span class="card-tile-top"><strong>${a.name}</strong><span class="card-chip" aria-hidden="true"></span></span>
          <span class="card-tile-open">${a.balanceCents < 0 ? `${m(-a.balanceCents)} em aberto` : m(a.balanceCents)}</span>
          <span class="card-tile-days">fecha dia ${a.closingDay} · vence dia ${a.dueDay}</span>
        </button>
        ${a.current ? html`<div class="invoices">${invoiceBlock("Fatura atual", a.current, m)}${invoiceBlock("Próxima", a.next, m)}</div>` : ""}
      </div>`)}</div>
      <p class="notice warn">Pague a fatura inteira para não cair no rotativo, o juro mais caro do país. Se não der, fale com o Bento no Consultor.</p>
    </section>` : ""}

    ${plain.length ? html`<section class="panel">
      <div class="panel-head"><h2>Contas</h2><span class="faint small">Toque numa conta para editar</span></div>
      <ul class="plain acct-list">${plain.map((a) => html`<li><button type="button" class="account acct-row" data-edit="${a.id}">
        <span class="icon-tile" style="--c: var(--accent); --s: 40px">${ICON[a.kind]}</span>
        <span class="acct-name"><strong>${a.name}</strong><small class="muted">${ACCOUNT_KINDS[a.kind]}</small></span>
        <span class="acct-balance"><strong class="${a.balanceCents < 0 ? "neg" : ""}">${m(a.balanceCents)}</strong>
          <span class="bar"><span style="width: ${Math.round((Math.abs(a.balanceCents) * 100) / biggest)}%"></span></span></span>
        <span class="acct-chevron">${ICON.chevron}</span>
      </button></li>`)}</ul>
    </section>` : ""}

    ${netWorthPanel(ledger)}
    ${accountForm()}`;
}

export function bind(root, ctx) {
  bindNetWorth(root);
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
}
