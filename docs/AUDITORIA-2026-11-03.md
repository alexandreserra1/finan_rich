# Auditoria de 03/11/2026: o que falta, segurança, testes e exageros

Revisão do repositório inteiro depois da auditoria com uma conta nova (02/11). Duas partes:
o **relatório** (o que foi visto) e o **plano** (como cada item é resolvido, em que ordem e o
que dá para fazer em paralelo). A coluna "Situação" do plano é atualizada a cada entrega.

**O que a Rich é:** finanças pessoais offline-first. As contas são feitas em Go, que roda no
computador, no navegador e no celular (WebAssembly). Os dados ficam no aparelho e sincronizam
com o computador ou com o Google Drive. A IA grátis passa por um intermediário na Vercel.

**Carga considerada:** o dono e algumas dezenas de pessoas. O computador atende uma pessoa.

## Parte 1: relatório

### O que já está bom

- **Testes de integração:** 10 suítes no navegador (~2.700 linhas), com Google, Drive, Pluggy e
  IA simulados. Cobertura do Go entre 66% (`domain`) e 97% (`site`) por pacote.
- **Segurança:** a revisão de 30/10 (passo-a-passo, seção 2.4) continua valendo. Refeito em
  03/11: `govulncheck` sem falhas; o app que vai para o celular sem avisos no `npm audit`.
- **Datas:** dia 31, fevereiro e virada do ano dão certo em parcelas, recorrências, metas e
  faturas (conferido com casos de teste).
- **Sincronização:** grava tudo numa transação; limite de tamanho em toda entrada; proteção
  contra outro site usar o acesso do computador (CSRF e DNS rebinding).
- **Código morto em Go:** nenhum (o que parece sem uso no computador é usado pelo WebAssembly).

### Achados

#### Precisa corrigir

1. **A fatura do cartão nunca é paga** (`internal/finance/accounts.go`)
   - **O que é:** o saldo do cartão é a soma de todas as compras feitas nele.
   - **Problema:** não existe "pagar fatura" nem transferência entre contas. Na auditoria,
     "Cartões em aberto" mostrou R$ 1.049,90 com a fatura atual em R$ 960, e o valor só cresce.
     O patrimônio só cai.
   - **Correção:** transferência entre contas (não conta como gasto nem como entrada) e o botão
     "Pagar fatura" no cartão.
   - **Se não fizer:** patrimônio e "em aberto" errados para todo mundo que usa cartão.

2. **A IA grátis atende qualquer um** (`internal/proxy/proxy.go:574`)
   - **O que é:** o intermediário aceita pedidos que trazem o texto de instruções do app.
   - **Problema:** o código é aberto, então qualquer pessoa copia esse texto. O limite de pedidos
     fica na memória de cada instância da Vercel e recomeça ao reiniciar. Um script em loop
     gasta a cota do dia e todos passam a ver "limite acabou".
   - **Correção:** regra de limite por IP no Firewall da Vercel (no painel, sem código).
   - **Se não fizer:** o problema existe desde já, mas pesa de verdade quando o site for divulgado.

3. **Computador novo começa sem IA** (`internal/config/config.go:28`)
   - **O que é:** a IA padrão no computador é o Ollama com `qwen3.5:9b`.
   - **Problema:** esse modelo não vem instalado. Numa instalação nova, toda pergunta livre
     responde "A IA não respondeu agora".
   - **Correção:** se o modelo padrão faltar, usar um modelo de conversa que já esteja no
     Ollama (continua tudo no computador, sem mudar a privacidade).
   - **Se não fizer:** a primeira impressão do Consultor no computador é ruim.

#### Deveria corrigir

4. **O motor do Safari não roda no CI** (`.github/workflows/ci.yml`)
   - **O que é:** o CI roda os testes no navegador só no Chromium.
   - **Problema:** o site no iPhone usa o motor do Safari (WebKit). A queda ao recarregar o
     Painel nesse motor continua sem explicação, e nada avisa se piorar.
   - **Correção:** um job no CI com o WebKit do Playwright.
   - **Se não fizer:** quebra no iPhone só aparece quando alguém reclamar.

5. **Ninguém confere a IA grátis publicada**
   - **O que é:** o Groq desliga modelos a cada poucas semanas; a lista de modelos fica no código.
   - **Problema:** se todos saírem do ar, a Rich passa a responder sem IA para todos, sem aviso.
   - **Correção:** um workflow diário que chama o endereço real e falha (e manda e-mail) se a IA
     não responder.
   - **Se não fizer:** você descobre pelos usuários.

