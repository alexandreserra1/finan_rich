// Teste de ponta a ponta das notas, da exportação e do backup no navegador.
//
// Cria notas e lançamentos, baixa planilha, PDF, CSV e backup (gerados no
// aparelho, pelo WebAssembly), apaga tudo e restaura o backup.
// Uso (na pasta e2e): FINANRICH_BIN=../dist/finanrich node export.test.mjs
import { chromium } from "playwright";
import { spawn } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const BIN = process.env.FINANRICH_BIN || "../dist/finanrich";
const PORT = 18092;
const B = `http://127.0.0.1:${PORT}`;
const dataDir = mkdtempSync(join(tmpdir(), "finan-e2e-export-"));
const downloads = mkdtempSync(join(tmpdir(), "finan-e2e-downloads-"));
let failures = 0;
const ok = (cond, msg) => {
  console.log(`${cond ? "OK   " : "FALHA"} ${msg}`);
  if (!cond) failures++;
};

const server = spawn(BIN, ["-data", dataDir, "-no-browser", "-addr", `127.0.0.1:${PORT}`], { stdio: "ignore" });
for (let i = 0; i < 100; i++) {
  try { if ((await fetch(`${B}/healthz`)).ok) break; } catch {}
  await new Promise((r) => setTimeout(r, 100));
}

