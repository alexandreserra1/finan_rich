# Engenharia verificável do Rich

Esta configuração serve ao Claude Code e ao Codex. Ela não altera a arquitetura do app,
não usa dados bancários reais e não publica alterações automaticamente.

## Diagnóstico e fontes únicas

Auditado em 16/11/2026: Go 1.26.11, JavaScript sem framework, SQLite, WebAssembly,
Capacitor 8 e Playwright 1.64.1. O app é offline-first; Vercel serve o site e o proxy.
O adaptador Supabase existe, mas não significa cadastro ativo: falta projeto e configuração.
Havia 16 scripts E2E e CI de Go, Wasm, Chromium, WebKit, PostgreSQL, Android e iOS.
O Superpowers já estava habilitado nos dois perfis locais do Claude. As skills existentes
já cobrem TDD, diagnóstico sistemático, planejamento, revisão e verificação.

As lacunas eram OpenSpec local, MCPs de navegador nos dois clientes, hooks compartilhados,
gate pre-commit e controle reproduzível da correção. Não foi instalado outro framework
de testes, novo design system, SDK bancário ou agente permanente.

| Assunto | Fonte canônica |
|---|---|
| Regras e convenções | [AGENTS.md](../../AGENTS.md), importado por CLAUDE.md |
| Arquitetura e fronteiras | [ARCHITECTURE.md](../ARCHITECTURE.md) |
| Contratos verificáveis | [openspec/specs](../../openspec/specs) |
| Mudança e tarefas atuais | [engineering-autonomy](../../openspec/changes/engineering-autonomy) |
| Problemas e investigações | [known-issues.md](known-issues.md) |
| Lições com evidência | [lessons-learned.md](lessons-learned.md) |
| Testes e limites | [testing-strategy.md](testing-strategy.md) |
| Critérios visuais | [design-quality.md](design-quality.md) |
| Open Finance e custos | [OPEN-FINANCE.md](../OPEN-FINANCE.md) |
| Segurança e privacidade | [SEGURANCA.md](../SEGURANCA.md) e [LIMITACAO-DE-TAXAS.md](../LIMITACAO-DE-TAXAS.md) |
| Memória entre sessões | ai-memory existente; não duplicar suas páginas duráveis |
| Mapa do código | MCP `graphify` (`graphify-out/`, fora do git); `scripts/graphify-memoria.sh` atualiza e grava no ai-memory |

## Instalação reproduzível

```sh
npm ci --prefix tools/engineering --ignore-scripts --no-fund
node scripts/engineering.mjs install-hooks
node scripts/engineering.mjs doctor
node scripts/engineering.mjs check tooling
```

Node precisa ser >=22.12.0. OpenSpec 2.0.0, Playwright MCP 0.0.85 e Chrome DevTools MCP
1.13.0 são dependências de desenvolvimento locais, com lockfile. O parser YAML 2.9.1,
já transitivo, é declarado diretamente para validar a sintaxe dos workflows. Não entram no binário,
APK, site ou bundle. Chrome precisa estar instalado para os MCPs. Os navegadores dos
E2E continuam independentes, instalados pelos comandos existentes.

OpenSpec foi escolhido por permitir adoção incremental no projeto existente. Spec-kit
não foi instalado para evitar dois formatos concorrentes. As seis skills oficiais geradas
ficam em `.agents/skills/openspec-*`, com links para Claude. Seus exemplos usam `openspec`;
neste repositório use sempre `tools/engineering/node_modules/.bin/openspec` no lugar do
comando global. Desative telemetria com `OPENSPEC_TELEMETRY=0 DO_NOT_TRACK=1`.

## Fluxo e ativação

Investigar → especificar → planejar → implementar → testar → revisar → corrigir → validar
→ registrar. Mudanças relevantes têm proposta, desenho e tarefas na própria pasta OpenSpec.
Correções triviais usam o contrato existente e teste proporcional, sem burocracia extra.

