// Metas: juntar um valor até um prazo. O progresso soma o que já estava
// guardado com as saídas lançadas na categoria da meta (ex.: "Reserva de
// emergência"), e a tela diz quanto guardar por mês para chegar lá.
import * as finance from "../finance.js";
import { BUCKETS, centsInput, html, today, addPanel } from "../ui.js";
import { pageHead } from "./dashboard.js";

export function goalCards(views, m, { compact = false } = {}) {
  return html`<ul class="goals">
    ${views.map((g) => html`<li class="goal ${g.done ? "done" : ""} ${g.late ? "late" : ""}">
      <div class="row"><strong>${g.name}</strong><span class="small muted">${g.deadline ? `até ${finance.monthLabel(g.deadline)}` : "sem prazo"}</span></div>
      <div class="bar"><span style="width: ${g.pct}%"></span></div>
      <div class="row small">
        <span>${m(g.progressCents)} de ${m(g.targetCents)} (${g.pct}%)</span>
        <span class="${g.late ? "neg" : g.done ? "pos" : "muted"}">${g.done ? "meta atingida! 🎉" : g.late ? `prazo passou · faltam ${m(g.remainingCents)}`
          : g.perMonthCents ? `guarde ${m(g.perMonthCents)}/mês` : `faltam ${m(g.remainingCents)}`}</span>
      </div>
      ${compact ? "" : html`<div class="note-actions">
        <a href="/metas?editar=${g.id}">editar</a>
        <button type="button" class="link" data-delete="${g.id}">apagar</button>
      </div>`}
    </li>`)}
  </ul>`;
}

export function render({ ledger, params }) {
  const m = finance.money;
  const views = finance.goals(ledger, today());
  const editing = ledger.goals.find((g) => g.id === params.get("editar") && !g.deleted);
  const e = editing || {};
  const cats = ledger.categories.filter((c) => !c.deleted && c.bucket !== "renda").sort((a, b) => a.name.localeCompare(b.name, "pt-BR"));
  return html`
    ${pageHead("Metas")}
    ${addPanel("goal", editing ? "Editar meta" : "+ Nova meta", html`
      <form data-form="goal" class="grid-form">
        <label class="wide">Nome <input name="name" required maxlength="80" value="${e.name || ""}" placeholder="Ex.: Reserva de emergência, Viagem, Trocar de carro"></label>
        <label class="half">Quanto quer juntar (R$) <input name="target" required inputmode="decimal" value="${editing ? centsInput(e.targetCents) : ""}" placeholder="10.000,00"></label>
        <label class="half">Já tem guardado (R$) <input name="saved" inputmode="decimal" value="${editing ? centsInput(e.savedCents) : ""}" placeholder="0,00"></label>
        <label class="half">Prazo <input type="month" name="deadline" value="${e.deadline || ""}"></label>
        <label>Aportes vêm da categoria
          <select name="category">
            <option value="">Nenhuma (atualizo o valor guardado à mão)</option>
            ${cats.map((c) => html`<option value="${c.id}" ${c.id === e.categoryId ? "selected" : ""}>${c.name} · ${BUCKETS[c.bucket]}</option>`)}
          </select>
        </label>
        <div class="form-actions">
          <button type="submit">Salvar meta</button>
          ${editing ? html`<a href="/metas">Cancelar</a>` : ""}
        </div>
      </form>
      <p class="small muted">Com uma categoria escolhida, cada gasto lançado nela a partir de hoje conta como dinheiro guardado para a meta (ex.: transferência para a poupança na categoria "Reserva de emergência").</p>`, !!editing || !views.length)}
    <section class="panel">
      <h2>Suas metas</h2>
      ${views.length ? goalCards(views, m) : html`<p class="empty">Nenhuma meta ainda. Comece pela reserva de emergência: 3 a 6 meses dos seus gastos.</p>`}
    </section>
    <p class="small"><a href="/chat">Conversar com o consultor IA sobre como chegar nas metas →</a></p>`;
}

export function bind(root, ctx) {
  const form = root.querySelector('[data-form="goal"]');
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = form.elements;
    const editing = ctx.ledger.goals.find((g) => g.id === ctx.params.get("editar"));
    try {
      const money = (input) => (input.value.trim() ? finance.parseMoney(input.value) : 0);
      await ctx.save("goal", {
        ...(editing ? { id: editing.id } : { startDate: today() }),
        name: f.name.value.trim(), targetCents: money(f.target), savedCents: money(f.saved),
        deadline: f.deadline.value || undefined, categoryId: f.category.value || undefined,
      }, "Meta salva.");
      if (editing) ctx.go("/metas");
    } catch (err) {
      ctx.error(err);
    }
  });
  root.querySelectorAll("[data-delete]").forEach((btn) => btn.addEventListener("click", async () => {
    const g = ctx.ledger.goals.find((x) => x.id === btn.dataset.delete);
    if (!confirm(`Apagar a meta "${g.name}"?`)) return;
    await ctx.remove("goal", g, "Meta apagada.");
  }));
}
