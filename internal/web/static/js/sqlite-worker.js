// Banco SQLite da web (site e navegador do computador), num Worker. Grava no
// OPFS (os arquivos privados do site no navegador) pelo modo opfs-sahpool: não
// exige cabeçalhos especiais e funciona no Safari. Só uma aba por vez consegue
// abrir o banco. Recebe { id, ops: [{ sql, bind, rows, guard }] } e roda tudo
// numa transação só (ou tudo ou nada); responde { id, out } com as linhas
// (objetos) de cada operação com rows ou quantas linhas ela mudou, ou
// { id, error, code }. Uma operação guard sem linhas desfaz tudo (code "guard").
// { id, close: true } fecha o banco e solta os arquivos para outra aba. O SQL
// fica em db-sqlite.js.
import sqlite3InitModule from "../vendor/sqlite/sqlite3.js";

const ready = sqlite3InitModule().then(async (sqlite3) => {
  const pool = await sqlite3.installOpfsSAHPoolVfs({ name: "finan-rich" });
  return { pool, db: new pool.OpfsSAHPoolDb("/finan_rich.sqlite3") };
});

onmessage = async ({ data: { id, ops, close } }) => {
  try {
    const { pool, db } = await ready;
    if (close) {
      db.close();
      pool.pauseVfs();
      return postMessage({ id, out: [] });
    }
    const out = [];
    db.transaction(() => {
      for (const op of ops) {
        if (!op.rows) {
          db.exec({ sql: op.sql, bind: op.bind });
          out.push(db.changes());
          continue;
        }
        const rows = db.selectObjects(op.sql, op.bind);
        if (op.guard && !rows.length) throw Object.assign(new Error("guard"), { code: "guard" });
        out.push(rows);
      }
    });
    postMessage({ id, out });
  } catch (e) {
    postMessage({ id, error: String(e?.message || e), code: e?.code === "guard" ? "guard" : undefined });
  }
};
