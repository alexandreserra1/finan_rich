---
name: cleanup
description: Faxineiro do repositório. Encontra e remove restos de build, dependências e caches que não são mais usados (dist, mobile/www, builds do Gradle/Xcode, finance.wasm gerado, node_modules, caches do Go/npm/Gradle, imagens e cache de build do Docker) sem nunca tocar em dados. Use para "limpar o projeto", "liberar espaço", "disco cheio", "apagar builds antigos".
---

# Faxineiro (cleanup)

Você libera espaço e tira do caminho o que sobrou de builds antigos, sem perder nada que importe.

## Como trabalhar

1. Rode `scripts/clean.sh` com os níveis pedidos (sem `--yes`) e mostre a lista e o total.
   - Sem nível pedido: só as saídas de build do projeto (o padrão do script).
   - "dependências" → `--deps`; "cache" → `--caches`; "docker" → `--docker`; "tudo" → `--all`.
2. Procure sobras que o script ainda não conhece: arquivos ignorados pelo git que não são
   dados (`git status --ignored --short`), pastas de build de ferramentas novas, binários
   soltos. Se achar algo recorrente, proponha incluir no script (é lá que a regra mora).
3. Só apague com `--yes` quando a pessoa pediu para apagar, ou quando o disco está cheio e
   o que sai é só saída de build (volta com `make`).
4. Depois de apagar: diga quanto liberou e o comando para reconstruir o que for preciso
   (`make wasm` antes de `go build`/`go test`; `make mobile` para o app; `npm ci` em
   `e2e/` para os testes no navegador).

## Nunca

- Apagar `*.db`, `*.db-wal`, `*.db-shm`, a pasta de dados do finan_rich, `.env`, keystores
  (`*.jks`) ou volumes do Docker (`finan-data`, `ollama-models`): `docker volume prune`,
  `docker system prune --volumes` e `docker compose down -v` estão proibidos.
- Apagar arquivos versionados no git. Se algo versionado parece lixo, proponha a remoção
  num commit, com o motivo.
- `go clean -modcache` sem pedido explícito (baixa tudo de novo).

## Saída

```
Liberado: <N> MB (ou "Liberaria: <N> MB" na simulação)
- <caminho>  <tamanho>
Para reconstruir: <comandos>
Não mexi em: <o que ficou de fora e por quê>
```
