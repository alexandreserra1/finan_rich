// Open Finance pelo Meu Pluggy: os lançamentos vêm direto do banco, com as
// credenciais da própria pessoa. No app com o Open Finance configurado a busca
// roda aqui (advisor.wasm, internal/pluggy) e o Client Secret fica no cofre do
// sistema; senão quem busca é o computador (/api/openfinance, Ajustes de lá).
import * as db from "./db.js";
import * as finance from "./finance.js";
import { endpoint, isApp, isSite } from "./device.js";
import { loadAdvisor } from "./cloud-ai.js";
import { shiftMonth, today } from "./ui.js";

const SECRET = "pluggy-secret";
const vault = () => window.Capacitor?.Plugins?.NativeBiometric;

// config: { clientId, itemIds } (o segredo nunca fica aqui).
export const config = () => db.getMeta("pluggy", null);

export async function save({ clientId, clientSecret, itemIds }) {
  const old = await config();
  if (clientSecret) {
    if (!vault()) throw new Error("este aparelho não tem cofre do sistema para guardar o Client Secret");
    await vault().setData({ key: SECRET, value: clientSecret.trim() });
  } else if (!old) {
    throw new Error("cole o Client Secret");
  }
  await db.setMeta("pluggy", { clientId: clientId.trim(), itemIds });
}

export async function forget() {
  await vault()?.deleteData({ key: SECRET }).catch(() => {});
  await db.setMeta("pluggy", null);
}

// fetchAccounts devolve [{ name, statement }] desde from (AAAA-MM-DD). auto: a
// busca automática, que aceita o que o computador buscou em segundo plano.
export async function fetchAccounts(from, auto = false) {
  const cfg = isApp ? await config() : null;
  if (!cfg) {
    const api = await endpoint("/api/openfinance");
    if (!api) throw new Error("configure o Open Finance em Ajustes");
    const res = await fetch(api.url, { ...api.init, method: "POST", headers: { ...api.init.headers, "Content-Type": "application/json" }, body: JSON.stringify({ from, auto }) });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) throw new Error(body.error || `o computador respondeu ${res.status}`);
    return body;
  }
  let clientSecret;
  try {
    clientSecret = (await vault().getData({ key: SECRET })).value;
  } catch {
    throw new Error("não consegui ler o Client Secret guardado; cole de novo em Ajustes");
  }
  await loadAdvisor();
  const r = JSON.parse(await window.finanAdvisor.openFinance(JSON.stringify({ credentials: { clientId: cfg.clientId, clientSecret, itemIds: cfg.itemIds }, from })));
  if (r.error) throw new Error(r.error);
  return r.ok;
}

// Busca sozinha: com a Rich aberta, busca no banco e guarda só os IDs dos
// lançamentos novos; o sininho avisa (finance.PendingAlerts) e a pessoa revisa
// como sempre. Nada entra sozinho. Os já revisados (importados ou deixados de
// fora) não voltam a contar como novos neste aparelho. O app com Open Finance
// próprio vai à Pluggy a cada 6 horas; os demais perguntam ao computador a cada
// meia hora, e ele responde com o que buscou em segundo plano (web/openfinance.go).
const AUTO = "pluggy-auto"; // { at: última busca que deu certo, ids: novos, seen: já revisados }
const EVERY = 6 * 3600_000; // a Pluggy atualiza com o banco algumas vezes por dia
const ASK = 20 * 60_000; // perguntar ao computador não vai à Pluggy; abaixo da meia hora do main.js, para perguntar a cada volta
let auto = { at: 0, ids: [], seen: [] };
let running = null;

export const defaultFrom = () => `${shiftMonth(today().slice(0, 7), -1)}-01`;
export const pending = () => auto.ids;

// autoFetch busca se já passou o prazo e há Open Finance para usar (no app ou no
// computador; o site não tem). Sem configurar ou sem conexão, tenta de novo na
// próxima chamada.
export function autoFetch(ledger) {
  running ??= (async () => {
    if (isSite) return;
    auto = { ...auto, ...(await db.getMeta(AUTO, {})) };
    const own = isApp && !!(await config());
    const age = Date.now() - auto.at; // negativo: o relógio voltou; busca de novo em vez de esperar
    if ((age >= 0 && age < (own ? EVERY : ASK)) || !(own || (await endpoint("/api/openfinance")))) return;
    const accounts = await fetchAccounts(defaultFrom(), true);
    const seen = new Set(auto.seen);
    auto.ids = accounts.flatMap((a) => finance.importPreviewStatement(ledger, a.statement, {}).items)
      .filter((it) => it.selected && !seen.has(it.id)).map((it) => it.id);
    auto.at = Date.now();
    await db.setMeta(AUTO, auto);
  })().finally(() => { running = null; });
  return running;
}

// reviewed: a pessoa revisou estes lançamentos; os que ficaram de fora não avisam mais.
export async function reviewed(ids) {
  const done = new Set(ids);
  auto = { ...auto, ...(await db.getMeta(AUTO, {})) };
  // shortcut: guarda os 2.000 últimos revisados (a busca cobre uns dois meses), aumentar se alguém passar disso no período
  auto.seen = [...new Set([...auto.seen, ...ids])].slice(-2000);
  auto.ids = auto.ids.filter((id) => !done.has(id));
  await db.setMeta(AUTO, auto);
}
