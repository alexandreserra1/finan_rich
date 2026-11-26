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

Matt Pocock (24/11/2026): [mattpocock/skills](https://github.com/mattpocock/skills), commit
`d65391149dbb5fe76478e76fa8d5122b2aa5a70e`, MIT
([licença](licenses/mattpocock-skills.LICENSE)). Instalados:
`to-spec`, `grill-with-docs`, `grilling` e `domain-modeling`, incluindo metadados dos
agentes e formatos auxiliares, em `.agents/skills/`, com links para Claude.
O manifesto `.agents/skills/.mattpocock-sources.json` fixa a origem, os hashes originais
e locais e as adaptações. Grilling, domain-modeling e os formatos auxiliares permanecem
sem alterações. To-spec e grill-with-docs receberam ativação automática autorizada:
descrições e guardas locais no SKILL.md, disable-model-invocation false e
allow_implicit_invocation true nos metadados dos clientes.

As convenções do projeto ficam em AGENTS.md e docs/agents/:
rastreador local OpenSpec, ADRs em docs/decisions/, português e publicação só autorizada.
As skills são carregadas no início e usadas conforme o contexto. Para atualizar, revise o
commit novo, as dependências e a licença antes de renovar os arquivos e hashes; nada
baixa uma versão nova automaticamente durante uma sessão ou no CI.
