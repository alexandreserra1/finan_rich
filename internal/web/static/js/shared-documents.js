import { isApp } from "./device.js";
import { html, toString } from "./ui.js";
import { MAX_FILE_BYTES } from "./document-reader.js";

const plugin = () => isApp && window.Capacitor?.getPlatform?.() === "android" ? window.Capacitor?.Plugins?.SharedDocuments : null;
let pending = null;
let requested = null;
let busy = false;
let generation = 0;
let refreshVersion = 0;
let box;
let context;

const locked = () => document.documentElement.classList.contains("app-locked");

function paint() {
  if (!box?.isConnected) return;
  const unavailable = busy || !!document.querySelector("[data-import-progress]");
  box.innerHTML = pending ? toString(html`<section class="notice" aria-label="Documento recebido">
    <strong>Um documento chegou de outro app.</strong>
    <p>Nada foi importado. Abra a revisão para conferir o arquivo. Este envio é temporário.</p>
    <div class="row-actions"><button type="button" data-shared-review ${unavailable ? "disabled" : ""}>Revisar documento recebido</button>
    <button type="button" class="secondary" data-shared-discard ${busy ? "disabled" : ""}>Dispensar documento recebido</button></div>
  </section>`) : "";
  box.querySelector("[data-shared-review]")?.addEventListener("click", () => {
    if (busy || locked() || document.querySelector("[data-import-progress]")) return;
    requested = pending.id;
    context.go("/importar");
  });
  box.querySelector("[data-shared-discard]")?.addEventListener("click", async () => {
    if (busy || locked()) return;
    try { await clear(); } catch { context.error(new Error("Não consegui dispensar o documento. Tente novamente.")); }
  });
}

async function refresh() {
  const version = ++refreshVersion;
  const value = await plugin().peek();
  if (version !== refreshVersion) return;
  pending = typeof value.id === "string" && value.id.length <= 128 ? { id: value.id } : null;
  paint();
}

export async function start() {
  if (!plugin()) return;
  try {
    await plugin().addListener("pending", () => { refresh().catch(() => context?.error(new Error("Não consegui consultar o documento recebido. Compartilhe novamente."))); });
    await refresh();
  } catch { pending = null; }
}

export function mount(root, ctx) {
  if (!plugin()) return;
  context = ctx;
  box = root.querySelector(":scope > [data-shared-document-notice]");
  if (!box) {
    box = document.createElement("div");
    box.dataset.sharedDocumentNotice = "";
    root.prepend(box);
  }
  paint();
}

export function takeRequest() {
  if (locked() || busy) return null;
  const value = requested;
  requested = null;
  return value;
}

export async function readFile(id) {
  if (!plugin() || locked() || busy || pending?.id !== id) throw new Error("O documento não está mais disponível. Compartilhe novamente.");
  busy = true;
  const current = generation;
  let file;
  paint();
  try {
    const value = await plugin().read({ id });
    if (current !== generation || locked()) throw new Error("Leitura interrompida. Desbloqueie o app e compartilhe novamente.");
    if (value.id !== id || typeof value.base64 !== "string" || value.base64.length > 4 * Math.ceil(MAX_FILE_BYTES / 3) || value.base64.length % 4 || !/^[A-Za-z0-9+/]*={0,2}$/.test(value.base64)) throw new Error("O documento recebido está danificado ou ultrapassa 8 MB.");
    const binary = atob(value.base64);
    if (!binary.length || binary.length > MAX_FILE_BYTES) throw new Error("Escolha um documento de até 8 MB.");
    const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));
    const name = typeof value.name === "string" && value.name.trim() ? value.name.slice(0, 120) : "Documento recebido";
    file = new File([bytes], name, { type: "application/octet-stream" });
  } finally {
    busy = false;
    await refresh().catch(() => { pending = null; paint(); });
  }
  if (current !== generation || locked()) throw new Error("Leitura interrompida. Desbloqueie o app e compartilhe novamente.");
  return file;
}

export async function clear() {
  generation++;
  requested = null;
  const id = pending?.id;
  if (plugin() && id) await plugin().discard({ id });
  await (plugin() ? refresh() : Promise.resolve());
}