// download clica no botão e devolve { name, bytes } do arquivo baixado.
async function download(page, selector) {
  const [dl] = await Promise.all([page.waitForEvent("download"), page.click(selector)]);
  const path = join(downloads, dl.suggestedFilename());
  await dl.saveAs(path);
  return { name: dl.suggestedFilename(), bytes: readFileSync(path), path };
}
const waitSynced = (page) => page.evaluate(() => window.finanSync());

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  const page = await browser.newPage({ acceptDownloads: true });
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("dialog", (d) => d.accept());

  // Dados: um lançamento e duas notas.
  await page.goto(`${B}/lancamentos`);
  await page.waitForSelector("form[data-form=transaction]", { timeout: 20000 });
  await page.fill("input[name=description]", "Mercado do mês");
  await page.fill("input[name=amount]", "812,40");
  await page.click("form[data-form=transaction] button[type=submit]");
  await page.waitForSelector("td:has-text('Mercado do mês')");

  await page.click("nav.tabs a[href='/configuracoes']"); // Notas fica em Ajustes
  await page.click("a.link-card[href='/notas']");
  await page.waitForSelector("form[data-form=note]");
  await page.fill("input[name=title]", "Reserva de emergência");
  await page.fill("textarea[name=body]", "Juntar 6 meses de gastos.\nComeçar com R$ 300 por mês.");
  await page.check("input[name=pinned]");
  await page.click("form[data-form=note] button[type=submit]");
  await page.waitForSelector(".note.pinned:has-text('Reserva de emergência')");
  await page.fill("input[name=title]", "Ideias");
  await page.fill("textarea[name=body]", "Cancelar streaming que não uso");
  await page.click("form[data-form=note] button[type=submit]");
  await page.waitForSelector(".note:has-text('Ideias')");
  ok(await page.locator(".note").first().textContent().then((t) => t.includes("Reserva")), "nota fixada fica no topo");
  await waitSynced(page);
  const recs = await (await fetch(`${B}/api/sync`, { method: "POST", headers: { "Content-Type": "application/json" }, body: '{"cursor":0}' })).json();
  ok(recs.changes.some((c) => c.type === "note" && c.record.title === "Reserva de emergência" && c.record.pinned), "nota sincronizou com o computador");

  // Nota em PDF e em texto.
  const notePdf = await download(page, ".note:has-text('Reserva') [data-export=pdf]");
  ok(notePdf.name === "nota_reserva_de_emergencia.pdf" && notePdf.bytes.subarray(0, 5).toString() === "%PDF-", `nota em PDF (${notePdf.name})`);
  const noteTxt = await download(page, ".note:has-text('Reserva') [data-export=txt]");
  ok(noteTxt.bytes.toString().startsWith("# Reserva de emergência"), "nota em texto");

  // Exportar: planilha, PDF e CSV.
  await page.goto(`${B}/exportar`);
  await page.waitForSelector("form[data-form=export]");
  const xlsx = await download(page, "[data-format=xlsx]");
  ok(xlsx.name.endsWith(".xlsx") && xlsx.bytes.subarray(0, 2).toString() === "PK", `planilha do Excel (${xlsx.name}, ${xlsx.bytes.length} bytes)`);
  const pdf = await download(page, "[data-format=pdf]");
  ok(pdf.name.startsWith("finan_rich_relatorio_") && pdf.bytes.subarray(0, 5).toString() === "%PDF-", `relatório em PDF (${pdf.name})`);
  await page.check("input[name=scope][value=all]");
  await page.selectOption("select[name=dataset]", "transactions");
  const csv = await download(page, "[data-format=csv]");
  ok(csv.name === "finan_rich_lancamentos_completo.csv" && /Saída;Mercado do mês;[^;]+;Necessidades;-812,40\r\n/.test(csv.bytes.toString()), `CSV de lançamentos (${csv.name})`);

  // Backup, apagar tudo deste aparelho e do computador, restaurar.
  const backup = await download(page, "[data-action=backup]");
  const content = JSON.parse(backup.bytes.toString());
  ok(content.app === "finan_rich" && content.counts.note === 2 && content.counts.transaction === 1, `backup com tudo (${JSON.stringify(content.counts)})`);

  // Outro aparelho (com o relógio adiantado) muda uma nota e lança algo depois do backup.
  const reserva = recs.changes.find((c) => c.type === "note" && c.record.title === "Reserva de emergência").record;
  const other = [
    { type: "note", record: { ...reserva, body: "Mudado no outro aparelho", updatedAt: Date.now() + 60000 } },
    { type: "transaction", record: { id: "outro-1", updatedAt: Date.now(), kind: "saida", description: "Feira (outro aparelho)", amountCents: 3000, categoryId: recs.changes.find((c) => c.type === "transaction").record.categoryId, date: new Date().toISOString().slice(0, 10) } },
  ];
  const sent = await fetch(`${B}/api/sync`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ cursor: 0, changes: other }) });
  ok(sent.ok, `outro aparelho sincronizou (${sent.status})`);
  await waitSynced(page);

  await page.goto(`${B}/notas`);
  await page.waitForSelector(".note:has-text('Ideias')");
  await page.click(".note:has-text('Ideias') [data-delete]");
  await page.waitForSelector(".note:has-text('Ideias')", { state: "detached" });
  await page.goto(`${B}/exportar`);
  await page.waitForSelector("input[name=restore]", { state: "attached" });
  await page.setInputFiles("input[name=restore]", backup.path);
  await page.waitForSelector(".flash.ok:has-text('Backup restaurado')", { timeout: 10000 });
  await page.goto(`${B}/notas`);
  await page.waitForSelector(".note:has-text('Ideias')", { timeout: 10000 });
  ok(true, "restaurar o backup trouxe de volta a nota apagada");
  await waitSynced(page);
  const after = await (await fetch(`${B}/api/sync`, { method: "POST", headers: { "Content-Type": "application/json" }, body: '{"cursor":0}' })).json();
  ok(after.changes.some((c) => c.type === "note" && c.record.title === "Ideias" && !c.record.deleted), "a restauração chegou ao computador");
  const latest = (type, id) => after.changes.filter((c) => c.type === type && c.record.id === id).at(-1)?.record;
  ok(latest("note", reserva.id)?.body === reserva.body, "conflito: a nota volta como estava no backup, mesmo mudada depois em outro aparelho");
  ok(latest("transaction", "outro-1")?.deleted !== true && latest("transaction", "outro-1")?.description === "Feira (outro aparelho)", "o que o outro aparelho criou depois do backup continua");

  // Banco completo do computador.
  const db = await fetch(`${B}/configuracoes/backup.db`);
  ok(db.ok && Buffer.from(await db.arrayBuffer()).subarray(0, 15).toString() === "SQLite format 3", "backup completo do banco (.db)");

  // Sem conexão, exportar continua funcionando (o arquivo é gerado no aparelho).
  await page.goto(`${B}/exportar`);
  await page.waitForSelector("[data-format=pdf]");
  await page.context().setOffline(true);
  const offlinePdf = await download(page, "[data-format=pdf]");
  ok(offlinePdf.bytes.subarray(0, 5).toString() === "%PDF-", "exportar funciona sem conexão");
  await page.context().setOffline(false);

  ok(errors.length === 0, `sem erros de JavaScript ${JSON.stringify(errors)}`);
} catch (err) {
  ok(false, err.message);
} finally {
  await browser.close();
  server.kill();
  rmSync(dataDir, { recursive: true, force: true });
  rmSync(downloads, { recursive: true, force: true });
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exit(failures ? 1 : 0);