6. **O Consultor está escrito duas vezes** (`static/chat.js` + `templates/chat.html` e
   `js/pages/advisor.js`)
   - **O que é:** o computador tem uma tela de Consultor própria; o app e o site usam outra.
   - **Problema:** toda mudança é feita duas vezes (no redesign, os dois lados precisaram da
     mesma troca de nome do personagem).
   - **Correção:** o computador usa `pages/advisor.js` com o `/api/chat`, que o celular
     pareado já usa. Saem ~360 linhas.
   - **Se não fizer:** os dois Consultores vão ficando diferentes.

7. **A IA dentro do celular é exagero** (`js/local-ai.js`, parte de `pages/device.js`,
   `advisor.LocalRequest`)
   - **O que é:** um modelo pequeno (Qwen 0,5B) rodando no celular, com download de 470 MB.
   - **Problema:** sem WebGPU, leva minutos por resposta; um modelo desse tamanho erra mais que a
     resposta pronta do app, que é exata e funciona sem internet. Exige receita especial no F-Droid.
   - **Correção:** apagar o recurso, a dependência `@wllama/wllama` e o passo do F-Droid
     (~300 linhas, 2,4 MB a menos no APK).
   - **Se não fizer:** custo de manter um recurso que quase ninguém vai usar.

8. **CSS morto** (`static/app.css`)
   - **O que é:** sobras do visual antigo (`acct-row`, `dash-row`, `hero-split`, `kpi`, `day-tile`…).
   - **Problema:** 74 regras que nenhuma tela usa, carregadas em todas as telas.
   - **Correção:** apagar (~80 linhas).
   - **Se não fizer:** cada mudança de visual tropeça nelas.

9. **Avisos nas ferramentas de montar o app** (`mobile/package-lock.json`)
   - **O que é:** `glob`, `minimatch` e `uuid`, usados só para montar o app (não vão dentro dele).
   - **Problema:** 6 avisos moderados; os de `glob`/`minimatch` saem com `npm audit fix` (sem
     `--force`). O de `uuid` (via `@capacitor/cli`) já era conhecido e só sai com versão nova do CLI.
   - **Correção:** `npm audit fix` e conferir licenças e a montagem do app.
   - **Se não fizer:** risco baixo, mas o aviso continua aparecendo.

#### Seria bom

10. **Computador como IA do intermediário, sem uso** (`cmd/finanlocal`, `internal/localguard`,
    `scripts/finanlocal.sh`)
    - **O que é:** o Mac serve de IA de reserva para a IA grátis.
    - **Problema:** ~345 linhas paradas até ser configurado, e só valem com o Mac ligado.
    - **Correção:** se o Mac não vai ficar ligado como servidor, apagar.
    - **Se não fizer:** só o custo de manter.

11. **SDK da Anthropic** (`internal/llm/anthropic.go`)
    - **O que é:** a biblioteca oficial da Anthropic, só para a opção paga do Claude no computador.
    - **Problema:** puxa ~15 bibliotecas indiretas para 131 linhas de uso.
    - **Correção:** a Anthropic também aceita o formato OpenAI, que a Rich já fala; apagar o
      arquivo e a dependência depois de conferir que as ferramentas funcionam por esse caminho.
    - **Se não fizer:** nada quebra.

### O que NÃO vale adicionar (seria exagero)

- Emulador Android no CI: lento e frágil; testar num celular de verdade custa menos.
- Contador de pedidos compartilhado feito por nós: o Firewall da Vercel resolve.
- Meta de porcentagem de cobertura: os testes que contam já existem.
- Mais provedores de IA: o intermediário já tem 5.
- Google Drive no computador (fase 5 do plano do site) enquanto ninguém pedir.

**Veredito:** começar pelo 1 (o único que dá número errado na tela); o 2 antes de divulgar.
**Enxugar:** ~1.200 linhas a menos e 2 dependências (itens 6, 7, 8, 10 e 11).
**Não conferido:** celular de verdade, login com Google real, Firewall e variáveis da Vercel,
scripts de instalação e o workflow de release.

## Parte 2: plano

### Como cada item é resolvido

Situação conferida na retomada pelo Codex em 03/11/2026. Os achados acima preservam o
diagnóstico original; o estado atual está na última coluna e nas verificações abaixo.

