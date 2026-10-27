// Metas: juntar um valor até um prazo. O progresso soma o que já estava
// guardado com as saídas lançadas na categoria da meta (ex.: "Reserva de
// emergência"), e a tela diz quanto guardar por mês para chegar lá. As contas
// (somas, quanto por mês, o que gravar ao guardar) vêm do Go (finance.GoalsPage,
// GoalPlan e Contribution).
import * as finance from "../finance.js";
import { BUCKETS, centsInput, closeModal, html, openModal, raw, today, toast } from "../ui.js";
import { PLUS, pageHead } from "./dashboard.js";

const CLOSE = raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6L6 18M6 6l12 12"/></svg>');
const CHAT = raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>');

// ring: o anel de progresso (r = 42, volta inteira ≈ 264).
const ring = (pct, size) => raw(`<svg class="goal-ring" width="${size}" height="${size}" viewBox="0 0 100 100" aria-hidden="true">
  <circle cx="50" cy="50" r="42" class="track"/><circle cx="50" cy="50" r="42" class="arc" style="--dash: ${(pct * 2.64).toFixed(1)}" transform="rotate(-90 50 50)"/></svg>`);

const status = (g) => (g.done ? ["Atingida", "pos"] : g.late ? ["Prazo passou", "neg"] : ["Em dia", ""]);

function advice(g, m) {
  if (g.done) return `Meta atingida: você juntou ${m(g.progressCents)}.`;
  if (g.late) return `O prazo passou e faltam ${m(g.remainingCents)}. Ajuste o prazo ou guarde um pouco a mais por mês.`;
  if (g.perMonthCents) return `Guarde ${m(g.perMonthCents)} por mês durante ${g.monthsLeft} ${g.monthsLeft === 1 ? "mês" : "meses"} para chegar lá.`;
  return `Faltam ${m(g.remainingCents)}. Com um prazo, o app diz quanto guardar por mês.`;
}

// goalCards: a versão compacta, para o Painel (anel pequeno, sem ações).
export function goalCards(views, m) {
  return html`<ul class="goals">
    ${views.map((g) => {
      const [label, tone] = status(g);
      return html`<li class="goal ${g.done ? "done" : ""} ${g.late ? "late" : ""}">
        <span class="goal-ring-wrap small">${ring(g.pct, 40)}</span>
        <span class="goal-text">
          <span class="goal-top"><strong>${g.name}</strong><span class="goal-chip ${tone}">${label}</span></span>
          <span class="small muted">${m(g.progressCents)} de ${m(g.targetCents)} · ${g.pct}%${g.perMonthCents ? ` · guarde ${m(g.perMonthCents)}/mês` : ""}</span>
        </span>
      </li>`;
    })}
  </ul>`;
}

function goalCard(g, m, cats) {
  const [label, tone] = status(g);
  const where = [g.deadline ? `até ${finance.monthLabel(g.deadline)}` : "sem prazo", g.categoryId && cats[g.categoryId] ? `aportes de ${cats[g.categoryId]}` : "atualizado à mão"];
  return html`<article class="goal-card ${g.done ? "done" : ""} ${g.late ? "late" : ""}">
    <div class="goal-main">
      <span class="goal-ring-wrap">${ring(g.pct, 96)}<strong>${g.pct}%</strong></span>
      <div class="goal-info">
        <div class="goal-top"><h2>${g.name}</h2><span class="goal-chip ${tone}">${label}</span></div>
        <span class="goal-amount"><b>${m(g.progressCents)}</b> de ${m(g.targetCents)}</span>
        <span class="faint small">${where.join(" · ")}</span>
      </div>
    </div>
    <p class="goal-advice">${advice(g, m)}</p>
    <div class="goal-actions">
      ${g.done ? "" : html`<form class="money-field goal-quick" data-quick="${g.id}">
        <span>R$</span><input name="amount" inputmode="decimal" placeholder="Quanto guardou?" aria-label="Valor para guardar em ${g.name}" autocomplete="off">
        <button type="submit" class="small-btn">Guardar</button>
      </form>`}
      <button type="button" class="secondary" data-edit="${g.id}">Editar</button>
    </div>
  </article>`;
}

