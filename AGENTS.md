# AGENTS.md

Guia para agentes de IA (Claude Code, Codex, Cursor, Copilot, Gemini…) que trabalham neste
repositório. Vale para qualquer pedido, do menor ao maior.

## O projeto

finan_rich: finanças pessoais offline-first. Um binário Go (servidor + telas web embutidas)
roda no computador; o app do celular (Capacitor) guarda os dados no aparelho e sincroniza com
ele. As regras de negócio são Go, compiladas também para WebAssembly e usadas no navegador.

| Parte | Onde |
|---|---|
| Programa do computador | `cmd/finanrich`, `internal/web` (HTTP, telas, sync), `internal/store` (SQLite) |
| Regras (Go → WebAssembly) | `internal/domain`, `internal/finance`, `internal/money`, `internal/report`, `internal/importer`, `internal/wasmapi`, `cmd/finanwasm` |
| Telas (JS sem framework) | `internal/web/static/js`, `internal/web/static/app.css`, `internal/web/templates` |
| App do celular | `mobile/` (Capacitor 8; `mobile/www` é gerado por `tools/mobilebuild`) |
| Intermediário da IA grátis (Vercel; ou Cloud Run) | `cmd/api`, `internal/proxy`, `vercel.json` (Vercel), `scripts/deploy-proxy.sh` (Cloud Run) ([docs/IA-GRATIS.md](docs/IA-GRATIS.md)) |
| Site (as telas do app na internet) | `site/` (servidor e `privacidade.html`), modo `-site` do `tools/mobilebuild`; vai no mesmo programa da Vercel (`cmd/api -tags sitebundle`) |
| Testes no navegador | `e2e/*.test.mjs` (Playwright) |
| Lojas | `fastlane/metadata/android`, `fdroid/`, `docs/DISTRIBUICAO.md` |

Arquitetura: `docs/ARCHITECTURE.md`. App do celular: `docs/MOBILE.md`. Licenças: `docs/LICENSES.md`.

## Comandos

```bash
make wasm          # gera internal/web/static/finance.wasm (precisa existir antes de go build/test)
make test          # gofmt, go vet, go test e as regras rodando como WebAssembly
make e2e           # testes no navegador (offline, sync, app, importação, exportação, metas)
make mobile        # mobile/www + cap sync;  make apk: APK de teste (Android SDK + Java 21)
scripts/check-licenses.sh   # só dependências de código aberto
scripts/clean.sh   # limpeza (veja "Limpeza" abaixo)
```

## Regras da casa

- Tudo que a pessoa lê (telas, mensagens de erro, docs, comentários, commits) é em
  **português do Brasil**, frases curtas e palavras do dia a dia.
- Dinheiro sempre em centavos (`int64`); datas `AAAA-MM-DD`; nada de `float` para valores.
- Toda regra nova de negócio vai em Go (pacotes de regras), não em JS: assim vale no
  computador, no navegador e no celular.
- Arquivo novo em `internal/web/static/js` entra na lista `PRECACHE` do `sw.js` (um teste
  confere).
- Dependência nova: só código aberto (MIT/BSD/ISC/Apache/MPL) e só se não der com o que já
  existe; `scripts/check-licenses.sh` barra o resto.
- Versão é publicada só pelo workflow Release (ele grava `version.properties` e cria a tag).
- Nunca versionar `dist/`, `mobile/www`, `finance.wasm`, bancos `*.db`, `.env`, keystores.

## Como programar: Ponytail + ciclo de skills

