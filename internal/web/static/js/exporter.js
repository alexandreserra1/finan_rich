// Leva arquivos para fora do app: planilha, CSV, PDF, notas e backup.
//
// Os arquivos são gerados neste aparelho pelas regras em Go (WebAssembly),
// então exportar funciona sem conexão. No navegador, o arquivo é baixado; no
// app do celular, é salvo e abre o menu de compartilhar (Drive, WhatsApp,
// e-mail, Arquivos...).
import * as finance from "./finance.js";
import { isApp } from "./device.js";

export async function exportData(ledger, options) {
  const file = finance.exportFile(ledger, options);
  await deliver(file);
  return file;
}

export const exportNote = (ledger, id, format) => exportData(ledger, { noteId: id, format });

async function deliver({ name, mime, data }) {
  const plugins = window.Capacitor?.Plugins;
  if (isApp && plugins?.Filesystem && plugins?.Share) {
    // Sem "encoding", o Filesystem entende o texto como base64 e grava os bytes.
    const saved = await plugins.Filesystem.writeFile({ path: name, data, directory: "CACHE" });
    try {
      await plugins.Share.share({ title: name, url: saved.uri, dialogTitle: "Salvar ou enviar" });
    } catch (err) {
      if (!/cancel/i.test(String(err?.message || err))) throw err; // fechar o menu não é erro
    }
    return;
  }
  const bytes = Uint8Array.from(atob(data), (c) => c.charCodeAt(0));
  const url = URL.createObjectURL(new Blob([bytes], { type: mime }));
  const a = Object.assign(document.createElement("a"), { href: url, download: name });
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 30_000);
}
