// Salva uma resposta do consultor como nota (o usuário escolhe o que guardar).
import * as db from "./db.js";
import * as finance from "./finance.js";
import * as sync from "./sync.js";
import { newId } from "./ui.js";

export async function saveNote(question, answer) {
  await finance.load();
  const title = "Consultor: " + (question || "resposta").replace(/\s+/g, " ").trim();
  const record = finance.validate("note", {
    id: newId(), updatedAt: Date.now(),
    title: title.length > 110 ? title.slice(0, 109) + "…" : title,
    body: answer,
  });
  await db.saveLocal("note", record);
  sync.soon();
}

// Lança um lançamento proposto pelo consultor, depois que a pessoa aprova no
// botão. Passa pelas mesmas regras (WebAssembly) de um lançamento digitado.
export async function saveProposal(proposal) {
  await finance.load();
  const { categoryName, ...tx } = proposal;
  const record = finance.validate("transaction", { ...tx, updatedAt: Date.now() });
  await db.saveLocal("transaction", record);
  sync.soon();
}

// Texto do cartão: "Almoço · R$ 45,90 · Restaurantes · 16/10/2026".
export function describeProposal(p) {
  const value = (p.amountCents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });
  const date = p.date.split("-").reverse().join("/");
  return `${p.kind === "entrada" ? "Entrada" : "Gasto"}: ${p.description} · ${value} · ${p.categoryName} · ${date}`;
}