As skills ficam em `.claude/skills/<nome>/SKILL.md` (vindas de
[addyosmani/agent-skills](https://github.com/addyosmani/agent-skills) e
[DietrichGebert/ponytail](https://github.com/DietrichGebert/ponytail), MIT; veja
`.claude/THIRD_PARTY.md`). Agentes sem suporte a skills: leiam o `SKILL.md` indicado e sigam.

**Ponytail está sempre ligado (nível full)**: a menor mudança que resolve o pedido inteiro.
Reusar o que o projeto já tem, depois a biblioteca padrão, depois uma dependência já
instalada; nada de abstração, opção ou config que ninguém pediu. Nunca cortar validação,
tratamento de erro que evita perda de dados, segurança ou acessibilidade. Lógica nova não
trivial (ramo, parser, dinheiro, segurança) sai com um teste. Atalho com limite conhecido
leva o comentário `// shortcut: <limite>, <quando melhorar>` (listados por `/ponytail-debt`).
Terminar a resposta com uma ou duas linhas: o que ficou de fora e o risco que a pessoa
precisa saber.

**Caveman está ligado por padrão** (skill `caveman`, de
[JuliusBrussee/caveman](https://github.com/JuliusBrussee/caveman), Apache-2.0): as respostas
do agente para a pessoa são curtas, resposta primeiro, sem cerimônia, com todo fato técnico,
código, caminho e erro exatos. Vale **só para a conversa**: telas, mensagens do app, docs,
comentários, commits e PRs continuam em português claro e completo (regra 5 da própria
skill). Perguntas à pessoa, avisos de segurança e ações irreversíveis vão em frases
completas. "normal mode" ou "stop caveman" desliga na conversa.

**Ciclo** (cada fase chama as skills dela; pedido pequeno passa rápido pelas fases, mas
passa):

| Fase | Quando | Skills | Comando |
|---|---|---|---|
| DEFINIR | funcionalidade nova ou pedido vago | `spec-driven-development` | `/spec` |
| PLANEJAR | mais de um passo | `planning-and-task-breakdown` | `/plan` |
| CONSTRUIR | sempre | `ponytail`, `incremental-implementation`, `test-driven-development`; tela: `frontend-ui-engineering` | `/build` |
| VERIFICAR | erro, teste falhando, comportamento estranho | `debugging-and-error-recovery` | `/test` |
| REVISAR | antes de commit grande ou merge | `ponytail-review`, `code-review-and-quality`, `code-simplification`; segurança: `security-and-hardening` | `/quality-review`, `/ponytail-review`, `/code-simplify` |
| ENTREGAR | merge na main, release | `shipping-and-launch`, `git-workflow-and-versioning`, `ci-cd-and-automation` | `/ship` |

Passo arriscado ou irreversível (apagar dados, migração, auth, segredos, publicar):
`doubt-driven-development` e confirmação da pessoa antes. Decisão de arquitetura que vale
registrar: `documentation-and-adrs`. Lentidão: `performance-optimization` (medir antes).
Auditoria do repo inteiro: `/ponytail-audit`.

## Orquestração de agentes

Três camadas, como no agent-skills: **skills** (o como), **agentes/personas** (o quem, em
`.claude/agents/`), **comandos** (o quando, em `.claude/commands/`). Quem orquestra é a
sessão principal (ou um comando); **um agente não chama outro agente**.

| Agente | Para quê |
|---|---|
| `code-reviewer` | revisão em 5 eixos (correção, leitura, arquitetura, segurança, desempenho) |
| `security-auditor` | ameaças, OWASP, segredos, auth (pareamento, tokens, CORS, acesso pela rede) |
| `test-engineer` | lacunas de teste: Go, WebAssembly e e2e |
| `web-performance-auditor` | carga das telas, service worker, tamanho do wasm |
| `release-manager` | publicar versão e manter lojas (F-Droid/IzzyOnDroid) em dia |
| `cleanup` | limpar restos de build, caches e Docker sem tocar em dados |

Padrão de orquestração: **fan-out em paralelo com uma etapa de junção**. O `/ship` dispara
`code-reviewer`, `security-auditor` e `test-engineer` na mesma rodada (chamadas paralelas),
junta os relatórios na sessão principal e decide GO/NO-GO com plano de volta. Pesquisa ampla
no código pode usar agentes de busca em paralelo do mesmo jeito; mudanças no mesmo arquivo
nunca vão para dois agentes ao mesmo tempo.

## Limpeza

`scripts/clean.sh` é a única fonte das regras de limpeza (o agente `cleanup` e o comando
`/cleanup` só chamam ele). Por padrão mostra o que apagaria; `--yes` apaga.

| Nível | O que sai | Volta com |
|---|---|---|
| (padrão) | `dist/`, `finance.wasm`, `wasm_exec.js`, `mobile/www`, builds do Gradle e do Xcode, cópias das telas dentro de `mobile/android`/`mobile/ios`, resultados de teste, `*.apk` soltos | `make wasm`, `make mobile`, `make apk` |
| `--deps` | `mobile/node_modules`, `e2e/node_modules` | `npm ci` |
| `--caches` | cache de build/testes do Go, cache do npm, cache de build do Gradle | o próximo build |
| `--docker` | contêineres parados, imagens sem tag, cache de build | `docker compose build` |

Nunca sai: bancos `*.db`, pasta de dados, `.env`, keystores, arquivos versionados e volumes do
Docker (`finan-data` tem os dados de quem usa pelo Docker). Rode a limpeza padrão antes de
medir tamanho de APK/binário ou quando o disco encher; depois, `make wasm` antes de compilar.

## Pronto quer dizer

`make test` passa, `make e2e` passa quando mexeu em tela ou sync, `scripts/check-licenses.sh`
passa quando mexeu em dependência, docs atualizadas quando o comportamento mudou, e a
resposta final diz o que não foi verificado.
