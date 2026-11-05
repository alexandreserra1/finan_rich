// Confere o watchErrors (errors.mjs) sem navegador: eventos simulados nas duas ordens.
import { EventEmitter } from "node:events";
import assert from "node:assert/strict";
import { watchErrors } from "./errors.mjs";

const cancel = (url) => ({ url: () => url, failure: () => ({ errorText: "cancelled" }) });
const msg = (url) => ({ message: `Fetch API cannot load ${url} due to access control checks.` });

// Aviso do pedido cancelado antes do erro: o erro nem entra.
let page = new EventEmitter(), errors = [];
watchErrors(page, errors);
page.emit("requestfailed", cancel("http://x/api/sync"));
page.emit("pageerror", msg("http://x/api/sync"));
assert.deepEqual(errors, []);

// Erro antes do aviso: entra e sai quando o cancelamento chega; os outros ficam.
page = new EventEmitter(); errors = [];
watchErrors(page, errors, (e) => "A: " + e.message);
page.emit("pageerror", msg("http://x/static/advisor.wasm"));
page.emit("pageerror", { message: "TypeError: x is undefined" });
page.emit("requestfailed", cancel("http://x/static/advisor.wasm"));
assert.deepEqual(errors, ["A: TypeError: x is undefined"]);

// Sem cancelamento daquele endereço (ou falha que não é cancelamento), o erro vale.
page = new EventEmitter(); errors = [];
watchErrors(page, errors);
page.emit("requestfailed", { url: () => "http://x/api/sync", failure: () => ({ errorText: "Connection refused" }) });
page.emit("requestfailed", cancel("http://x/outro"));
page.emit("pageerror", msg("http://x/api/sync"));
assert.equal(errors.length, 1);

console.log("Tudo certo.");
