// Sincronização com o computador onde o finan_rich roda.
//
// Envia a fila de alterações feitas neste aparelho e recebe o que mudou lá
// (inclusive o que outros aparelhos enviaram). Roda ao abrir, quando a conexão
// volta, a cada 30 segundos e logo depois de cada alteração local. Sem
// conexão, nada se perde: a fila fica guardada até dar certo.

import { applyRemote, clearSent, getMeta, pending, setMeta } from "./db.js";
import { endpoint } from "./device.js";

const PERIOD_MS = 30_000;
let running = null;
let again = false;
let timer = null;

// status: "ok" | "syncing" | "offline" | "login" | "unpaired" | "error"
export const state = { status: "syncing", pending: 0, lastSync: null, message: "" };

function emit(dataChanged = false) {
  window.dispatchEvent(new CustomEvent("finan-sync", { detail: { ...state, dataChanged } }));
}

// Tipos que o computador ainda não conhece (finan_rich desatualizado lá, ex.:
// notas antes da v1.3). Ficam na fila, sem travar o resto da sincronização.
const unsupported = new Set();

async function round() {
  const out = await pending();
  state.pending = out.length;
  let cursor = await getMeta("cursor", 0);
  let changed = false;
  let toSend = out.filter((c) => !unsupported.has(c.type));
  const api = await endpoint("/api/sync");
  if (!api) throw Object.assign(new Error("unpaired"), { kind: "unpaired" });
  for (;;) {
    const res = await fetch(api.url, {
      ...api.init,
      method: "POST",
      headers: { ...api.init.headers, "Content-Type": "application/json" },
      body: JSON.stringify({ cursor, changes: toSend.map(({ type, record }) => ({ type, record })) }),
    });
    if (res.status === 401) throw Object.assign(new Error("login"), { kind: "login" });
    const body = await res.json().catch(() => ({}));
    const unknownType = res.status === 422 && /tipo "(\w+)"/.exec(body.error || "")?.[1];
    if (unknownType && !unsupported.has(unknownType) && toSend.some((c) => c.type === unknownType)) {
      unsupported.add(unknownType);
      toSend = toSend.filter((c) => c.type !== unknownType);
      continue;
    }
    if (!res.ok) throw Object.assign(new Error(body.error || res.statusText), { kind: "error" });
    if (toSend.length) {
      await clearSent(toSend);
      toSend = [];
    }
    if (await applyRemote(body.changes || [])) changed = true;
    cursor = body.cursor;
    await setMeta("cursor", cursor);
    if (!body.more) break;
  }
  state.pending = (await pending()).length;
  state.message = unsupported.size ? "Atualize o finan_rich do computador para sincronizar as notas." : "";
  return changed;
}

// sync roda uma sincronização (ou agenda outra, se já houver uma rodando).
export async function sync() {
  if (running) {
    again = true;
    return running;
  }
  running = (async () => {
    state.status = "syncing";
    emit();
    try {
      do {
        again = false;
        const changed = await round();
        state.status = "ok";
        state.lastSync = new Date();
        emit(changed);
      } while (again);
    } catch (err) {
      state.pending = (await pending()).length;
      if (err.kind === "login" || err.kind === "unpaired") state.status = err.kind;
      else if (err.kind === "error") { state.status = "error"; state.message = err.message; }
      else state.status = "offline"; // fetch falhou: sem conexão com o computador
      emit();
    } finally {
      running = null;
    }
  })();
  return running;
}

// soon agenda uma sincronização logo depois de uma alteração local.
export function soon() {
  clearTimeout(timer);
  timer = setTimeout(sync, 400);
  pending().then((p) => { state.pending = p.length; emit(); });
}

export function start() {
  sync();
  window.addEventListener("online", sync);
  document.addEventListener("visibilitychange", () => { if (document.visibilityState === "visible") sync(); });
  setInterval(sync, PERIOD_MS);
}
