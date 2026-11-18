## Why

O Claude e o Codex trabalham no mesmo repositório pelo Orca, que mostra os MCPs do
`.mcp.json`. Faltavam o mapa do código (graphify) e a memória longa (ai-memory) nessa lista: o
ai-memory só estava na configuração global de cada cliente, e o graphify não estava em
nenhuma. O grafo também não chegava à memória, e dois terços dele eram bibliotecas copiadas.

## What Changes

- `graphify` e `ai-memory` entram no `.mcp.json` e no `.codex/config.toml`, com a mesma lista
  nos dois (teste em `scripts/engineering.test.mjs`). O ai-memory do Codex mantém as
  aprovações da configuração global.
- `scripts/graphify-memoria.sh` atualiza o grafo (só o código, sem IA) e grava o resumo no
  ai-memory em `concepts/mapa-do-codigo`.
- `.graphifyignore` tira do grafo as bibliotecas copiadas, os `.min.js` e pastas locais.
- AGENTS.md, o aviso de início de sessão e o índice de engenharia explicam quando usar cada um.

## Capabilities

### Modified Capabilities
- engineering: mapa do código e memória longa iguais para o Claude e o Codex.

## Impact

Só ferramentas de desenvolvimento: nada entra no app, no site ou no CI. O grafo fica fora do
git. Quem não tiver o graphify ou o ai-memory instalados vê esses dois MCPs sem subir.
