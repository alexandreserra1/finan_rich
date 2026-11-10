import { pathToFileURL } from "node:url";

export async function checkCloudflare(env, request = fetch, report = console.log) {
  let invalid = false;
  for (const name of ["CLOUDFLARE_ACCOUNT_ID", "CLOUDFLARE_API_TOKEN"]) {
    const value = env[name] || "";
    const problems = [];
    if (!value) problems.push("secret ausente");
    if (value.includes("\n")) problems.push("quebra de linha LF");
    if (value.includes("\r")) problems.push("quebra de linha CR");
    if (value.includes("\t")) problems.push("tabulação");
    if (value.includes(" ")) problems.push("espaço");
    if (/[^\x20-\x7e\r\n\t]/.test(value)) problems.push("caractere fora do ASCII visível");
    if (name === "CLOUDFLARE_ACCOUNT_ID" && value && !/^[a-f0-9]{32}$/i.test(value)) {
      problems.push("o Account ID deve ter 32 caracteres hexadecimais, sem URL");
    }
    if (problems.length) {
      report(`${name}: ${problems.join("; ")}. Corrija esse secret no GitHub.`);
      invalid = true;
    }
  }
  if (invalid) return 1;
  report("Formato dos dois secrets válido. Nenhum valor foi exibido ou alterado.");
  try {
    const response = await request(`https://api.cloudflare.com/client/v4/accounts/${env.CLOUDFLARE_ACCOUNT_ID}/pages/projects`, {
      headers: { Authorization: `Bearer ${env.CLOUDFLARE_API_TOKEN}` },
      redirect: "error",
      signal: AbortSignal.timeout(30_000),
    });
    if (!response.ok) {
      report(`A consulta ao Cloudflare falhou: HTTP ${response.status}. Confira validade, conta e permissão do token.`);
      return 1;
    }
    const data = await response.json();
    if (data?.success !== true || !Array.isArray(data.result)) {
      report("O Cloudflare não confirmou a consulta aos projetos. A resposta foi omitida por segurança.");
      return 1;
    }
    report(data.result.length ? "Consulta aos projetos Pages permitida." : "Consulta permitida; nenhum projeto Pages criado ainda. Isso não é erro.");
    report("Esta consulta não altera projetos nem confirma a permissão de escrita Pages Write/Edit.");
    return 0;
  } catch {
    report("Falha de rede, cabeçalho ou resposta inválida. O erro original foi omitido para proteger os secrets.");
    return 1;
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  process.exitCode = await checkCloudflare(process.env);
}
