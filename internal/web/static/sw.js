// Service worker do finan_rich: faz o app abrir e funcionar sem conexão.
//
// - Telas de dados (Painel, Lançamentos, Dívidas, Categorias): tenta a rede e,
//   sem conexão, usa o "casco" guardado; os dados vêm do banco do aparelho.
// - Scripts, estilos, ícones e as regras em WebAssembly: guardados na instalação.
// - Chat e Ajustes precisam do computador: sem conexão, mostram /offline.
// - A sincronização (/api/sync) e outros POSTs nunca passam pelo cache.
//
// __VERSION__ é trocado pelo servidor por uma impressão digital dos arquivos:
// quando o app é atualizado, o cache antigo é descartado.
const VERSION = "finan-__VERSION__";
const APP_ROUTES = ["/", "/lancamentos", "/dividas", "/categorias", "/notas", "/exportar", "/importar", "/metas", "/analise"];
APP_ROUTES.push("/contas");
const PRECACHE = [
  ...APP_ROUTES, "/offline",
  "/static/app.css", "/static/app.js", "/static/chat.js", "/static/settings.js", "/static/favicon.svg", "/static/img/brand-mark.png", "/static/img/avatar-pantera.png", "/static/img/avatar-urso.png", "/static/img/avatar-raposa.png", "/static/img/avatar-texugo.png", "/static/img/avatar-coruja.png", "/static/vendor/geist/Geist-Variable.woff2",
  "/static/icons/icon-192.png", "/static/finance.wasm",
  "/static/js/main.js", "/static/js/db.js", "/static/js/sync.js", "/static/js/finance.js", "/static/js/ui.js", "/static/js/bell.js",
  "/static/js/device.js", "/static/js/pages/device.js", "/static/js/pages/advisor.js",
  "/static/js/exporter.js", "/static/js/pages/notes.js", "/static/js/pages/export.js", "/static/js/pages/import.js", "/static/js/pages/goals.js", "/static/js/pages/insights.js", "/static/js/pdftext.js", "/static/js/notes-save.js", "/static/js/native.js", "/static/js/qrscan.js", "/static/vendor/jsqr/jsQR.min.js",
  "/static/vendor/pdfjs/pdf.min.js", "/static/vendor/pdfjs/pdf.worker.min.js",
  "/static/js/wasm_exec.js", "/static/js/cloud-ai.js", "/static/js/pages/dashboard.js", "/static/js/pages/transactions.js",
  "/static/js/pages/debts.js", "/static/js/pages/categories.js",
  "/static/js/pages/welcome.js",
  "/static/js/pages/accounts.js",
  "/static/js/pages/start.js",
  "/static/js/pages/landing.js",
  "/static/js/pages/oauth.js",
  "/static/js/google.js",
  "/static/js/gdrive.js",
  "/static/js/lock.js",
  "/static/js/theme.js",
  "/static/js/openfinance.js",
  "/static/js/banknotes.js",
  "/static/js/local-ai.js",
  "/static/js/charts.js",
  "/static/vendor/chartjs/chart.umd.min.js",
];

// Guarda cada arquivo, ignorando respostas redirecionadas (ex.: sessão expirada
// levando à tela de login), para nunca guardar a página errada no lugar do app.
async function precache() {
  const cache = await caches.open(VERSION);
  await Promise.allSettled(PRECACHE.map(async (url) => {
    const res = await fetch(url, { credentials: "same-origin", cache: "no-cache" });
    if (res.ok && !res.redirected) await cache.put(url, res);
  }));
}

self.addEventListener("install", (event) => {
  event.waitUntil(precache().then(() => self.skipWaiting()));
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== VERSION).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  const url = new URL(req.url);
  if (req.method !== "GET" || url.origin !== location.origin) return;

  if (url.pathname.startsWith("/static/")) {
    event.respondWith(caches.match(req, { ignoreSearch: true }).then((hit) => hit || fetch(req)));
    return;
  }

  if (req.mode === "navigate") {
    if (APP_ROUTES.includes(url.pathname)) {
      event.respondWith(
        fetch(req)
          .then((res) => {
            // Atualiza o casco guardado (se a sessão expirou, o servidor redireciona: não guarda).
            if (res.ok && !res.redirected) caches.open(VERSION).then((c) => c.put(url.pathname, res.clone()));
            return res;
          })
          .catch(async () => (await caches.match(url.pathname)) || caches.match("/")),
      );
    } else {
      event.respondWith(fetch(req).catch(() => caches.match("/offline")));
    }
  }
});
