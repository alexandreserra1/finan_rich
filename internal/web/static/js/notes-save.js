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
