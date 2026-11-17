# Validação da implantação — 16/11/2026

## A. Diagnóstico inicial

O Rich já tinha domínio Go/Wasm, SQLite, JavaScript sem framework, Capacitor, 16 scripts
E2E, testes Go, contrato PostgreSQL e CI multiplataforma. Superpowers e skills equivalentes
já existiam. Faltavam contratos OpenSpec, MCPs de navegador nos dois clientes, gates locais
compartilhados e um ciclo de correção limitado. Não foi medido percentual de cobertura.

## B. Ferramentas integradas

- OpenSpec 2.0.0 local, com seis skills oficiais compartilhadas entre Claude e Codex.
- Playwright MCP 0.0.85 e Chrome DevTools MCP 1.13.0, isolados e com versões fixas.
- YAML 2.9.1, já transitivo, declarado para validar configurações sem inventar um linter.
- Superpowers, TDD, diagnóstico, planejamento e revisão existentes reaproveitados.
- Spec-kit e plugin Ralph não foram instalados: evitamos fluxos concorrentes e adotamos
  um controlador local equivalente, sem daemon nem outro LLM.

## C. Alterações

Contratos em `openspec/specs`, proposta/desenho/tarefas em `openspec/changes/engineering-autonomy`,
ferramentas em `tools/engineering`, comandos e provas em `scripts/engineering*.mjs`.
Hooks em `.claude/settings.json`, `.codex/hooks.json` e `.githooks/pre-commit`; MCPs em
`.mcp.json` e `.codex/config.toml`. Roteamento em AGENTS e na skill rich-engineering.
Cinco personas adicionais complementam os revisores existentes; QA reusa test-engineer.
O índice de engenharia aponta às fontes canônicas, sem copiar arquitetura ou memória.

## D. Automação

O Git hook foi instalado localmente com `core.hooksPath=.githooks`. Testes isolados
comprovaram um commit recusado por sintaxe inválida e um commit aceito após corrigir a
fixture, sem criar commits neste repositório. Hooks SessionStart/Stop foram invocados
com eventos sintéticos e responderam com contexto, pedido de correção e interrupção sem
progresso. Isso não comprova que o cliente recarregou hooks na sessão já aberta.

O job engineering e o upload de screenshots estão configurados na CI. Não foram
publicados nem executados no GitHub nesta tarefa. Não houve push, merge ou deploy.
Nenhum serviço continuará trabalhando após o encerramento da sessão.

## E. Testes e evidências

Resultados locais executados, sem credenciais reais:

| Verificação | Resultado |
|---|---|
| `make test` | Passou: formatação, vet, testes Go e Wasm |
| `scripts/test-identity-db.sh` | Passou; 5 chamadas aceitas e 15 recusadas no teste concorrente |
| `PORT_OFFSET=4100 make e2e` | Passou: 16 scripts, Chromium e Google simulado |
| `PORT_OFFSET=5200 npm run webkit` em e2e | Passou: 15 scripts do núcleo |
| `node --test scripts/engineering.test.mjs` | 21 testes locais passaram; rerodados no gate final |
| OpenSpec `validate --all --strict` | 5 itens válidos, incluindo a mudança atual |
| `scripts/check-licenses.sh` | Passou para dependências do produto |
| Licenças transitivas das ferramentas | Verificadas por teste sobre o lockfile |
| `npm audit --prefix tools/engineering --audit-level=high` | Nenhuma vulnerabilidade encontrada |
| govulncheck 1.8.0 | Nenhuma vulnerabilidade encontrada |

A prova direta MCP usa initialize, tools/list e navegação no site real local, não apenas
presença de configuração. Os resultados ficam em `.artifacts/engineering/browser-proof.json`,
snapshots e screenshots. A captura móvel completa e recortes da demonstração e do cartão
foram examinados visualmente: contraste e separação da legenda estavam legíveis.
Os E2E verificaram as quatro larguras, dois temas, abas, contraste e ausência de overflow.
Novas capturas não são baselines aprovados nem medição de Core Web Vitals.

O controlador foi exercitado com fixture temporária incorreta, correção somente da fixture
e nova verificação aprovada. Testes cobrem falta de progresso, limite de cinco pedidos,
prazo, exceção, índice diferente do arquivo, aprovação invalidada por falha com mesmo hash,
compactação, arquivo vazio/removido e encerramento de filho e neto.

## F. Limitações

- Claude informou **Pending approval** para os dois MCPs; a pessoa precisa aprová-los uma
  vez ao abrir o cliente. O Codex listou ambos habilitados na configuração do projeto.
- Hooks nativos precisam da confiança nativa e de uma nova sessão. Não alteramos permissões
  globais nem contornamos a aprovação. Testes de hooks são prova do handler, não dessa confiança.
- Não houve teste com contas Google/Drive reais, dados bancários reais, Supabase em produção,
  Android/iOS ou Safari físico. A causa do 403 pessoal do Drive continua não confirmada.
- Comparação de pixels depende de baselines aprovados. LCP, CLS e INP não foram medidos.
- O controle de árvores de processos foi validado no macOS; equivalência no Windows não
  está garantida. Git hooks podem ser ignorados; proteção obrigatória depende da CI e das
  regras de branch, que não foram alteradas.
- Open Finance comercial gratuito não foi confirmado. Não foi removida a Pluggy nem criada
  integração fictícia. Condições pessoais, custos e caminhos estão em OPEN-FINANCE.md.

## G. Estado final

Operacionais localmente: OpenSpec, controlador e gates, pre-commit, testes Go/Wasm/DB/E2E,
ferramentas MCP e documentação compartilhada. Configurados, mas dependentes de aprovação
ou publicação: carregamento dos hooks/MCPs nas próximas sessões e os novos passos da CI.
Pendentes: contratação/autorização de integração bancária, credenciais reais e baselines.

As três alterações nativas do Capacitor já existentes e `docs/rich-mais-recente` foram
preservadas. Nenhum arquivo de dados, segredo, banco real ou configuração global foi alterado.
