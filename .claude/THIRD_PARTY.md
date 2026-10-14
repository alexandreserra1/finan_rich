# Skills e agentes de terceiros

Copiados (sem mudanças de conteúdo, só o prefixo `agent-skills:` removido dos nomes das
skills e um ajuste em `commands/test.md` para usar os testes Playwright do projeto):

| Origem | Commit | Licença | O que veio |
|---|---|---|---|
| [addyosmani/agent-skills](https://github.com/addyosmani/agent-skills) | `aeac41d` | MIT ([texto](licenses/agent-skills.LICENSE)) | 16 skills em `skills/`, `references/`, os agentes `code-reviewer`, `security-auditor`, `test-engineer`, `web-performance-auditor` e os comandos `spec`, `plan`, `build`, `test`, `code-simplify`, `ship`, `webperf` e `quality-review` (o `review` original, renomeado para não colidir com o `/review` do Claude Code) |
| [DietrichGebert/ponytail](https://github.com/DietrichGebert/ponytail) | `2961b52` | MIT ([texto](licenses/ponytail.LICENSE)) | skills `ponytail`, `ponytail-review`, `ponytail-audit`, `ponytail-debt` |

Próprios do projeto: agentes `cleanup` e `release-manager`, skill `cleanup`.

Para atualizar: clonar os repositórios, copiar as mesmas pastas por cima e repetir os dois
ajustes acima.
