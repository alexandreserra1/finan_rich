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
