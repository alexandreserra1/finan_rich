// IA dentro do celular: o modelo roda no próprio aparelho, com o wllama
// (llama.cpp em WebAssembly, MIT). Depois de baixado, nada sai do celular:
// nem a pergunta, nem os números. O modelo (~470 MB) é baixado uma vez,
// sob demanda, e fica fora do APK. O wllama só vai no app (tools/mobilebuild).
//
// As instruções e o retrato financeiro vêm das mesmas regras em Go do
// consultor (advisor.wasm, localPrompt); aqui só se gera o texto. Modelos tão
// pequenos não usam ferramentas: a resposta é só texto, sem cartões.
import * as db from "./db.js";
import { loadAdvisor } from "./cloud-ai.js";
import { today } from "./ui.js";

const VENDOR = "/static/vendor/wllama/";
// Qwen2.5 0.5B Instruct (Apache-2.0), fixado num commit para o arquivo não mudar.
export const MODEL = {
  name: "Qwen2.5 0.5B",
  url: "https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF/resolve/9217f5db79a29953eb74d5343926648285ec7e67/qwen2.5-0.5b-instruct-q4_k_m.gguf",
  mb: 470,
};

let lib = null; // o módulo do wllama
let models = null; // ModelManager: guarda o modelo no armazenamento do app (OPFS)
let engine = null; // Wllama com o modelo carregado na memória

function loadLib() {
  lib ??= import(VENDOR + "index.js").catch((err) => {
    lib = null;
    throw err;
  });
  return lib;
}

async function manager() {
  if (!models) models = new (await loadLib()).ModelManager();
  return models;
}

const cached = async () => (await (await manager()).getModels()).find((m) => m.url === MODEL.url);

// available diz se este app tem a IA dentro do celular (só o app leva o wllama).
export async function available() {
  try {
    await loadLib();
    return true;
  } catch {
    return false;
  }
}

export const config = () => db.getMeta("local-ai", null); // { enabled }
export const setEnabled = (enabled) => db.setMeta("local-ai", { enabled });
export const downloaded = async () => !!(await cached().catch(() => null));

// ready: ligada e com o modelo baixado.
export async function ready() {
  return !!(await config())?.enabled && (await downloaded());
}

// download baixa o modelo; onProgress recebe de 0 a 1.
export async function download(onProgress) {
  await (await manager()).downloadModel(MODEL.url, {
    progressCallback: ({ loaded, total }) => onProgress?.(total ? loaded / total : 0),
  });
}

export async function remove() {
  await engine?.exit().catch(() => {});
  engine = null;
  await (await cached())?.remove();
  await setEnabled(false);
}

async function load() {
  if (engine) return engine;
  const model = await cached();
  if (!model) throw new Error("baixe o modelo em Ajustes → IA dentro do celular");
  const { Wllama } = await loadLib();
  const w = new Wllama({ default: VENDOR + "wllama.wasm" }, { modelManager: await manager(), suppressNativeLog: true });
  // Sem os cabeçalhos COOP/COEP (o app não tem servidor), roda com 1 thread.
  await w.loadModel(model, { n_ctx: 8192, n_threads: 1 });
  engine = w;
  return w;
}

// ask responde à pergunta dentro do celular. onText recebe a resposta até
// ali, a cada pedaço gerado. Devolve { answer, proposals: [] }.
export async function ask(ledger, history, question, onText) {
  await loadAdvisor();
  const req = { ledger, question, today: today(), history: history.map(({ role, content }) => ({ role, content })) };
  const r = JSON.parse(window.finanAdvisor.localPrompt(JSON.stringify(req)));
  if (r.error) throw new Error(r.error);
  const w = await load();
  const stream = await w.createChatCompletion({ messages: r.ok, max_tokens: 512, temperature: 0.3, stream: true });
  let answer = "";
  for await (const chunk of stream) {
    const piece = chunk.choices?.[0]?.delta?.content || "";
    if (!piece) continue;
    answer += piece;
    onText?.(answer);
  }
  answer = answer.trim();
  if (!answer) throw new Error("a IA dentro do celular não respondeu; tente perguntar de outro jeito");
  return { answer, proposals: [] };
}
