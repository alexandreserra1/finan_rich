// Dívidas: cadastro, simulação pagando o mínimo e ordem de pagamento.
import * as finance from "../finance.js";
import { centsInput, html, ratePct } from "../ui.js";
import { pageHead } from "./dashboard.js";

export function render({ ledger, params }) {
  const v = finance.debts(ledger);
  const m = finance.money;
  const editing = ledger.debts.find((d) => d.id === params.get("editar") && !d.deleted);
  const e = editing || {};

  return html`
    ${pageHead("Dívidas")}
    <section class="cards">
      <div class="card"><span class="muted">Total devido</span><strong class="neg">${m(v.totalCents)}</strong></div>
      <div class="card"><span class="muted">Parcelas mínimas / mês</span><strong>${m(v.totalMinCents)}</strong></div>
      <div class="card"><span class="muted">Juros gerados / mês</span><strong class="neg">${m(v.monthlyInterestCents)}</strong></div>
    </section>

    <section class="panel">
      <h2>${editing ? "Editar dívida" : "Nova dívida"}</h2>
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
      </form>
    </section>

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
                ? `${d.months} meses · ${m(d.interestCents)} de juros`
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

    ${v.debts.length > 1 ? html`
      <section class="panel two">
        <div>
          <h3>Método avalanche</h3>
          <p class="small muted">Pague o mínimo em todas e jogue todo o dinheiro extra na de <strong>maior juros</strong>. É o que mais economiza.</p>
          <ol>${v.avalanche.map((n) => html`<li>${n}</li>`)}</ol>
        </div>
        <div>
          <h3>Método bola de neve</h3>
          <p class="small muted">Quite primeiro a de <strong>menor saldo</strong>. Custa um pouco mais, mas dá vitórias rápidas e motivação.</p>
          <ol>${v.snowball.map((n) => html`<li>${n}</li>`)}</ol>
        </div>
      </section>` : ""}
    <p class="small"><a href="/chat">Conversar com o consultor IA sobre um plano para sair das dívidas →</a></p>`;
}

export function bind(root, ctx) {
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
