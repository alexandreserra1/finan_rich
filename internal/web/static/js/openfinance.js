// Open Finance pelo Meu Pluggy: os lançamentos vêm direto do banco, com as
// credenciais da própria pessoa. No app com o Open Finance configurado a busca
// roda aqui (advisor.wasm, internal/pluggy) e o Client Secret fica no cofre do
// sistema; senão quem busca é o computador (/api/openfinance, Ajustes de lá).
import * as db from "./db.js";
import { endpoint, isApp } from "./device.js";
import { loadAdvisor } from "./cloud-ai.js";

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

// fetchAccounts devolve [{ name, statement }] desde from (AAAA-MM-DD).
export async function fetchAccounts(from) {
  const cfg = isApp ? await config() : null;
  if (!cfg) {
    const api = await endpoint("/api/openfinance");
    if (!api) throw new Error("configure o Open Finance em Ajustes");
    const res = await fetch(api.url, { ...api.init, method: "POST", headers: { ...api.init.headers, "Content-Type": "application/json" }, body: JSON.stringify({ from }) });
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
