import assert from "node:assert/strict";
import test from "node:test";
import { checkCloudflare } from "./check-cloudflare.mjs";

const valid = { CLOUDFLARE_ACCOUNT_ID: "a".repeat(32), CLOUDFLARE_API_TOKEN: "token-ficticio" };
const formatOK = "Formato dos dois secrets válido. Nenhum valor foi exibido ou alterado.";

test("recusa caracteres invisíveis sem enviar nem mostrar os secrets", async () => {
  for (const [suffix, message] of [["\n", "quebra de linha LF"], ["\r", "quebra de linha CR"], ["\t", "tabulação"], [" ", "espaço"], ["\u200b", "caractere fora do ASCII visível"], ["\0", "caractere fora do ASCII visível"]]) {
    const messages = [];
    const env = { ...valid, CLOUDFLARE_API_TOKEN: valid.CLOUDFLARE_API_TOKEN + suffix };
    const result = await checkCloudflare(env, () => assert.fail("não deve chamar a API"), (message) => messages.push(message));
    assert.equal(result, 1);
    assert.deepEqual(messages, [`CLOUDFLARE_API_TOKEN: ${message}. Corrija esse secret no GitHub.`]);
  }
});

test("recusa secrets ausentes e Account ID com formato inválido", async () => {
  for (const env of [{}, { ...valid, CLOUDFLARE_ACCOUNT_ID: "https://dash.cloudflare.com/conta" }]) {
    assert.equal(await checkCloudflare(env, () => assert.fail("não deve chamar a API"), () => {}), 1);
  }
});

test("consulta projetos com Bearer e aceita conta ainda sem projeto", async () => {
  const messages = [];
  const request = async (url, options) => {
    assert.equal(url, `https://api.cloudflare.com/client/v4/accounts/${valid.CLOUDFLARE_ACCOUNT_ID}/pages/projects`);
    assert.equal(options.headers.Authorization, `Bearer ${valid.CLOUDFLARE_API_TOKEN}`);
    assert.equal(options.redirect, "error");
    return { ok: true, json: async () => ({ success: true, result: [] }) };
  };
  assert.equal(await checkCloudflare(valid, request, (message) => messages.push(message)), 0);
  assert.deepEqual(messages, [formatOK,
    "Consulta permitida; nenhum projeto Pages criado ainda. Isso não é erro.",
    "Esta consulta não altera projetos nem confirma a permissão de escrita Pages Write/Edit.",
  ]);
});

test("recusa falha HTTP, JSON inválido e falha da API sem mostrar resposta nem erro", async () => {
  const hiddenError = "Falha de rede, cabeçalho ou resposta inválida. O erro original foi omitido para proteger os secrets.";
  for (const [request, expected] of [
    [async () => ({ ok: false, status: 403, json: async () => assert.fail("não ler erro remoto") }),
      "A consulta ao Cloudflare falhou: HTTP 403. Confira validade, conta e permissão do token."],
    [async () => ({ ok: true, json: async () => { throw new Error(valid.CLOUDFLARE_API_TOKEN); } }), hiddenError],
    [async () => ({ ok: true, json: async () => ({ success: false, errors: [valid.CLOUDFLARE_API_TOKEN] }) }),
      "O Cloudflare não confirmou a consulta aos projetos. A resposta foi omitida por segurança."],
    [async () => { throw new Error(valid.CLOUDFLARE_API_TOKEN); }, hiddenError],
  ]) {
    const messages = [];
    assert.equal(await checkCloudflare(valid, request, (message) => messages.push(message)), 1);
    assert.deepEqual(messages, [formatOK, expected]);
  }
});
