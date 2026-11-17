# Skills e agentes de terceiros

Copiados (sem mudanças de conteúdo, só o prefixo `agent-skills:` removido dos nomes das
skills e um ajuste em `commands/test.md` para usar os testes Playwright do projeto):

| Origem | Commit | Licença | O que veio |
|---|---|---|---|
| [addyosmani/agent-skills](https://github.com/addyosmani/agent-skills) | `aeac41d` | MIT ([texto](licenses/agent-skills.LICENSE)) | 16 skills em `skills/`, `references/`, os agentes `code-reviewer`, `security-auditor`, `test-engineer`, `web-performance-auditor` e os comandos `spec`, `plan`, `build`, `test`, `code-simplify`, `ship`, `webperf` e `quality-review` (o `review` original, renomeado para não colidir com o `/review` do Claude Code) |
| [DietrichGebert/ponytail](https://github.com/DietrichGebert/ponytail) | `2961b52` | MIT ([texto](licenses/ponytail.LICENSE)) | skills `ponytail`, `ponytail-review`, `ponytail-audit`, `ponytail-debt` |

| [JuliusBrussee/caveman](https://github.com/JuliusBrussee/caveman) | `d262ab3` | Apache-2.0 ([texto](licenses/caveman.LICENSE), [NOTICE](licenses/caveman.NOTICE)) | skill `caveman` (só a skill: sem a CLI, o proxy, a telemetria ou o Caveman Cloud) |

Próprios do projeto: agentes `cleanup` e `release-manager`, skill `cleanup`.

Engenharia compartilhada (16/11/2026): OpenSpec 2.0.0 (MIT) gerou as seis skills
`openspec-*` em `.agents/skills`, ligadas ao Claude. Playwright MCP 0.0.85 e Chrome
DevTools MCP 1.13.0 usam Apache-2.0. Pacotes e licenças transitivas ficam no lockfile de
`tools/engineering`; são ferramentas locais, não dependências do produto. As personas
`architecture-reviewer`, `frontend-design-reviewer`, `backend-reviewer`, `root-cause-analyst`,
`knowledge-maintainer` e a skill `rich-engineering` são próprias do projeto. Superpowers
já está habilitado nos perfis locais do Claude e não foi copiado nem reinstalado aqui.

Para atualizar: clonar os repositórios, copiar as mesmas pastas por cima e repetir os dois
ajustes acima.
