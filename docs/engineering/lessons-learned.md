# Lições verificadas

## ENG-L001 — Ferramenta instalada não significa servidor conectado

A auditoria encontrou skills de navegador, mas nenhum Playwright MCP ou DevTools MCP no
Codex. A configuração só deve ser declarada operacional após initialize, tools/list e
abertura real de uma página. A prova é `scripts/engineering-browser-proof.mjs`; o resultado
fica em `.artifacts/engineering/browser-proof.json`, quando executada com sucesso.

## ENG-L002 — Gratuidade pessoal não autoriza produto multicliente

O guia pessoal da Pluggy e a página comercial têm condições distintas. A afirmação antiga
de custo zero não era suficiente para assumir distribuição comercial gratuita. A revisão
e as fontes estão em [OPEN-FINANCE.md](../OPEN-FINANCE.md).

## ENG-L003 — Aprovação precisa corresponder ao conteúdo testado

O gate grava o hash do conteúdo e reprova alterações durante sua execução. O Stop exige
evidência atual. Isso reduz aprovação acidental de uma versão anterior, mas não transforma
arquivos locais em um mecanismo de segurança resistente à adulteração; a CI é independente.

## ENG-L004 — Duas suítes e2e ao mesmo tempo falham à toa

As suítes de `e2e/` usam portas fixas (o app em 18097, o computador em 18098). Em 18/11/2026
o WebKit falhou em passos que não tinham mudado ("Conectar a outro computador", escolher o
tema) porque outra suíte ainda rodava em segundo plano, ou a captura de telas disputava a
máquina. Rodando sozinha, a mesma versão passou 3 de 3. Antes de concluir que o código
quebrou: espere a suíte anterior terminar (a notificação chega depois), não rode capturas
junto e repita a suíte sozinha. Uma falha que muda de passo a cada rodada é sinal de
interferência, não de defeito.

## ENG-L005 — O `check affected` não vê o que já foi commitado

O `check affected` escolhe os testes pelos arquivos ainda não commitados: o índice, o
arquivo de trabalho e os arquivos novos. Em 19/11/2026, com o commit feito antes do gate,
ele só viu os atalhos de `node_modules` criados nas pastas de trabalho:
- num PR, rodou só as checagens de ferramentas;
- em outro, o atalho de `mobile/node_modules` puxou o `make e2e`, mas não o `make test`.

O CI cobriu tudo (o job `test` roda `go test -race ./...`), mas o relatório dizia que o gate
local tinha rodado completo. Para evitar:
- rode o gate antes do commit, com a mudança no índice ou no arquivo de trabalho;
- depois do commit, chame o gate pelo nome (`check backend`, `check frontend`);
- em qualquer caso, confira nas linhas `Executando:` o que de fato rodou.