function goalForm(cats) {
  return html`<div class="modal" id="goal-modal" hidden>
    <div class="modal-scrim" data-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="goal-title">
      <form data-form="goal" class="modal-form" novalidate>
        <div class="modal-head"><h2 id="goal-title">Nova meta</h2><button type="button" class="icon-btn" data-close aria-label="Fechar">${CLOSE}</button></div>
        <label class="field">Nome <input name="name" maxlength="80" placeholder="Ex.: Reserva de emergência, Viagem, Trocar de carro" autocomplete="off" autofocus>
          <span class="field-error" data-error="name" hidden></span></label>
        <div class="field-grid">
          <label class="field">Quanto quer juntar (R$) <input name="target" inputmode="decimal" placeholder="10.000,00" autocomplete="off">
            <span class="field-error" data-error="target" hidden></span></label>
          <label class="field">Já tem guardado (R$) <input name="saved" inputmode="decimal" placeholder="0,00" autocomplete="off"></label>
          <label class="field">Prazo <input type="month" name="deadline"></label>
          <label class="field">Aportes vêm da categoria
            <select name="category">
              <option value="">Nenhuma (atualizo à mão)</option>
              ${cats.map((c) => html`<option value="${c.id}">${c.name} · ${BUCKETS[c.bucket]}</option>`)}
            </select></label>
        </div>
        <p class="notice ok goal-preview" data-preview hidden></p>
        <p class="small muted">Com uma categoria, cada gasto lançado nela a partir de hoje conta como dinheiro guardado (ex.: a transferência para a poupança na categoria "Reserva de emergência").</p>
        <div class="modal-foot">
          <button type="button" class="ghost danger edit-only" data-action="delete-editing">Apagar</button>
          <span class="spacer"></span>
          <button type="button" class="secondary" data-close>Cancelar</button>
          <button type="submit">Salvar meta</button>
        </div>
      </form>
    </div>
  </div>`;
}

export function render({ ledger }) {
  const m = finance.money;
  const p = finance.goalsPage(ledger, today());
  const cats = ledger.categories.filter((c) => !c.deleted && c.bucket !== "renda").sort((a, b) => a.name.localeCompare(b.name, "pt-BR"));
  const catNames = Object.fromEntries(cats.map((c) => [c.id, c.name]));
  const head = pageHead("Metas", null, { kicker: "Juntar um valor até um prazo", actions: html`<button type="button" data-action="new-goal">${PLUS}Nova meta</button>` });

  if (!p.goals.length) {
    return html`${head}
      <section class="goal-empty">
        <div class="panel">
          <strong>Comece pela reserva de emergência.</strong>
          <span class="muted">É a primeira meta de quase todo mundo: 3 a 6 meses dos seus gastos guardados.${p.reserveCents
            ? ` Com os seus números, isso dá entre ${m(p.reserveCents)} e ${m(p.reserveCents * 2)}.` : " Lance alguns gastos e o app calcula o valor com os seus números."}</span>
          <div class="goal-empty-actions">
            <button type="button" data-action="new-reserve">Criar reserva de emergência</button>
            <button type="button" class="secondary" data-action="new-goal">Outra meta</button>
          </div>
        </div>
        <div class="goal-empty-art" aria-hidden="true">${ring(25, 150)}</div>
      </section>
      ${goalForm(cats)}`;
  }

  const fits = p.investPlannedCents - p.perMonthCents;
  return html`${head}
    <section class="sum-strip goal-strip">
      <div><span>Guardado nas metas</span><strong>${m(p.savedCents)}</strong></div>
      <div><span>Falta juntar</span><strong>${m(p.remainingCents)}</strong></div>
      <div><span>Guardar por mês, somando</span><strong class="accent">${m(p.perMonthCents)}</strong></div>
      <div><span>Cabe nos Investimentos?</span>${p.investPlannedCents
        ? html`<strong class="${fits >= 0 ? "pos" : "neg"}">${fits >= 0 ? "Sim" : "Não"}</strong><span>${fits >= 0 ? `sobram ${m(fits)} dos ${m(p.investPlannedCents)}` : `faltam ${m(-fits)} nos ${m(p.investPlannedCents)}`}</span>`
        : html`<strong class="muted">—</strong><span>lance o salário do mês para comparar</span>`}</div>
    </section>
    <section class="goal-grid">${p.goals.map((g) => goalCard(g, m, catNames))}</section>
    <a class="btn secondary" href="/chat">${CHAT}Conversar com o Tito sobre as metas</a>
    ${goalForm(cats)}`;
}

