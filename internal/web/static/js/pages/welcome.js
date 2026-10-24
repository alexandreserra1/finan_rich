// Primeiro uso: sem nenhum lançamento, o Painel mostra um passo a passo curto
// em vez de uma tela vazia. Cada passo grava com o mesmo ctx.save (e as mesmas
// regras em Go) das outras telas: salário recorrente, divisão do salário,
// dívida e meta. O passo fica no endereço (?passo=2), então voltar funciona.
import * as finance from "../finance.js";
import { BUCKETS, SPENDING, currentMonth, html, today } from "../ui.js";

// Preferência sincronizada: o guia terminou (ou foi pulado) em algum aparelho.
const DONE = "primeiro-uso";
const STEPS = ["Quanto você ganha", "Divisão do salário", "Dívidas", "Uma meta"];

export function show({ ledger, params }) {
  if (params.get("passo")) return true;
  return !ledger.transactions.some((t) => !t.deleted) && !ledger.recurring.some((r) => !r.deleted) &&
    !ledger.prefs.some((p) => p.id === DONE && !p.deleted);
}

const stepOf = (params) => Math.min(Math.max(Number(params.get("passo")) || 1, 1), STEPS.length);

function splitOf(ledger) {
  return ledger.prefs.find((p) => p.id === "split" && !p.deleted)?.value || { necessidades: 50, desejos: 30, investimentos: 20 };
}

const HINTS = {
  necessidades: "moradia, mercado, contas, transporte",
  desejos: "lazer, restaurantes, compras",
  investimentos: "reserva de emergência e investimentos",
};

function body(step, ledger) {
  switch (step) {
    case 1:
      return html`
        <h2>Quanto você ganha por mês?</h2>
        <p class="muted">Vira um lançamento de salário que entra sozinho todo mês, no dia do pagamento. Dá para mudar depois em Lançamentos.</p>
        <form data-form="welcome" class="grid-form">
          <label class="half">Salário líquido (R$) <input name="amount" required inputmode="decimal" placeholder="3.500,00" autofocus></label>
          <label class="half">Dia do pagamento <input type="number" name="day" min="1" max="31" value="5" required></label>
          <button type="submit">Continuar</button>
        </form>`;
    case 2: {
      const split = splitOf(ledger);
      return html`
        <h2>Como dividir o salário?</h2>
        <p class="muted">A regra 50/30/20 é um bom começo: metade para o essencial, 30% para o que você gosta e 20% para guardar. Ajuste se a sua realidade for outra (a soma precisa dar 100%).</p>
        <form data-form="welcome" class="grid-form">
          ${SPENDING.map((b) => html`<label class="half">${BUCKETS[b]} (%) <input type="number" name="${b}" min="0" max="100" value="${split[b] ?? 0}" required>
            <small class="muted">${HINTS[b]}</small></label>`)}
          <button type="submit">Continuar</button>
        </form>`;
    }
    case 3:
      return html`
        <h2>Tem alguma dívida?</h2>
        <p class="muted">Cartão, empréstimo, cheque especial… Cadastrando, o app mostra quanto de juros ela gera e qual pagar primeiro. Tem mais de uma? Cadastre a maior agora e as outras em Dívidas.</p>
        <form data-form="welcome" class="grid-form">
          <label>Nome <input name="name" required placeholder="Ex.: Cartão de crédito"></label>
          <label class="half">Quanto deve (R$) <input name="balance" required inputmode="decimal"></label>
          <label class="half">Juros ao mês (%) <input name="rate" inputmode="decimal" placeholder="Ex.: 12,5"></label>
          <label class="half">Parcela mínima (R$) <input name="min_payment" inputmode="decimal"></label>
          <label class="half">Dia do vencimento <input type="number" name="due_day" min="1" max="31"></label>
          <button type="submit">Cadastrar dívida</button>
        </form>
        <p><button type="button" class="secondary" data-action="next">Não tenho dívidas</button></p>`;
    default:
      return html`
        <h2>Uma meta para começar</h2>
        <p class="muted">Se não sabe por onde começar, a reserva de emergência (3 a 6 meses dos seus gastos) é a primeira meta de quase todo mundo.</p>
        <form data-form="welcome" class="grid-form">
          <label class="wide">Nome <input name="name" required maxlength="80" value="Reserva de emergência"></label>
          <label class="half">Quanto quer juntar (R$) <input name="target" required inputmode="decimal" placeholder="10.000,00"></label>
          <label class="half">Prazo <input type="month" name="deadline"></label>
          <button type="submit">Criar meta e terminar</button>
        </form>`;
  }
}

const STEP_HINTS = ["Vira um salário que entra sozinho todo mês.", "50/30/20 ou a sua: essencial, desejos e guardar.",
  "Mostra os juros e qual pagar primeiro.", "A reserva de emergência é o começo de quase todo mundo."];

