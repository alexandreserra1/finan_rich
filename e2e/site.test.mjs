// Teste de ponta a ponta do site: o programa da Vercel (cmd/api compilado com
// -tags sitebundle) servindo as telas do app em modo site e o intermediário da
// IA no mesmo endereço. Sem computador: os dados ficam no navegador.
// Uso (na pasta e2e): SITE_BIN=../dist/finansite node site.test.mjs
import { chromium as chromiumEngine, devices, webkit } from "playwright";
const chromium = process.env.BROWSER === "webkit" ? webkit : chromiumEngine; // BROWSER=webkit: o motor do Safari
import { spawn } from "node:child_process";
import { watchErrors } from "./errors.mjs";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

const BIN = process.env.SITE_BIN || "../dist/finansite";
const OFF = Number(process.env.PORT_OFFSET || 0); // PORT_OFFSET=100: roda ao lado de outra cópia do repositório
const PORT = 18099 + OFF;
const SITE = `http://127.0.0.1:${PORT}`;
let failures = 0;
const ok = (cond, msg) => {
  console.log(`${cond ? "OK   " : "FALHA"} ${msg}`);
  if (!cond) failures++;
};
// Abre a janela de novo lançamento (se ainda não estiver aberta).
const openTx = async (page) => {
  if (await page.locator("#tx-modal .modal-panel").isVisible()) return;
  await page.locator('[data-action="new-tx"]:visible').first().click();
  await page.waitForSelector("#tx-modal .modal-panel");
};

const checkVectorIcons = async (page, selector, label) => {
  const result = await page.locator(selector).evaluate((root) => {
    const emojiProne = /[\u2194-\u2199\u21A9\u21AA\u2600-\u27BF\uFE0F]|\p{Emoji_Presentation}/u;
    const issues = [];
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    while (walker.nextNode()) {
      const node = walker.currentNode;
      if (!node.parentElement.closest("script, style") && emojiProne.test(node.textContent)) {
        issues.push(node.textContent.trim());
      }
    }
    for (const element of [root, ...root.querySelectorAll("*")]) {
      for (const pseudo of ["::before", "::after"]) {
        const content = getComputedStyle(element, pseudo).content;
        if (emojiProne.test(content)) issues.push(`${element.className.baseVal ?? element.className}${pseudo}: ${content}`);
      }
    }
    const brand = root.querySelector(".lp-brand svg, .brand svg");
    let brandContrast = 0;
    if (brand) {
      const channels = (value) => value.match(/[\d.]+/g).slice(0, 3).map(Number);
      const luminance = (color) => channels(color).map((channel) => {
        const value = channel / 255;
        return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
      }).reduce((sum, channel, index) => sum + channel * [0.2126, 0.7152, 0.0722][index], 0);
      let background = brand.parentElement;
      while (background.parentElement && getComputedStyle(background).backgroundColor === "rgba(0, 0, 0, 0)") background = background.parentElement;
      const front = luminance(getComputedStyle(brand).color);
      const back = luminance(getComputedStyle(background).backgroundColor);
      brandContrast = (Math.max(front, back) + 0.05) / (Math.min(front, back) + 0.05);
    }
    return { issues, brandSvg: !!brand, brandContrast };
  });
  ok(result.issues.length === 0 && result.brandSvg && result.brandContrast >= 3,
    `${label}: marca em SVG e nenhum símbolo Unicode sujeito a emoji no texto ou nos pseudo-elementos (${JSON.stringify(result)})`);
};

