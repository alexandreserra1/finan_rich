// Sincronização com o computador onde o finan_rich roda.
//
// Envia a fila de alterações feitas neste aparelho e recebe o que mudou lá
// (inclusive o que outros aparelhos enviaram). Roda ao abrir, quando a conexão
// volta, a cada 30 segundos e logo depois de cada alteração local. Sem
// conexão, nada se perde: a fila fica guardada até dar certo.

import { applyRemote, clearSent, getMeta, pending, setMeta } from "./db.js";

const PERIOD_MS = 30_000;
let running = null;
let again = false;
let timer = null;

// status: "ok" | "syncing" | "offline" | "login" | "error"
export const state = { status: "syncing", pending: 0, lastSync: null, message: "" };

function emit(dataChanged = false) {
  window.dispatchEvent(new CustomEvent("finan-sync", { detail: { ...state, dataChanged } }));
}

async function round() {
  const out = await pending();
  state.pending = out.length;
  let cursor = await getMeta("cursor", 0);
  let changed = false;
  let toSend = out;
  for (;;) {
    const res = await fetch("/api/sync", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "same-origin",
      body: JSON.stringify({ cursor, changes: toSend.map(({ type, record }) => ({ type, record })) }),
    });
    if (res.status === 401) throw Object.assign(new Error("login"), { kind: "login" });
    const body = await res.json().catch(() => ({}));
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
        state.message = "";
        emit(changed);
      } while (again);
    } catch (err) {
      state.pending = (await pending()).length;
      if (err.kind === "login") state.status = "login";
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