| # | Como | Arquivos | Como confere | Quem | Situação |
|---|---|---|---|---|---|
| 1 | Registro `transfer` separado dos lançamentos; atualiza saldo e faturas, sem alterar gastos e renda. Na tela Contas: "Transferir" e "Pagar fatura". Abate valores antigos primeiro e aceita antecipação. | `internal/domain`, `internal/store`, `internal/finance/accounts.go`, `js/db.js`, `js/main.js`, `js/pages/accounts.js`, `app.css` | Go: validação, saldos, pagamentos, patrimônio. e2e `accounts.test`: fatura paga sem mudar o Painel. | Claude | feito em `939d4fd` |
| 2 | Passo a passo de limite por IP para `/chat/completions` e `/v1/chat/completions`. Mexer no painel é com você. | `docs/IA-GRATIS.md` | Criar a regra e conferir eventos e resposta 429, conforme o limite escolhido. | você; documentação pelo Codex | documentado; configuração pendente |
| 3 | Sem IA salva nos Ajustes e com o modelo padrão faltando no Ollama, usar um modelo local de conversa já baixado. Sem nenhum, manter o aviso e a resposta sem IA. Escolha salva não muda. | `internal/web/aiconfig.go` | Go: seleção, chamada real ao Ollama falso, modelos de nuvem e vetores excluídos, erro e lista vazia. | Codex, concluindo o trabalho do Claude | implementado e testes Go aprovados |
| 4 | Playwright 1.57.0 corrige a queda nativa. Matriz obrigatória Chromium/WebKit no CI após a aprovação local. Google permanece no Chromium porque o mock usa redirecionamento 302. | `.github/workflows/ci.yml`, `e2e/package.json`, testes | Nove suítes WebKit e dez Chromium aprovadas localmente; acompanhar a primeira execução da matriz no GitHub. | Codex | implementado, aprovado localmente e ativado no CI |
| 5 | Workflow diário e manual com `/healthz` e pergunta de conexão no endereço real. Só aceita resposta completa com texto "ok". As notificações dependem das preferências da conta GitHub. | `.github/workflows/ia-diaria.yml`, `docs/IA-GRATIS.md` | Resposta publicada validada, sete casos do validador e execução `41747839412` aprovada no GitHub. | Codex | implementado e conferido no GitHub; notificações pendentes |
| 6 | O computador abre `/chat` com `pages/advisor.js` e `/api/chat`; saíram a tela e as rotas antigas. | `internal/web/chat.go`, `server.go`, `js/main.js`, `js/pages/advisor.js`, testes | Go e suítes `app.test` e `budget.test`. | Claude e agente em paralelo | feito em `0af70a2` |
| 7 | Apagar a IA dentro do celular. | `js/local-ai.js`, `pages/device.js`, `advisor`, `askapi`, `mobile/package.json`, F-Droid, docs | `make test`, `make e2e`, `make mobile`. | eu, **depois da sua confirmação** | aguardando você |
| 8 | Apagar regras de CSS sem uso (contagem final: 78). | `static/app.css` | Busca dos nomes e testes no navegador. | Claude | feito em `1fbf982` |
| 9 | Lockfile atualizado pelo Claude; conferir também licenças e avisos remanescentes antes de outra atualização. | `mobile/package-lock.json` | `npm audit`, licenças e montagem do app. | Claude | parcial: uuid e licenças de ferramentas ainda em revisão |
| 10 | Apagar o computador como IA do intermediário. | `cmd/finanlocal`, `internal/localguard`, `scripts/finanlocal.sh`, `internal/proxy`, docs | `make test`. | Claude, após confirmação registrada na sessão anterior | feito em `8b13010` |
| 11 | Trocar o SDK pelo formato OpenAI da Anthropic. | `internal/llm` | Só com uma chave da Anthropic para testar as ferramentas. | fica para quando houver chave | adiado |

### Ordem e paralelo

1. **Agora, rápido (eu):** este documento; itens 8 e 9. Cada um com commit próprio.
2. **Em paralelo:**
   - um agente numa cópia isolada do repositório faz o 6 (só mexe no Consultor);
   - eu faço o 1 (o maior), depois o 3 e o 5;
   - a rodada do WebKit (item 4) já corre enquanto isso.
3. **Juntar:** trazer o trabalho do agente para a `main`, rodar `make test` e o `make e2e`
   completo, um de cada vez. Os testes no navegador usam portas fixas, então duas rodadas ao
   mesmo tempo dão erro de porta. O agente roda só as suítes dele.
4. **Por último:** o 4 no CI, com a rodada local verde. Os itens 7 e 10 entram quando você
   confirmar; o 2 é no painel da Vercel.

### Verificações da retomada (03/11/2026)

- **1 feito:** transferência e pagamento de fatura em `939d4fd`, com testes em Go e
  `e2e/accounts.test.mjs`. O pagamento abate primeiro valores antigos; não depende apenas
  de pagamentos feitos depois do fechamento, como a proposta inicial dizia.
- **2 documentado:** passo a passo em `docs/IA-GRATIS.md`; a regra ainda precisa ser aplicada
  no painel da Vercel. Cobrir os dois caminhos, `/chat/completions` e `/v1/chat/completions`.
