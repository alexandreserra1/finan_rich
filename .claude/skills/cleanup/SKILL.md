---
name: cleanup
description: >
  Limpa restos de build, dependências, caches e imagens Docker sem uso, sem
  tocar em dados. Use para "limpar", "clean up", "liberar espaço", "disco
  cheio", "apagar builds antigos", /cleanup [deps|caches|docker|all].
argument-hint: "[deps|caches|docker|all] [--yes]"
---

Delegue ao agente `cleanup` (Agent tool, `subagent_type: cleanup`) passando os níveis
pedidos em `$ARGUMENTS`. Sem `--yes` nos argumentos, é só simulação: mostre o que sairia e
quanto libera, e pergunte antes de apagar. As regras do que nunca apagar estão em
`.claude/agents/cleanup.md` e em `scripts/clean.sh`.