- `SessionStart`: injeta o índice e identifica alterações pendentes. Não roda testes pesados.
- `Stop`: verifica sintaxe e exige evidência atual de `check affected` para código alterado.
  A aprovação é ligada ao hash dos arquivos; uma edição posterior invalida a evidência.
- Falha: o hook pede investigação ao agente atual. Limites padrão: cinco pedidos e quinze
  minutos por problema. A mesma falha sem mudança interrompe o ciclo, ainda reprovado.
- `pre-commit`: verifica os blobs preparados no índice, formatação Go, sintaxe JS, JSON,
  whitespace e caminhos proibidos. Não adiciona, formata ou comita arquivos sozinho.
- CI: job `engineering` valida OpenSpec, controlador e vulnerabilidades das ferramentas.
  Os jobs completos existentes continuam independentes e não aceitam reparos cegos.
  Capturas da landing com dados fictícios ficam como artefatos de revisão por sete dias.

Os hooks nativos dependem da confiança aprovada pelo cliente e da abertura de uma nova
sessão. Não alteramos permissões globais ou a confiança para contornar isso. `codex mcp
list --json` reconhece os dois servidores locais neste checkout. O Claude lê `.mcp.json`
após aprovar os servidores do projeto. Nesta implantação, `claude mcp get` confirmou
os dois em **Pending approval**; abra o Claude e aprove as entradas do projeto. A prova direta usa os mesmos executáveis e não
substitui essa aprovação. Um clone novo precisa rodar a instalação acima para o Git hook.

Isso é um equivalente controlado ao Ralph Loop, não uma instalação do plugin Ralph.
Não inicia outro LLM, não instala um daemon e não continua trabalhando após fechar a sessão.
Os limites controlam continuações locais; não são um limite monetário garantido do provedor.

## Responsabilidades e skills

A sessão principal orquestra e junta resultados; subagentes não chamam outros agentes.
O Codex recebe o conteúdo integral da persona em `.claude/agents`. Sem subagentes, faz
as mesmas revisões sequencialmente. Escritas no mesmo arquivo nunca são paralelas.

| Responsabilidade | Persona | Skills reaproveitadas |
|---|---|---|
| Arquitetura | architecture-reviewer | architecture-review, code-review-and-quality |
| Frontend e design | frontend-design-reviewer | frontend-ui-engineering, Taste, Transitions |
| Backend | backend-reviewer | api-and-interface-design, security-and-hardening |
| QA | test-engineer existente | test-driven-development, incremental-implementation |
| Causa-raiz | root-cause-analyst | debugging-and-error-recovery |
| Conhecimento | knowledge-maintainer | documentation-and-adrs, ai-memory conforme roteamento |

Superpowers no Claude é complementar, não substitui regras locais. No Codex usamos as
skills equivalentes existentes, sem instalar outra cópia concorrente do framework.

## Segurança e manutenção

MCPs usam perfis isolados. O launcher não repassa variáveis de credenciais e desativa
estatísticas, consulta CrUX e busca de atualização do DevTools. Playwright limita origens
a localhost; essa opção não é uma sandbox de segurança. DevTools ainda pode navegar fora
do projeto se o agente pedir: use somente dados fictícios e URLs locais durante QA.

Evidências ficam em `.artifacts/engineering`, ignorada pelo Git. Não publique screenshots,
DOM, rede ou logs de contas reais. As remoções de credenciais conhecidas dos diagnósticos
são defesa adicional, não garantia de anonimização de texto arbitrário.

Fontes: [OpenSpec](https://github.com/Fission-AI/OpenSpec),
[Superpowers](https://github.com/obra/superpowers),
[Playwright MCP](https://github.com/microsoft/playwright-mcp),
[DevTools MCP](https://github.com/ChromeDevTools/chrome-devtools-mcp),
[hooks do Claude](https://code.claude.com/docs/en/hooks),
[hooks do Codex](https://learn.chatgpt.com/docs/hooks),
[Ralph Loop oficial](https://github.com/anthropics/claude-plugins-official/tree/main/plugins/ralph-loop).

## Resultado desta implantação

Os resultados finais e suas limitações ficam em [validation.md](validation.md).
