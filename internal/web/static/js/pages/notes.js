// Notas: anotações livres (metas, combinados, lembretes). Ficam no aparelho,
// sincronizam como o resto e o consultor de IA também lê.
import { exportNote } from "../exporter.js";
import { closeModal, html, openModal, raw } from "../ui.js";
import { PLUS, pageHead } from "./dashboard.js";

const fmtDate = (ms) => new Date(ms).toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" });

const ICON = {
  pin: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 17v5M9 3h6l-1 6 4 4H6l4-4z"/></svg>'),
  pencil: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20h4L19 9l-4-4L4 16v4zM14 6l4 4"/></svg>'),
  trash: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6"/></svg>'),
  close: raw('<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6L6 18M6 6l12 12"/></svg>'),
};

function activeNotes(ledger) {
  return ledger.notes.filter((n) => !n.deleted)
    .sort((a, b) => (Number(!!b.pinned) - Number(!!a.pinned)) || (b.updatedAt - a.updatedAt));
}

// noteForm: a janela de criar e editar.
function noteForm() {
  return html`<div class="modal" id="note-modal" hidden>
    <div class="modal-scrim" data-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="note-title">
      <form data-form="note" class="modal-form">
        <div class="modal-head"><h2 id="note-title">Nova nota</h2><button type="button" class="icon-btn" data-close aria-label="Fechar">${ICON.close}</button></div>
        <label class="field">Título <input name="title" maxlength="120" placeholder="Ex.: Meta da reserva de emergência" autofocus></label>
        <label class="field">Texto
          <textarea name="body" rows="8" maxlength="20000" placeholder="Escreva o que quiser: metas, combinados, ideias para cortar gastos…"></textarea>
        </label>
        <label class="check"><input type="checkbox" name="pinned"> Fixar no topo</label>
        <div class="modal-foot"><span class="spacer"></span><button type="button" class="secondary" data-close>Cancelar</button><button type="submit">Salvar nota</button></div>
      </form>
    </div>
  </div>`;
}

export function render({ ledger, params }) {
  const notes = activeNotes(ledger);
  const q = (params.get("busca") || "").trim();
  const match = (n) => !q || `${n.title}\n${n.body}`.toLocaleLowerCase("pt-BR").includes(q.toLocaleLowerCase("pt-BR"));
  const shown = notes.filter(match);

  return html`
    ${pageHead("Ideias boas merecem ficar.", null, { sub: "Guarde planos, aprendizados e respostas do Consultor.", actions: html`<button type="button" data-action="new-note">${PLUS}Nova nota</button>` })}
    <form data-form="search" class="note-search" role="search">
      <label class="field">Buscar nas notas <input type="search" name="busca" value="${q}" placeholder="Uma ideia, um plano…"></label>
    </form>
    ${shown.length ? html`<ul class="notes">
      ${shown.map((n) => html`<li class="note${n.pinned ? " pinned" : ""}">
        <div class="note-head">
          <strong>${n.title || "Sem título"}</strong>
          <button type="button" class="icon-btn note-pin" data-pin="${n.id}" aria-pressed="${n.pinned ? "true" : "false"}" aria-label="${n.pinned ? "Soltar do topo" : "Fixar no topo"}">${ICON.pin}</button>
        </div>
        ${n.body ? html`<p class="note-body">${n.body}</p>` : ""}
        <div class="note-foot">
          <small class="faint">${fmtDate(n.updatedAt)}</small>
          <span class="note-actions">
            <button type="button" class="round-btn" data-edit="${n.id}" aria-label="Editar ${n.title || "nota"}">${ICON.pencil}</button>
            <button type="button" class="round-btn txt" data-export="pdf" data-id="${n.id}" aria-label="Baixar em PDF">PDF</button>
            <button type="button" class="round-btn txt" data-export="txt" data-id="${n.id}" aria-label="Baixar em texto">TXT</button>
            <button type="button" class="round-btn danger" data-delete="${n.id}" aria-label="Apagar ${n.title || "nota"}">${ICON.trash}</button>
          </span>
        </div>
      </li>`)}
    </ul>`
    : html`<p class="panel empty">${q ? "Nenhuma nota com esse texto." : "Nenhuma nota ainda. Que tal anotar sua primeira meta?"}</p>`}
    <p class="faint tiny">O consultor lê suas notas mais recentes para entender seus planos. Cada nota sai em PDF ou texto; para levar tudo para fora do app, use <a href="/exportar">Exportar e backup</a>.</p>
    ${noteForm()}`;
}

export function bind(root, ctx) {
  const modal = root.querySelector("#note-modal");
  const form = root.querySelector('[data-form="note"]');
  const f = form.elements;
  let editing = null;
  const open = (n, trigger) => {
    editing = n || null;
    form.reset();
    form.querySelector("#note-title").textContent = n ? "Editar nota" : "Nova nota";
    if (n) {
      f.title.value = n.title || "";
      f.body.value = n.body || "";
      f.pinned.checked = !!n.pinned;
    }
    openModal(modal, trigger);
  };
  root.querySelector('[data-action="new-note"]').addEventListener("click", (e) => open(null, e.currentTarget));
  root.querySelectorAll("[data-edit]").forEach((b) => b.addEventListener("click", () => open(ctx.ledger.notes.find((x) => x.id === b.dataset.edit), b)));

  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    try {
      closeModal(modal);
      await ctx.save("note", {
        ...(editing ? { id: editing.id } : {}),
        title: f.title.value.trim(), body: f.body.value.replace(/\s+$/, ""), pinned: f.pinned.checked,
      }, "Nota salva.");
    } catch (err) {
      ctx.error(err);
    }
  });

  root.querySelector('[data-form="search"]').addEventListener("submit", (e) => {
    e.preventDefault();
    const q = e.target.elements.busca.value.trim();
    ctx.go(q ? `/notas?busca=${encodeURIComponent(q)}` : "/notas");
  });

  root.querySelectorAll("[data-pin]").forEach((btn) => btn.addEventListener("click", async () => {
    const n = ctx.ledger.notes.find((x) => x.id === btn.dataset.pin);
    await ctx.save("note", { id: n.id, pinned: !n.pinned }, n.pinned ? "Nota solta do topo." : "Nota fixada no topo.");
  }));

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
