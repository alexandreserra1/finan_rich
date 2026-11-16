// Sincronização pelo Google Drive da pessoa (regras em internal/drivesync): um
// arquivo, finan_rich.json, na pasta de dados do app (appDataFolder, que só o
// finan_rich enxerga), com todos os registros. Baixa quando o arquivo muda,
// aplica o que for mais novo e regrava o arquivo se este aparelho tiver algo
// que falta lá. Duas gravações ao mesmo tempo: a que perdeu regrava na próxima
// vez, porque o dado continua no aparelho dela.
import { allRecords, applyRemote, checkEpoch, databaseEpoch, getMeta, setMeta } from "./db.js";
import * as finance from "./finance.js";
import * as google from "./google.js";

const FILE = "finan_rich.json";
const API = "https://www.googleapis.com/drive/v3/files";
const UPLOAD = "https://www.googleapis.com/upload/drive/v3/files";

async function call(url, init, epoch) {
  checkEpoch(epoch);
  const token = await google.token();
  checkEpoch(epoch);
  const res = await fetch(url, { ...init, headers: { ...init?.headers, Authorization: `Bearer ${token}` } });
  checkEpoch(epoch);
  if (res.status === 401) google.expire(); // o próximo pedido renova o token
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    checkEpoch(epoch);
    const error = body?.error;
    const reasons = [
      ...(Array.isArray(error?.errors) ? error.errors.map((item) => item?.reason) : []),
      ...(Array.isArray(error?.details) ? error.details.filter((item) => item?.["@type"] === "type.googleapis.com/google.rpc.ErrorInfo").map((item) => item.reason) : []),
    ];
    let message = `o Google Drive respondeu ${res.status}; tente de novo`;
    let recovery = "";
    if (res.status === 403) {
      if (reasons.some((reason) => ["accessNotConfigured", "SERVICE_DISABLED"].includes(reason))) {
        message = "A API do Google Drive não está ativada no projeto do login da Rich. Quem administra o site precisa ativá-la no Google Cloud e tentar novamente.";
      } else if (reasons.some((reason) => ["insufficientPermissions", "ACCESS_TOKEN_SCOPE_INSUFFICIENT"].includes(reason))) {
        message = "A Rich não recebeu permissão para usar a pasta de dados do app no seu Google Drive. Você precisa entrar de novo com Google e permitir esse acesso.";
        recovery = "google";
      } else if (reasons.includes("storageQuotaExceeded")) {
        message = "O armazenamento do seu Google Drive está cheio. Libere espaço e tente sincronizar novamente. Seus dados continuam neste aparelho.";
      } else if (reasons.some((reason) => ["rateLimitExceeded", "userRateLimitExceeded", "dailyLimitExceeded"].includes(reason))) {
        message = "O Google Drive atingiu um limite de uso. Espere um pouco antes de tentar sincronizar novamente. Seus dados continuam neste aparelho.";
      } else {
        // O código do Google (ex.: "forbidden") é só um identificador, sem dado pessoal: mostrá-lo
        // deixa a pessoa dizer a causa sem abrir o Network. Só entra o que tem cara de código.
        const codes = [...new Set(reasons.filter((reason) => typeof reason === "string" && /^[A-Za-z0-9_.-]{1,60}$/.test(reason)))];
        message = `O Google Drive recusou o acesso (403). Confira as permissões da conta e a configuração do projeto antes de tentar novamente.${codes.length ? ` Código do Google: ${codes.join(", ")}.` : ""}`;
      }
    }
    throw Object.assign(new Error(message), { kind: "error", recovery });
  }
  return res;
}

// find devolve { id, version } do arquivo, ou null se ainda não existe.
async function find(epoch) {
  const q = new URLSearchParams({ spaces: "appDataFolder", q: `name = '${FILE}'`, fields: "files(id,version)", pageSize: "10" });
  const response = await call(`${API}?${q}`, {}, epoch);
  checkEpoch(epoch);
  const { files } = await response.json();
  checkEpoch(epoch);
  return files?.[0] || null;
}

async function write(id, content, epoch) {
  checkEpoch(epoch);
  const body = JSON.stringify(content);
  if (id) {
    const response = await call(`${UPLOAD}/${id}?uploadType=media&fields=id,version,modifiedTime`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body }, epoch);
    const saved = await response.json();
    checkEpoch(epoch);
    return saved;
  }
  const boundary = "finanrich" + Math.random().toString(36).slice(2);
  const meta = JSON.stringify({ name: FILE, parents: ["appDataFolder"] });
  const multipart = `--${boundary}\r\nContent-Type: application/json; charset=UTF-8\r\n\r\n${meta}\r\n` +
    `--${boundary}\r\nContent-Type: application/json\r\n\r\n${body}\r\n--${boundary}--`;
  const response = await call(`${UPLOAD}?uploadType=multipart&fields=id,version,modifiedTime`, { method: "POST", headers: { "Content-Type": `multipart/related; boundary=${boundary}` }, body: multipart }, epoch);
  const saved = await response.json();
  checkEpoch(epoch);
  return saved;
}

const newest = (items) => items.reduce((max, it) => Math.max(max, it.record.updatedAt || 0), 0);

// syncDrive faz uma rodada e diz se chegaram dados novos de outro aparelho.
// localChanged: houve alteração neste aparelho desde a última rodada.
// onClock(horaDoGoogle, enviadoEm): ao gravar, o Drive diz a hora dele
// (modifiedTime), e o aparelho acerta o carimbo das edições por ela.
export async function syncDrive(localChanged, onClock, epoch = databaseEpoch()) {
  checkEpoch(epoch);
  await finance.load();
  checkEpoch(epoch);
  const file = await find(epoch);
  checkEpoch(epoch);
  const last = (await getMeta("drive", null, epoch)) || {};
  checkEpoch(epoch);
  let remote = null;
  let changed = false;
  if (file && file.version !== last.version) {
    const response = await call(`${API}/${file.id}?alt=media`, {}, epoch);
    checkEpoch(epoch);
    remote = (await response.json()).items || [];
    checkEpoch(epoch);
    changed = (await applyRemote(remote, epoch)) > 0;
    checkEpoch(epoch);
  }
  const local = await allRecords(epoch);
  checkEpoch(epoch);
  const upload = remote ? finance.driveNeedsUpload(local, remote)
    : !file ? local.length > 0
    : localChanged || newest(local) > (last.at || 0);
  let version = file?.version;
  if (upload) {
    const sentAt = Date.now();
    const saved = await write(file?.id, { version: 1, items: local }, epoch);
    checkEpoch(epoch);
    version = saved.version;
    const at = Date.parse(saved.modifiedTime || "");
    if (at) await onClock?.(at, sentAt);
    checkEpoch(epoch);
  }
  await setMeta("drive", { version, at: newest(local) }, epoch);
  checkEpoch(epoch);
  return changed;
}
