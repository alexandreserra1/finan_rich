// watchErrors junta em `errors` os erros de JavaScript da página, menos um caso:
// o pedido que a própria troca de página cancelou. O WebKit do Linux (o do CI)
// acusa esse cancelamento como erro da página ("Fetch API cannot load <url> due
// to access control checks"), mesmo com o app tratando o erro, porque a página
// é desmontada antes do tratamento rodar. Só é descartado quando <url> é do
// mesmo endereço (origem) da página: ali não existe checagem de acesso de
// verdade, que só vale entre endereços diferentes. O aviso de "pedido cancelado"
// do Playwright não serve de prova: às vezes não chega (CI de 05/11/2026).
// Qualquer outro erro continua valendo. format monta o texto guardado.
const CANCELLED = /cannot load (\S+) due to access control checks/;

const origin = (url) => {
  try {
    return new URL(url).origin;
  } catch {
    return "";
  }
};

export function watchErrors(page, errors, format = (e) => e.message) {
  page.on("pageerror", (e) => {
    const url = CANCELLED.exec(e.message)?.[1];
    const here = origin(page.url());
    if (url && here && here !== "null" && origin(url) === here) return;
    errors.push(format(e));
  });
}
