// Sincronização pelo Google Drive da pessoa (regras em internal/drivesync): um
// arquivo, finan_rich.json, na pasta de dados do app (appDataFolder, que só o
// finan_rich enxerga), com todos os registros. Baixa quando o arquivo muda,
// aplica o que for mais novo e regrava o arquivo se este aparelho tiver algo
// que falta lá. Duas gravações ao mesmo tempo: a que perdeu regrava na próxima
// vez, porque o dado continua no aparelho dela.
import { allRecords, applyRemote, getMeta, setMeta } from "./db.js";
import * as finance from "./finance.js";
import * as google from "./google.js";

const FILE = "finan_rich.json";
const API = "https://www.googleapis.com/drive/v3/files";
const UPLOAD = "https://www.googleapis.com/upload/drive/v3/files";

async function call(url, init = {}) {
  const res = await fetch(url, { ...init, headers: { ...init.headers, Authorization: `Bearer ${await google.token()}` } });
  if (res.status === 401) google.expire(); // o próximo pedido renova o token
  if (!res.ok) throw Object.assign(new Error(`o Google Drive respondeu ${res.status}; tente de novo`), { kind: "error" });
  return res;
}

// find devolve { id, version } do arquivo, ou null se ainda não existe.
async function find() {
  const q = new URLSearchParams({ spaces: "appDataFolder", q: `name = '${FILE}'`, fields: "files(id,version)", pageSize: "10" });
  const { files } = await (await call(`${API}?${q}`)).json();
  return files?.[0] || null;
}

async function write(id, content) {
  const body = JSON.stringify(content);
  if (id) {
    return (await call(`${UPLOAD}/${id}?uploadType=media&fields=id,version,modifiedTime`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body })).json();
  }
  const boundary = "finanrich" + Math.random().toString(36).slice(2);
  const meta = JSON.stringify({ name: FILE, parents: ["appDataFolder"] });
  const multipart = `--${boundary}\r\nContent-Type: application/json; charset=UTF-8\r\n\r\n${meta}\r\n` +
    `--${boundary}\r\nContent-Type: application/json\r\n\r\n${body}\r\n--${boundary}--`;
  return (await call(`${UPLOAD}?uploadType=multipart&fields=id,version,modifiedTime`, { method: "POST", headers: { "Content-Type": `multipart/related; boundary=${boundary}` }, body: multipart })).json();
}

const newest = (items) => items.reduce((max, it) => Math.max(max, it.record.updatedAt || 0), 0);

// syncDrive faz uma rodada e diz se chegaram dados novos de outro aparelho.
// localChanged: houve alteração neste aparelho desde a última rodada.
// onClock(horaDoGoogle, enviadoEm): ao gravar, o Drive diz a hora dele
// (modifiedTime), e o aparelho acerta o carimbo das edições por ela.
export async function syncDrive(localChanged, onClock) {
  await finance.load();
  const file = await find();
  const last = (await getMeta("drive", null)) || {};
  let remote = null;
  let changed = false;
  if (file && file.version !== last.version) {
    remote = (await (await call(`${API}/${file.id}?alt=media`)).json()).items || [];
    changed = (await applyRemote(remote)) > 0;
  }
  const local = await allRecords();
  const upload = remote ? finance.driveNeedsUpload(local, remote)
    : !file ? local.length > 0
    : localChanged || newest(local) > (last.at || 0);
  let version = file?.version;
  if (upload) {
    const sentAt = Date.now();
    const saved = await write(file?.id, { version: 1, items: local });
    version = saved.version;
    const at = Date.parse(saved.modifiedTime || "");
    if (at) onClock?.(at, sentAt);
  }
  await setMeta("drive", { version, at: newest(local) });
  return changed;
}