export function render({ ledger, params }) {
  // Sem passo escolhido: o estado vazio do Painel, com o começo do guia.
  if (!params.get("passo")) {
    return html`
      <div class="page-head"><div class="title"><span class="kicker">Visão do mês</span><h1>Painel</h1></div></div>
      <section class="welcome welcome-empty">
        <div class="card hero first-use">
          <span class="hero-label caps">Primeiro uso</span>
          <div class="first-use-text">
            <h2>Seu painel começa pelo salário.</h2>
            <p>Quatro passos, uns 2 minutos. Depois o mês se monta sozinho: entradas, saídas e quanto cabe em cada grupo.</p>
          </div>
          <div class="first-use-actions">
            <a class="btn" href="/?passo=1">Começar agora</a>
            <button type="button" class="link plain" data-action="finish">Pular e ir para o Painel</button>
          </div>
        </div>
        <ol class="panel step-list">
          ${STEPS.map((title, i) => html`<li><span class="step-n">${i + 1}</span><span><strong>${title}</strong><span class="muted small">${STEP_HINTS[i]}</span></span></li>`)}
        </ol>
      </section>`;
  }
  const step = stepOf(params);
  return html`
    <div class="page-head"><div class="title"><span class="kicker">Primeiro uso · passo ${step} de ${STEPS.length}</span><h1>Bem-vindo ao finan_rich</h1></div></div>
    <section class="panel welcome">
      <ol class="steps" aria-label="Passos">
        ${STEPS.map((s, i) => html`<li class="${i + 1 < step ? "done" : ""}" ${i + 1 === step ? html`aria-current="step"` : ""}>${s}</li>`)}
      </ol>
      <p class="small muted">Passo ${step} de ${STEPS.length} · leva uns 2 minutos</p>
      ${body(step, ledger)}
      <div class="row welcome-skip">
        ${step < STEPS.length ? html`<button type="button" class="link plain" data-action="next">Pular este passo</button>` : ""}
        <button type="button" class="link plain" data-action="finish">${step < STEPS.length ? "Pular tudo" : "Pular"} e ir para o Painel</button>
      </div>
    </section>`;
}

export function bind(root, ctx) {
  const step = stepOf(ctx.params);
  const next = () => ctx.go(`/?passo=${step + 1}`);
  const finish = async (message) => {
    await ctx.save("pref", { id: DONE, value: true }, null, { rerender: false });
    ctx.go("/");
    if (message) ctx.flash(message);
  };
  root.querySelectorAll('[data-action="next"]').forEach((b) => b.addEventListener("click", next));
  root.querySelectorAll('[data-action="finish"]').forEach((b) => b.addEventListener("click", () => finish().catch((err) => ctx.error(err))));

  const form = root.querySelector('[data-form="welcome"]');
  if (!form) return; // estado vazio: só o "Começar agora" e o "Pular"
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = form.elements;
    const money = (input, label) => {
      if (!input.value.trim()) return 0;
      try { return finance.parseMoney(input.value); } catch (err) { throw new Error(`${label}: ${err.message}`); }
    };
    const quiet = { rerender: false };
    try {
      if (step === 1) {
        const cats = ctx.ledger.categories.filter((c) => !c.deleted && c.bucket === "renda");
        const salary = cats.find((c) => /sal[aá]rio/i.test(c.name)) || cats[0];
        if (!salary) throw new Error("crie uma categoria de renda em Categorias antes");
        await ctx.save("recurring", {
          kind: "entrada", description: "Salário", amountCents: money(f.amount, "salário"),
          categoryId: salary.id, day: Number(f.day.value), start: currentMonth(),
        }, null, quiet);
        next();
      } else if (step === 2) {
        const value = Object.fromEntries(SPENDING.map((b) => [b, Number(f[b].value)]));
        await ctx.save("pref", { id: "split", value }, null, quiet);
        next();
      } else if (step === 3) {
        await ctx.save("debt", {
          name: f.name.value.trim(), creditor: "", balanceCents: money(f.balance, "quanto deve"),
          monthlyRateBp: money(f.rate, "juros"), // "12,5" → 1250 pontos-base, como na tela de Dívidas
          minPaymentCents: money(f.min_payment, "parcela"), dueDay: Number(f.due_day.value) || 0,
        }, null, quiet);
        next();
      } else {
        await ctx.save("goal", {
          name: f.name.value.trim(), targetCents: money(f.target, "valor da meta"), savedCents: 0,
          deadline: f.deadline.value || undefined, startDate: today(),
        }, null, quiet);
        await finish("Pronto! Seu painel está montado. Agora é só lançar os gastos do dia a dia.");
      }
    } catch (err) {
      ctx.error(err);
    }
  });
}
