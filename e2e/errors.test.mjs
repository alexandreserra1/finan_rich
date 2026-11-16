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

// O app pareado fala com o computador em outro endereço, que libera /api/ para
// qualquer origem: o cancelamento de um pedido a ele não entra; outro endereço, sim.
page = fakePage("http://127.0.0.1:18097/lancamentos");
errors = [];
watchErrors(page, errors, undefined, ["http://127.0.0.1:18098"]);
page.emit("pageerror", msg("http://127.0.0.1:18098/api/openfinance"));
page.emit("pageerror", msg("http://127.0.0.1:18099/api/openfinance"));
assert.equal(errors.length, 1);
// Às vezes o WebKit manda o erro sem pilha: o endereço sai do nome e da mensagem.
page.emit("pageerror", { ...msg("http://127.0.0.1:18098/api/openfinance"), stack: "" });
page.emit("pageerror", { ...msg("http://127.0.0.1:18097/api/sync"), stack: "" });
page.emit("pageerror", { ...msg("http://127.0.0.1:18099/api/sync"), stack: "" });
assert.equal(errors.length, 2);

// Página já fechada (about:blank) não tem endereço: o erro vale.
page = fakePage("about:blank");
errors = [];
watchErrors(page, errors);
page.emit("pageerror", msg("http://127.0.0.1:18097/api/sync"));
assert.equal(errors.length, 1);

console.log("Tudo certo.");

// Carga de módulo cancelada (Worker do SQLite importando sqlite3.js no WebKit do CI, 16/11):
// o console avisa do cancelamento de um arquivo do mesmo endereço e, junto, vem o pageerror
// sem pilha. Só esse par é descartado.
const consoleMsg = (text) => ({ text: () => text });
page = fakePage("http://127.0.0.1:18097/configuracoes");
errors = [];
watchErrors(page, errors);
page.emit("console", consoleMsg("Cannot load http://127.0.0.1:18097/static/vendor/sqlite/sqlite3.js due to access control checks."));
page.emit("pageerror", { name: "TypeError", message: "Importing a module script failed.", stack: "" });
assert.deepEqual(errors, [], "módulo cancelado com o aviso do console não entra");
// Sem o aviso de cancelamento, a mesma falha continua sendo erro de verdade.
page = fakePage("http://127.0.0.1:18097/configuracoes");
errors = [];
watchErrors(page, errors);
page.emit("pageerror", { name: "TypeError", message: "Importing a module script failed.", stack: "" });
assert.equal(errors.length, 1, "módulo que falha sem cancelamento continua valendo");
// O cancelamento é de outro endereço: não vale.
page = fakePage("http://127.0.0.1:18097/configuracoes");
errors = [];
watchErrors(page, errors);
page.emit("console", consoleMsg("Cannot load https://exemplo.com/x.js due to access control checks."));
page.emit("pageerror", { name: "TypeError", message: "Importing a module script failed.", stack: "" });
assert.equal(errors.length, 1, "cancelamento de outro endereço não perdoa");
// O aviso é antigo (mais de 2 s): não vale.
page = fakePage("http://127.0.0.1:18097/configuracoes");
errors = [];
watchErrors(page, errors);
const real = Date.now;
page.emit("console", consoleMsg("Cannot load http://127.0.0.1:18097/static/a.js due to access control checks."));
Date.now = () => real() + 3000;
page.emit("pageerror", { name: "TypeError", message: "Importing a module script failed.", stack: "" });
Date.now = real;
assert.equal(errors.length, 1, "aviso de mais de 2 s atrás não perdoa");
