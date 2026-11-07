// Banco SQLite da web (site e navegador do computador), num Worker. Grava no
// OPFS (os arquivos privados do site no navegador) pelo modo opfs-sahpool: não
// exige cabeçalhos especiais e funciona no Safari. Só uma aba por vez consegue
// abrir o banco. Recebe { id, ops: [{ sql, bind, rows }] } e roda tudo numa
// transação só (ou tudo ou nada); responde { id, out } com as linhas (objetos)
// de cada operação com rows, ou { id, error }. O SQL fica em db.js.
import sqlite3InitModule from "../vendor/sqlite/sqlite3.js";

const ready = sqlite3InitModule().then(async (sqlite3) => {
  const pool = await sqlite3.installOpfsSAHPoolVfs({ name: "finan-rich" });
  return new pool.OpfsSAHPoolDb("/finan_rich.sqlite3");
});

onmessage = async ({ data: { id, ops } }) => {
  try {
    const db = await ready;
    const out = [];
    db.transaction(() => {
      for (const op of ops) out.push(op.rows ? db.selectObjects(op.sql, op.bind) : (db.exec({ sql: op.sql, bind: op.bind }), null));
    });
    postMessage({ id, out });
  } catch (e) {
    postMessage({ id, error: String(e?.message || e) });
  }
};
