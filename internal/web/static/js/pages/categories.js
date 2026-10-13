// Categorias e divisão do salário.
import { BUCKETS, SPENDING, html } from "../ui.js";
import { pageHead } from "./dashboard.js";

const ORDER = { renda: 0, necessidades: 1, desejos: 2, investimentos: 3 };

function currentSplit(ledger) {
  const pref = ledger.prefs.find((p) => p.id === "split" && !p.deleted);
  return pref?.value || { necessidades: 50, desejos: 30, investimentos: 20 };
}

export function render({ ledger }) {
  const split = currentSplit(ledger);
  const cats = ledger.categories.filter((c) => !c.deleted)
    .sort((a, b) => ORDER[a.bucket] - ORDER[b.bucket] || a.name.localeCompare(b.name, "pt-BR"));

  return html`
    ${pageHead("Categorias")}
    <section class="panel">
      <h2>Divisão do salário</h2>
      <p class="muted">Quanto da renda do mês vai para cada grupo. A regra clássica é 50/30/20; ajuste à sua realidade (a soma precisa dar 100%).</p>
      <form data-form="split" class="grid-form">
        ${SPENDING.map((b) => html`<label class="half">${BUCKETS[b]} (%) <input type="number" name="${b}" min="0" max="100" value="${split[b] ?? 0}"></label>`)}
        <button type="submit">Salvar divisão</button>
      </form>
    </section>

    <section class="panel">
      <h2>Nova categoria</h2>
      <form data-form="category" class="grid-form">
        <label>Nome <input name="name" required placeholder="Ex.: Pet"></label>
        <label>Grupo
          <select name="bucket">${Object.entries(BUCKETS).map(([k, label]) => html`<option value="${k}">${label}</option>`)}</select>
        </label>
        <button type="submit">Criar</button>
      </form>
    </section>

    <section class="panel">
      <h2>Categorias</h2>
      <div class="table-wrap"><table class="cards-mobile">
        <thead><tr><th>Nome</th><th>Grupo</th><th></th></tr></thead>
        <tbody>
          ${cats.map((c) => html`<tr>
            <td class="primary">${c.name}</td>
            <td data-label="Grupo"><span class="tag b-${c.bucket}">${BUCKETS[c.bucket]}</span></td>
            <td class="num actions"><button class="link" type="button" data-delete="${c.id}">apagar</button></td>
          </tr>`)}
        </tbody>
      </table></div>
    </section>`;
}

export function bind(root, ctx) {
  root.querySelector('[data-form="split"]').addEventListener("submit", async (e) => {
    e.preventDefault();
    const value = Object.fromEntries(SPENDING.map((b) => [b, Number(e.target.elements[b].value)]));
    try {
      await ctx.save("pref", { id: "split", value }, "Divisão do salário atualizada.");
    } catch (err) {
      ctx.error(err);
    }
  });

  root.querySelector('[data-form="category"]').addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = e.target.elements;
    const name = f.name.value.trim();
    if (ctx.ledger.categories.some((c) => !c.deleted && c.name.toLowerCase() === name.toLowerCase())) {
      ctx.error(new Error("já existe uma categoria com esse nome"));
      return;
    }
    try {
      await ctx.save("category", { name, bucket: f.bucket.value }, "Categoria criada.");
    } catch (err) {
      ctx.error(err);
    }
  });

  root.querySelectorAll("[data-delete]").forEach((btn) => btn.addEventListener("click", async () => {
    const c = ctx.ledger.categories.find((x) => x.id === btn.dataset.delete);
    if (ctx.ledger.transactions.some((t) => !t.deleted && t.categoryId === c.id)) {
      ctx.error(new Error("categoria possui lançamentos; apague ou mova os lançamentos antes"));
      return;
    }
    if (!confirm(`Apagar a categoria ${c.name}?`)) return;
    await ctx.remove("category", c, "Categoria apagada.");
  }));
}
