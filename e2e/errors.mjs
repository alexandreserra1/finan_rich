// watchErrors junta em `errors` os erros de JavaScript da página, menos um caso:
// o pedido que a própria troca de página cancelou. O WebKit do Linux (o do CI)
// acusa esse cancelamento como erro da página ("Fetch API cannot load <url> due
// to access control checks"), mesmo com o app tratando o erro, porque a página
// é desmontada antes do tratamento rodar. Só é descartado quando o navegador
// também marcou o pedido daquele mesmo endereço como cancelado; qualquer outro
// erro continua valendo. format monta o texto guardado (padrão: a mensagem).
const CANCELLED = /cannot load (\S+) due to access control checks/;

export function watchErrors(page, errors, format = (e) => e.message) {
  const cancelled = new Set();
  const pending = []; // erro de cancelamento visto antes do aviso do pedido: { url, text }
  page.on("requestfailed", (r) => {
    if (!/cancel|abort/i.test(r.failure()?.errorText || "")) return;
    cancelled.add(r.url());
    for (const p of pending.filter((p) => p.url === r.url())) {
      const i = errors.indexOf(p.text);
      if (i >= 0) errors.splice(i, 1);
    }
  });
  page.on("pageerror", (e) => {
    const url = CANCELLED.exec(e.message)?.[1];
    if (url && cancelled.has(url)) return;
    const text = format(e);
    if (url) pending.push({ url, text });
    errors.push(text);
  });
}