- **3 concluído:** sem escolha salva, prefere modelos locais conhecidos já baixados; ignora
  nuvem e vetores. O teste de privacidade falhava antes da correção: apenas modelos de
  nuvem disponíveis levavam à escolha de `kimi-k2:1t-cloud`. Agora mantém o modelo local
  padrão e o aviso. A revisão também reproduziu aliases remotos sem sufixo de nuvem:
  `remote_host` e `remote_model` da lista do Ollama são conferidos antes de escolher.
  Não altera escolha explícita nem baixa modelo.
- **4 concluído nesta investigação:** a queda original em `offline.test.mjs` foi
  reproduzida. O relatório nativo do macOS aponta `EXC_BREAKPOINT` em
  `WebCore::Navigation::initializeForNewWindow`, a mesma assinatura da
  [regressão #37766 do WebKit usado pelo Playwright](https://github.com/microsoft/playwright/issues/37766).
  Atualização mínima de 1.56.1 (revisão 2215) para 1.57.0 (revisão 2227), que inclui a
  correção upstream. Nenhuma alteração no Go, WebAssembly, gráfico ou dados da pessoa.
  As nove suítes WebKit passaram; a matriz do CI agora exige ambos os motores.
  Antes de trocar de página, os testes offline, de importação e de dívidas aguardam a
  sincronização; isso evita o diagnóstico nativo de cancelamento `access control checks`.
  A câmera falsa foi corrigida no protótipo de `MediaDevices`, mantendo a leitura real
  do QR pelo jsQR. Não houve filtro de `pageerror`, teste removido ou `continue-on-error`.
- **CI Chromium:** o job da main anterior falhou antes dos testes porque o Playwright
  1.56.1 não suporta `ubuntu26.04-x64` (execução `41730531349`). O job `e2e` foi fixado em
  `ubuntu-24.04`, sem trocar dependência nem desativar teste. Go, Android e iOS dessa
  execução anterior estavam aprovados.
- **5 conferido:** `/healthz` respondeu `ok`; a pergunta real retornou `ok` de
  `openai/gpt-oss-120b`, com término `stop`. O validador rejeitou HTML, erro, resposta vazia
  e resposta pronta do app. Não envia dados financeiros. Execução diária às 12h17 UTC.
- **6 feito:** Consultor unificado em `0af70a2`; computador, app e site usam `pages/advisor.js`.
- **8 feito:** `1fbf982` removeu 78 regras de CSS; a contagem inicial era 74.
- **9 parcial:** o lockfile já foi atualizado pelo Claude. Continuam a revisão do `uuid`
  de desenvolvimento e das licenças BlueOak declaradas por `glob` e `minimatch`; não foram
  introduzidas novas dependências nesta retomada.
- **10 feito:** removido em `8b13010` e integrado na main. Não reintroduzir `FINAN_LOCAL_*`.
- **Teste de importação isolado:** o cenário "sem IA disponível" consultava o Ollama real
  do Mac. O servidor do teste agora usa um endereço local sem serviço. O teste passou sem
  depender dos modelos instalados na máquina.

Os itens 7 e 11 continuam com as condições originais. O protótipo em
`docs/rich-mais-recente/`, os arquivos gerados do Capacitor e os worktrees foram preservados.

**Validação final desta retomada:** `make test`, as dez suítes Chromium de `make e2e`
e as nove suítes de `PORT_OFFSET=2200 npm run webkit` passaram. A importação Chromium foi
repetida após a espera nova e passou. Licenças, `npm audit` do e2e e validação do workflow
com `actionlint` também passaram. Revisões independentes de código, segurança e testes
não encontraram bloqueadores. A primeira execução da matriz no GitHub ainda será
acompanhada após o push. A execução `41749749940` expôs uma medição da caixa de mensagem
antes de ela aparecer no Chromium; o teste agora aguarda a caixa ficar visível. O WebKit
Linux registrou `TypeError: Load failed`, sem nova queda nativa; o diagnóstico agora inclui
o endereço e a pilha no momento do erro. A aprovação remota continua pendente.
Android real, Safari real e login Google real continuam pendentes.
Os testes direcionados de Ollama e configuração já tinham passado com `-race`.

### Riscos

| Risco | Efeito | Como fica |
|---|---|---|
| Computador antigo recebe uma transferência do celular novo | recusa o registro | o app já pula tipos desconhecidos e mostra "atualize a Rich do computador" (o texto deixa de falar só de notas) |
| Duas rodadas de testes no navegador ao mesmo tempo | erro de porta em uso | uma rodada de cada vez; o agente roda só as suítes dele |
| Uma regressão de navegador volta no CI | job vermelho | motores fixados pelo Playwright; ambos obrigatórios, sem ignorar erros |