export function bind(root, ctx) {
  const m = finance.money;
  const modal = root.querySelector("#goal-modal");
  const form = root.querySelector('[data-form="goal"]');
  const f = form.elements;
  const preview = form.querySelector("[data-preview]");
  let editing = null;
  const money = (input) => (input.value.trim() ? finance.parseMoney(input.value) : 0);

  // A prévia usa a mesma conta da meta salva (finance.GoalPlan).
  const showPreview = () => {
    let left = 0;
    try { left = money(f.target) - money(f.saved); } catch { left = 0; }
    const plan = f.deadline.value && left > 0 ? finance.goalPlan(left, f.deadline.value, today()) : null;
    preview.hidden = !plan || (!plan.months && !plan.late);
    if (!plan) return;
    preview.textContent = plan.late ? "Esse prazo já passou: escolha um mês a partir de agora."
      : `Para chegar até ${finance.monthLabel(f.deadline.value)}: guarde ${m(plan.perMonthCents)} por mês durante ${plan.months} ${plan.months === 1 ? "mês" : "meses"}.`;
    preview.classList.toggle("warn", plan.late);
    preview.classList.toggle("ok", !plan.late);
  };
  ["target", "saved", "deadline"].forEach((n) => f[n].addEventListener("input", showPreview));

  const open = (g, trigger, preset = {}) => {
    editing = g || null;
    form.reset();
    form.querySelectorAll("[data-error]").forEach((el) => { el.hidden = true; });
    modal.classList.toggle("editing", !!g);
    form.querySelector("#goal-title").textContent = g ? "Editar meta" : "Nova meta";
    const v = g || preset;
    f.name.value = v.name || "";
    f.target.value = v.targetCents ? centsInput(v.targetCents) : "";
    f.saved.value = v.savedCents ? centsInput(v.savedCents) : "";
    f.deadline.value = v.deadline || "";
    f.category.value = v.categoryId || "";
    showPreview();
    openModal(modal, trigger);
  };
  root.querySelectorAll('[data-action="new-goal"]').forEach((b) => b.addEventListener("click", () => open(null, b)));
  root.querySelector('[data-action="new-reserve"]')?.addEventListener("click", (e) => {
    const reserve = ctx.ledger.categories.find((c) => !c.deleted && c.name === "Reserva de emergência");
    open(null, e.currentTarget, { name: "Reserva de emergência", targetCents: finance.goalsPage(ctx.ledger, today()).reserveCents, categoryId: reserve?.id });
  });
  root.querySelectorAll("[data-edit]").forEach((b) => b.addEventListener("click", () => open(ctx.ledger.goals.find((g) => g.id === b.dataset.edit), b)));
  const wanted = ctx.ledger.goals.find((g) => g.id === ctx.params.get("editar") && !g.deleted);
  if (wanted) open(wanted);

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const errors = {};
    if (!f.name.value.trim()) errors.name = "Dê um nome à meta.";
    let target = 0;
    try { target = money(f.target); } catch {}
    if (!(target > 0)) errors.target = "Valor inválido. Use o formato 1.234,56.";
    form.querySelectorAll("[data-error]").forEach((el) => {
      el.hidden = !errors[el.dataset.error];
      el.textContent = errors[el.dataset.error] || "";
      f[el.dataset.error]?.setAttribute("aria-invalid", errors[el.dataset.error] ? "true" : "false");
    });
    if (Object.keys(errors).length) return;
    try {
      await ctx.save("goal", {
        ...(editing ? { id: editing.id } : { startDate: today() }),
        name: f.name.value.trim(), targetCents: target, savedCents: money(f.saved),
        deadline: f.deadline.value || undefined, categoryId: f.category.value || undefined,
      }, "Meta salva.");
      if (ctx.params.get("editar")) ctx.go("/metas");
    } catch (err) {
      ctx.error(err);
    }
  });

  form.querySelector('[data-action="delete-editing"]').addEventListener("click", async () => {
    if (!editing) return;
    const g = editing;
    closeModal(modal);
    await ctx.remove("goal", g);
    toast(`Meta "${g.name}" apagada.`, () => ctx.save("goal", { ...g, deleted: false }, "Meta de volta."));
  });

  // Guardar rápido: a mesma ação do consultor ("guardei 500 na viagem").
  root.querySelectorAll("[data-quick]").forEach((quick) => quick.addEventListener("submit", async (e) => {
    e.preventDefault();
    const g = ctx.ledger.goals.find((x) => x.id === quick.dataset.quick);
    try {
      const cents = money(quick.elements.amount);
      const { type, record } = finance.contribution(g, cents, today());
      await ctx.save(type, record);
      toast(`${m(cents)} guardados em ${g.name}.`);
    } catch (err) {
      ctx.error(err);
    }
  }));
}
