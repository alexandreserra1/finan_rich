// Confere o watchErrors (errors.mjs) sem navegador, com uma página simulada.
import { EventEmitter } from "node:events";
import assert from "node:assert/strict";
import { watchErrors } from "./errors.mjs";

// O erro como o Playwright monta no WebKit (server/webkit/wkPage.js): ele corta o
// texto no primeiro ":", que é o de "http:"; o texto inteiro fica só no stack.
const msg = (url) => {
  const text = `Fetch API cannot load ${url} due to access control checks.`;
  const cut = text.indexOf(":");
  return { name: text.slice(0, cut), message: text.substring(cut + 2), stack: `${text}\n    at unknown (${url}:1:1)` };
};
const fakePage = (url) => Object.assign(new EventEmitter(), { url: () => url });

// Pedido ao mesmo endereço da página cancelado pela troca de página: não entra,
// mesmo sem o navegador avisar o cancelamento (foi o que faltou no CI de 05/11).
let page = fakePage("http://127.0.0.1:18097/configuracoes"), errors = [];
watchErrors(page, errors, (e) => "A: " + e.message);
page.emit("pageerror", msg("http://127.0.0.1:18097/static/advisor.wasm"));
page.emit("pageerror", msg("http://127.0.0.1:18097/api/sync"));
page.emit("pageerror", { message: "TypeError: x is undefined" });
assert.deepEqual(errors, ["A: TypeError: x is undefined"]);

// Outro endereço (outra porta conta): erro de acesso de verdade, continua valendo.
page = fakePage("http://127.0.0.1:18097/");
errors = [];
watchErrors(page, errors);
page.emit("pageerror", msg("http://127.0.0.1:18098/token"));
page.emit("pageerror", msg("https://exemplo.com/api"));
assert.equal(errors.length, 2);

// Página já fechada (about:blank) não tem endereço: o erro vale.
page = fakePage("about:blank");
errors = [];
watchErrors(page, errors);
page.emit("pageerror", msg("http://127.0.0.1:18097/api/sync"));
assert.equal(errors.length, 1);

console.log("Tudo certo.");