const checkLandingVisual = async (browser) => {
  const screenshots = await mkdtemp(join(tmpdir(), "rich-landing-ficticia-"));
  console.log(`Screenshots da landing com dados explicitamente fictícios: ${screenshots}`);
  for (const width of [360, 390, 600, 768, 1440, 1920]) {
    for (const colorScheme of ["light", "dark"]) {
      const context = await browser.newContext({ viewport: { width, height: 1000 }, colorScheme });
      const page = await context.newPage();
      const errors = [];
      watchErrors(page, errors);
      try {
        await page.goto(`${SITE}/`);
        await page.waitForSelector(".landing .lp-dashboard");
        await page.evaluate(() => document.fonts.ready);
        for (const photo of await page.locator('.landing img[loading="lazy"]').all()) {
          await photo.scrollIntoViewIfNeeded();
          await photo.evaluate((element) => element.decode());
        }
        ok(await page.getAttribute("html", "data-theme") === colorScheme, `${width}px: tema ${colorScheme} aplicado`);
        ok(await page.locator(".lp-democopy .lp-fine").innerText() === "Demonstração com dados fictícios. Nenhuma conta conectada.",
          `${width}px/${colorScheme}: a demonstração identifica os dados fictícios`);
        if (width <= 600) {
          const mobile = await page.evaluate(() => {
            const convo = document.querySelector(".lp-convo");
            const goal = document.querySelector(".lp-goalfloat").getBoundingClientRect();
            const walker = document.createTreeWalker(convo, NodeFilter.SHOW_TEXT);
            const overlaps = [];
            let textRects = 0;
            while (walker.nextNode()) {
              const node = walker.currentNode;
              if (!node.textContent.trim()) continue;
              const range = document.createRange();
              range.selectNodeContents(node);
              for (const rect of range.getClientRects()) {
                if (!rect.width || !rect.height) continue;
                textRects++;
                if (Math.min(rect.right, goal.right) - Math.max(rect.left, goal.left) > 1 &&
                    Math.min(rect.bottom, goal.bottom) - Math.max(rect.top, goal.top) > 1) overlaps.push(node.textContent.trim());
              }
            }
            const rows = [...document.querySelectorAll(".lp-compare tbody tr")];
            const comparison = rows.map((row) => {
              const cells = [...row.querySelectorAll("td")];
              const stacked = cells.some((cell) => getComputedStyle(cell).display !== "table-cell");
              return {
                heading: !!row.querySelector('th[scope="row"]')?.textContent.trim(),
                cells: cells.length,
                valid: cells.every((cell, index) => {
                  const rect = cell.getBoundingClientRect();
                  if (!cell.textContent.trim() || !rect.width || !rect.height || rect.left < -1 ||
                      rect.right > document.documentElement.clientWidth + 1 || cell.scrollWidth > cell.clientWidth + 1) return false;
                  if (!stacked) return true;
                  const before = getComputedStyle(cell, "::before");
                  const expected = ["Rich", "Outros"][index];
                  return cell.dataset.label === expected && before.content.replace(/^["']|["']$/g, "") === expected &&
                    before.display !== "none" && before.visibility === "visible" && Number(before.opacity) > 0;
                }),
              };
            });
            return { textRects, overlaps, goalVisible: goal.width > 0 && goal.height > 0, comparison };
          });
          ok(mobile.textRects > 0 && mobile.goalVisible && mobile.overlaps.length === 0,
            `${width}px/${colorScheme}: o cartão de meta não cobre o texto da conversa (${JSON.stringify(mobile.overlaps)})`);
          ok(mobile.comparison.length > 0 && mobile.comparison.every((row) => row.heading && row.cells === 2 && row.valid),
            `${width}px/${colorScheme}: comparação legível, sem corte e com labels Rich/Outros em cada célula quando empilhada (${JSON.stringify(mobile.comparison)})`);
        }
        for (const mode of ["gastos", "metas", "resumo"]) {
          const label = `${width}px/${colorScheme}/${mode}`;
          try {
            await page.locator(`#lp-tab-${mode}`).click();
            await page.mouse.move(0, 0);
            await page.waitForFunction((mode) => {
              const panel = document.querySelector(".lp-dashboard");
              return panel.getAttribute("aria-labelledby") === `lp-tab-${mode}` &&
                document.querySelector(`#lp-tab-${mode}`).getAttribute("aria-selected") === "true" &&
                panel.querySelector({ gastos: ".lp-chart", metas: ".lp-goalresult", resumo: ".lp-receipt" }[mode]);
            }, mode);
            await page.evaluate(() => document.fonts.ready);
            await checkVectorIcons(page, ".landing", label);
            await page.waitForFunction(() => {
              const elements = [...document.querySelectorAll(".lp-dashboard, .lp-dashboard *, .lp-walletnote, .lp-walletstage .rich-card, .lp-pill.light, .lp-tab")];
              const moving = elements.some((element) => element.getAnimations().some((animation) =>
                ["running", "pending"].includes(animation.playState) && animation.effect.getComputedTiming().iterations !== Infinity));
              const snapshot = JSON.stringify(elements.map((element) => {
                const rect = element.getBoundingClientRect();
                const style = getComputedStyle(element);
                return [rect.x, rect.y, rect.width, rect.height, style.color, style.backgroundColor, style.transform];
              }));
              const previous = window.__landingVisualStable;
              const frames = !moving && previous?.snapshot === snapshot ? previous.frames + 1 : 0;
              window.__landingVisualStable = { snapshot, frames };
              return frames >= 4;
            }, undefined, { polling: "raf", timeout: 10000 });
            await page.screenshot({ path: join(screenshots, `${width}-${colorScheme}-${mode}.png`), fullPage: true });
            if (mode === "gastos") {
              await page.locator(".lp-demo").screenshot({ path: join(screenshots, `${width}-${colorScheme}-demo.png`) });
              await page.locator(".lp-walletstage").screenshot({ path: join(screenshots, `${width}-${colorScheme}-cartao.png`) });
            }
            const result = await page.evaluate(() => {
              const panel = document.querySelector(".lp-dashboard");
              const card = document.querySelector(".lp-walletstage .rich-card").getBoundingClientRect();
              const note = document.querySelector(".lp-walletnote").getBoundingClientRect();
              const parseColor = (value) => {
                const channels = value.match(/[\d.]+/g)?.map(Number);
                if (!/^rgba?\(/.test(value) || !channels || channels.length < 3) throw new Error(`Cor não suportada: ${value}`);
                return [...channels.slice(0, 3), channels[3] ?? 1];
              };
              const composite = (front, back) => front.slice(0, 3).map((channel, index) => channel * front[3] + back[index] * (1 - front[3]));
              const luminance = (color) => color.map((channel) => {
                const value = channel / 255;
                return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
              }).reduce((sum, channel, index) => sum + channel * [0.2126, 0.7152, 0.0722][index], 0);
              const contrastIssues = [];
              const overflowIssues = [];
              let dashboardTexts = 0;
              let buttonTexts = 0;
              const roots = [panel, ...document.querySelectorAll('.lp-pill.light, .lp-tab[aria-selected="true"]')];
              for (const root of roots) {
                const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
                while (walker.nextNode()) {
                  const node = walker.currentNode;
                  const text = node.textContent.trim();
                  const element = node.parentElement;
                  if (!text || !element.checkVisibility({ visibilityProperty: true, opacityProperty: true })) continue;
                  const range = document.createRange();
                  range.selectNodeContents(node);
                  const rect = range.getBoundingClientRect();
                  if (!rect.width || !rect.height) continue;
                  if (root === panel) dashboardTexts++; else buttonTexts++;
                  if (rect.left < 0 || rect.right > document.documentElement.clientWidth) overflowIssues.push(text);
                  const layers = [];
                  let opacity = 1;
                  for (let ancestor = element; ancestor; ancestor = ancestor.parentElement) {
                    const style = getComputedStyle(ancestor);
                    opacity *= Number(style.opacity);
                    const bounds = ancestor.getBoundingClientRect();
                    const centerX = rect.left + rect.width / 2;
                    const centerY = rect.top + rect.height / 2;
                    if (centerX < bounds.left || centerX > bounds.right || centerY < bounds.top || centerY > bounds.bottom) continue;
                    if (style.backgroundImage !== "none") throw new Error(`Fundo não uniforme em ${text}: ${style.backgroundImage}`);
                    layers.push(parseColor(style.backgroundColor));
                  }
                  const background = layers.reverse().reduce((back, front) => composite(front, back), [255, 255, 255]);
                  const style = getComputedStyle(element);
                  const foreground = parseColor(style.color);
                  foreground[3] *= opacity;
                  const textLight = luminance(composite(foreground, background));
                  const backLight = luminance(background);
                  const ratio = (Math.max(textLight, backLight) + 0.05) / (Math.min(textLight, backLight) + 0.05);
                  const size = parseFloat(style.fontSize);
                  const minimum = size >= 24 || (size >= 18.6667 && Number(style.fontWeight) >= 700) ? 3 : 4.5;
                  if (ratio < minimum) contrastIssues.push({ text, ratio: Number(ratio.toFixed(2)), minimum, color: style.color, background });
                }
              }
              return {
                documentWidth: document.documentElement.scrollWidth,
                viewportWidth: document.documentElement.clientWidth,
                panelWidth: panel.clientWidth, panelScrollWidth: panel.scrollWidth,
                gap: note.top - card.bottom, dashboardTexts, buttonTexts, contrastIssues, overflowIssues,
              };
            });
            ok(result.documentWidth <= result.viewportWidth && result.panelScrollWidth <= result.panelWidth && result.overflowIssues.length === 0,
              `${label}: sem overflow da página, painel ou textos (${JSON.stringify(result)})`);
            ok(result.dashboardTexts > 0 && result.buttonTexts > 0 && result.contrastIssues.length === 0,
              `${label}: textos do painel e botões claros com contraste WCAG AA (${JSON.stringify(result.contrastIssues)})`);
            ok(result.gap >= 20, `${label}: anotação abaixo e fora do cartão, com pelo menos 20px de respiro (${result.gap.toFixed(2)}px)`);
          } catch (err) {
            ok(false, `${label}: ${err.message}`);
          }
        }
        await page.goto(`${SITE}/?entrar=1`);
        await page.waitForSelector(".start [data-unavailable]:not([hidden])");
        await page.evaluate(() => document.fonts.ready);
        await checkVectorIcons(page, ".start", `${width}px/${colorScheme}/login`);
        const headingFocus = await page.locator(".start h1").evaluate((heading) => {
          heading.tabIndex = -1;
          heading.focus({ preventScroll: true });
          const style = getComputedStyle(heading);
          return document.activeElement === heading && (style.outlineStyle === "none" || parseFloat(style.outlineWidth) === 0);
        });
        ok(headingFocus, `${width}px/${colorScheme}: h1 do login recebe foco programático sem contorno retangular`);
        await page.keyboard.press(process.env.BROWSER === "webkit" && process.platform === "darwin" ? "Alt+Tab" : "Tab");
        const keyboardFocus = await page.evaluate(() => {
          const element = document.activeElement;
          const style = getComputedStyle(element);
          return element.matches('.start a[href], .start button:not([disabled])') && element.matches(":focus-visible") &&
            style.outlineStyle !== "none" && style.outlineStyle !== "hidden" && parseFloat(style.outlineWidth) > 0 &&
            style.outlineColor !== "transparent" && !/^rgba\(.*,[\s]*0\)$/.test(style.outlineColor);
        });
        ok(keyboardFocus, `${width}px/${colorScheme}: Tab mantém foco visível em link ou botão do login`);
        await page.screenshot({ path: join(screenshots, `${width}-${colorScheme}-login.png`), fullPage: true });
        ok(errors.length === 0, `${width}px/${colorScheme}: sem erros de JavaScript ${JSON.stringify(errors)}`);
      } catch (err) {
        ok(false, `${width}px/${colorScheme}: ${err.message}`);
      } finally {
        await context.close();
      }
    }
  }
};

const server = spawn(BIN, [], { env: { ...process.env, PORT: String(PORT), GROQ_API_KEY: "teste", FINAN_PROXY_UPSTREAM: "http://127.0.0.1:9" }, stdio: "ignore" });
for (let i = 0; i < 100; i++) {
  try { if ((await fetch(`${SITE}/healthz`)).ok) break; } catch {}
  await new Promise((r) => setTimeout(r, 100));
}

const browser = await chromium.launch({ executablePath: process.env.CHROMIUM_PATH || undefined });
try {
  const page = await browser.newPage({ ...devices["Pixel 7"] });
  const errors = [];
  watchErrors(page, errors);
  page.on("dialog", (d) => d.accept());

  // Antes de escolher a tela (landing ou Painel), a moldura do app não aparece.
  await page.addInitScript(() => document.addEventListener("DOMContentLoaded", () => {
    window.__shellBefore = getComputedStyle(document.querySelector("nav.tabs")).visibility;
  }));
  // Primeira visita: a landing, sem menu, cabendo na tela do celular.
  await page.goto(`${SITE}/`);
  await page.waitForSelector(".landing .lp-hero h1", { timeout: 20000 });
  ok(await page.locator("body.landing-mode").count() === 1 && await page.locator(".tabs").isHidden(), "primeira visita mostra a landing, sem o menu");
  ok(await page.getAttribute(".lp-hero a.lp-cta", "href") === "/?entrar=1", "Começar agora leva ao login");
  ok(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), "landing cabe na largura do celular");
  ok(await page.locator(".lp-agent").count() === 3, "landing mostra o time (Otávio, Lia e Tito)");
  ok(await page.evaluate(() => window.__shellBefore) === "hidden", "o menu do app não pisca antes da landing");
  ok(await page.evaluate(() => { const s = document.querySelector(".lp-faq"), w = s.querySelector(".lp-wrap"), q = s.querySelector("summary");
    return getComputedStyle(w).position === "static" && getComputedStyle(w).backgroundColor === "rgba(0, 0, 0, 0)" && getComputedStyle(q).color === "rgb(34, 35, 35)"; }),
    "estilos do app não vazam na landing (dúvidas com fundo claro e texto escuro)");

  ok(await page.locator('[data-action="solo"]').count() === 0 && !/Usar sem conta/.test(await page.innerText("main")), "a landing não oferece uso sem conta");
  await page.click(".lp-brand");
  ok(new URL(page.url()).pathname === "/" && await page.locator(".lp-hero h1").isVisible(), "o ícone da pantera, sem login, fica na página inicial");

  // Login: sem "Usar sem conta"; com o Google ainda não ligado no servidor, avisa.
  await page.goto(`${SITE}/?entrar=1`);
  await page.waitForSelector(".start [data-unavailable]:not([hidden])", { timeout: 20000 });
  ok(await page.locator("body.landing-mode").count() === 0, "fora da landing, o modo tela cheia sai");
  const welcome = await page.innerText("main");
  ok(await page.locator(".start-google").isHidden() && /abre em breve/.test(welcome) && !/Usar sem conta|Começar a usar/.test(welcome),
    "o login sem Google ligado avisa e não deixa usar sem conta");
  await page.click(".start .brand");
  await page.waitForSelector(".lp-hero h1");
  ok(new URL(page.url()).search === "", "o ícone da pantera na tela de login volta para a landing");

  // Barreira: nenhum endereço do app abre sem login, nem digitado, nem com dados antigos no navegador.
  await page.evaluate(async () => {
    const f = await import("/static/js/finance.js");
    await f.load();
    // Grava como se tivessem vindo de fora (applyRemote não pede sessão), no banco que estiver em uso.
    await (await import("/static/js/db.js")).applyRemote(f.defaultCategories(Date.now()).map((record) => ({ type: "category", record })));
  });
  let leaked = [];
  for (const path of ["/", "/dividas", "/lancamentos", "/metas", "/analise", "/contas", "/chat", "/configuracoes", "/exportar"]) {
    await page.goto(`${SITE}${path}`);
    await page.waitForSelector(".landing, .start", { timeout: 20000 });
    const url = new URL(page.url());
    const visible = await page.evaluate(() => ["nav.tabs", "header.top", "#sync-status"].filter((q) => document.querySelector(q)?.checkVisibility({ visibilityProperty: true })));
    if (visible.length || !/^\/(\?entrar=1)?$/.test(url.pathname + url.search) || await page.locator(".cards, .tx-card, table:not(.lp-compare)").count()) leaked.push(path);
  }
  ok(leaked.length === 0, `sem login, nenhum endereço do app abre, mesmo com dados no navegador (vazou: ${leaked})`);

  // Ajustes do site (conferidos com login em google.test): aqui só a política de privacidade.
  // Política de privacidade (o Google exige o link).
  const privacy = await page.request.get(`${SITE}/privacidade.html`);
  ok(privacy.ok() && /drive\.appdata/.test(await privacy.text()), "política de privacidade publicada");
  // Página de baixar o app (o QR code leva a ela), aberta sem login.
  const dl = await browser.newPage();
  await dl.goto(`${SITE}/baixar`);
  const qr = await dl.request.get(`${SITE}/baixar/qr.png`);
  const qrShown = await dl.$eval('img[src="/baixar/qr.png"]', (img) => img.complete && img.naturalWidth > 0).catch(() => false);
  ok((await dl.textContent("h1")) === "Baixe a Rich" && qrShown && qr.headers()["content-type"] === "image/png",
    "página /baixar sem login, com o QR code que a abre no celular");
  const apk = await dl.request.get(`${SITE}/baixar/android`, { maxRedirects: 0 });
  await dl.close();
  ok(apk.status() === 503 || (apk.status() === 302 && /\/android\/finan_rich\.apk$/.test(apk.headers().location)),
    `baixar o app: leva ao APK, ou avisa que ainda não está no ar (${apk.status()})`);

  // Segurança: a página do site só fala https (nada de http para a rede local).
  const csp = await page.getAttribute('meta[http-equiv="Content-Security-Policy"]', "content");
  ok(/connect-src 'self' https:;/.test(csp) && !/ http:/.test(csp), `CSP do site só https (${csp.match(/connect-src[^;]*/)?.[0]})`);

  // No mesmo endereço, o intermediário da IA continua respondendo.
  ok(await (await fetch(`${SITE}/healthz`)).text() === "ok", "intermediário da IA no mesmo endereço (/healthz)");
  const chat = await fetch(`${SITE}/chat/completions`, { method: "POST", headers: { "Content-Type": "application/json" }, body: '{"messages":[{"role":"user","content":"oi"}]}' });
  ok(chat.status === 403, `pedido fora do formato do app continua recusado pelo intermediário (${chat.status})`);
  ok((await fetch(`${SITE}/static/nao-existe.js`)).status === 404, "arquivo que não existe dá 404, não a página do app");

  ok(errors.length === 0, `sem erros de JavaScript ${JSON.stringify(errors)}`);
  await checkLandingVisual(browser);
} catch (err) {
  ok(false, err.message);
} finally {
  await browser.close();
  server.kill();
}
console.log(failures ? `\n${failures} falha(s)` : "\nTudo certo.");
process.exit(failures ? 1 : 0);
