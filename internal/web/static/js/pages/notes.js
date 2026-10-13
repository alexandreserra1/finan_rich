// Notas: anotações livres (metas, combinados, lembretes). Ficam no aparelho,
// sincronizam como o resto e o consultor de IA também lê.
import { exportNote } from "../exporter.js";
import { html } from "../ui.js";
import { pageHead } from "./dashboard.js";

const fmtDate = (ms) => new Date(ms).toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" });

const PIN = html`<svg class="icon pin" viewBox="0 0 24 24" aria-label="fixada"><path d="M12 17v5M9 3h6l-1 6 4 4H6l4-4z"/></svg>`;

function activeNotes(ledger) {
  return ledger.notes.filter((n) => !n.deleted)
    .sort((a, b) => (Number(!!b.pinned) - Number(!!a.pinned)) || (b.updatedAt - a.updatedAt));
}

export function render({ ledger, params }) {
  const notes = activeNotes(ledger);
  const editing = notes.find((n) => n.id === params.get("editar"));
  const e = editing || {};
  const q = (params.get("busca") || "").trim();
  const match = (n) => !q || `${n.title}\n${n.body}`.toLocaleLowerCase("pt-BR").includes(q.toLocaleLowerCase("pt-BR"));
  const shown = notes.filter(match);

  return html`
    ${pageHead("Notas")}
    <section class="panel">
      <h2>${editing ? "Editar nota" : "Nova nota"}</h2>
      <form data-form="note" class="note-form">
        <label>Título <input name="title" maxlength="120" value="${e.title || ""}" placeholder="Ex.: Meta da reserva de emergência"></label>
        <label>Texto
          <textarea name="body" rows="${editing ? 10 : 4}" maxlength="20000" placeholder="Escreva o que quiser: metas, combinados, ideias para cortar gastos…">${e.body || ""}</textarea>
        </label>
        <label class="check"><input type="checkbox" name="pinned" ${e.pinned ? "checked" : ""}> Fixar no topo</label>
        <div class="form-actions">
          <button type="submit">Salvar nota</button>
          ${editing ? html`<a href="/notas">Cancelar</a>` : ""}
        </div>
      </form>
    </section>

    <section class="panel">
      <div class="panel-head">
        <h2>Suas notas</h2>
        ${notes.length > 3 ? html`<form data-form="search" class="search" role="search">
          <input type="search" name="busca" value="${q}" placeholder="Buscar nas notas" aria-label="Buscar nas notas">
        </form>` : ""}
      </div>
      ${shown.length ? html`<ul class="notes">
        ${shown.map((n) => html`<li class="note${n.pinned ? " pinned" : ""}">
          <div class="note-head">
            <strong>${n.pinned ? PIN : ""}${n.title || "Sem título"}</strong>
            <small class="muted">${fmtDate(n.updatedAt)}</small>
          </div>
          ${n.body ? html`<p class="note-body">${n.body}</p>` : ""}
          <div class="note-actions">
            <a href="/notas?editar=${n.id}">editar</a>
            <button type="button" class="link plain" data-export="pdf" data-id="${n.id}">PDF</button>
            <button type="button" class="link plain" data-export="txt" data-id="${n.id}">texto</button>
            <button type="button" class="link" data-delete="${n.id}">apagar</button>
          </div>
        </li>`)}
      </ul>`
      : html`<p class="empty">${q ? "Nenhuma nota com esse texto." : "Nenhuma nota ainda. Que tal anotar sua primeira meta?"}</p>`}
    </section>
    <p class="small muted">O consultor de IA lê suas notas mais recentes para entender seus planos. Para levar tudo para fora do app, use <a href="/exportar">Exportar e backup</a>.</p>`;
}

export function bind(root, ctx) {
  const form = root.querySelector('[data-form="note"]');
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    const f = form.elements;
    const editing = ctx.ledger.notes.find((n) => n.id === ctx.params.get("editar"));
    try {
      await ctx.save("note", {
        ...(editing ? { id: editing.id } : {}),
        title: f.title.value.trim(), body: f.body.value.replace(/\s+$/, ""), pinned: f.pinned.checked,
      }, "Nota salva.");
      if (editing) ctx.go("/notas");
    } catch (err) {
      ctx.error(err);
    }
  });

  root.querySelector('[data-form="search"]')?.addEventListener("submit", (e) => {
    e.preventDefault();
    const q = e.target.elements.busca.value.trim();
    ctx.go(q ? `/notas?busca=${encodeURIComponent(q)}` : "/notas");
  });

  root.querySelectorAll("[data-export]").forEach((btn) => btn.addEventListener("click", async () => {
    try {
      await exportNote(ctx.ledger, btn.dataset.id, btn.dataset.export);
    } catch (err) {
      ctx.error(err);
    }
  }));

  root.querySelectorAll("[data-delete]").forEach((btn) => btn.addEventListener("click", async () => {
    const n = ctx.ledger.notes.find((x) => x.id === btn.dataset.delete);
    if (!confirm(`Apagar a nota "${n.title || "Sem título"}"?`)) return;
    await ctx.remove("note", n, "Nota apagada.");
  }));
}
