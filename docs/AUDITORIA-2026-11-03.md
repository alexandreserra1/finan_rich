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

| # | Como | Arquivos | Como confere | Quem | Situação |
|---|---|---|---|---|---|
| 1 | Registro novo `transfer` (de uma conta para outra, valor, data, descrição), separado dos lançamentos: nenhum total de gasto, entrada, orçamento, análise ou IA muda. O saldo das contas e o "em aberto" do cartão passam a contar as transferências; a fatura mostra "Paga" quando os pagamentos depois do fechamento cobrem o total. Na tela Contas: "Transferir" e, no cartão, "Pagar fatura" (já com o valor da fatura). Computador antigo recusa o tipo novo: o app já pula tipos desconhecidos e avisa para atualizar. | `internal/domain`, `internal/store`, `internal/finance/accounts.go`, `js/db.js`, `js/main.js`, `js/pages/accounts.js`, `app.css` | Go: validação, saldos, fatura paga, patrimônio. e2e `accounts.test`: pagar a fatura zera o "em aberto" e não muda o Painel. Auditoria do dia 02 repetida. | eu | a fazer |
| 2 | Passo a passo do Firewall da Vercel (regra de limite por IP no `/chat/completions`), no `docs/IA-GRATIS.md`. Mexer no painel é com você. | `docs/IA-GRATIS.md` | Você cria a regra; um teste com 30 pedidos seguidos recebe 429. | você (eu escrevo o passo a passo) | a fazer |
| 3 | Sem IA salva nos Ajustes e com o modelo padrão faltando no Ollama, usar o primeiro modelo de conversa já baixado (o mesmo da lista "Usar …" de hoje). Sem nenhum modelo, nada muda (resposta pronta e o aviso de hoje). | `internal/web/aiconfig.go` | Go: Ollama falso com só `qwen2.5:7b-instruct` → o Consultor usa ele. | eu | a fazer |
| 4 | Job `e2e-webkit` no CI, depois de a rodada local no WebKit passar (se a queda do Painel aparecer, investigar antes de ligar). | `.github/workflows/ci.yml` | A rodada local; o job verde no GitHub. | eu | rodando localmente |
| 5 | Workflow diário (e "rodar agora") que chama `/healthz` e uma pergunta de teste (a mesma do botão "Testar conexão") no endereço real; falhou, o GitHub manda e-mail. | `.github/workflows/ia-diaria.yml` | Rodar o mesmo `curl` daqui; disparar o workflow à mão. | eu | a fazer |
| 6 | O computador passa a abrir `/chat` com a mesma tela do app (`pages/advisor.js`), falando com `/api/chat`; saem `chat.js`, `chat.html` e as rotas antigas (`/chat/enviar`, `/chat/limpar`). O aviso de modelo faltando continua (link para Ajustes). | `internal/web/chat.go`, `server.go`, `templates/chat.html`, `static/chat.js`, `js/main.js`, `js/pages/advisor.js`, `e2e/app.test.mjs`, `e2e/budget.test.mjs` | `go test ./internal/web`; e2e `app.test` e `budget.test` (partes do computador). | agente em paralelo (cópia isolada do repositório) | a fazer |
| 7 | Apagar a IA dentro do celular. | `js/local-ai.js`, `pages/device.js`, `advisor`, `askapi`, `mobile/package.json`, F-Droid, docs | `make test`, `make e2e`, `make mobile`. | eu, **depois da sua confirmação** | aguardando você |
| 8 | Apagar as 74 regras de CSS sem uso. | `static/app.css` | Busca dos nomes no código; `make e2e`; prints das telas. | eu | a fazer |
| 9 | `npm audit fix` em `mobile/` (sem `--force`). | `mobile/package-lock.json` | `npm audit`, `scripts/check-licenses.sh`, `make mobile`. | eu | a fazer |
| 10 | Apagar o computador como IA do intermediário. | `cmd/finanlocal`, `internal/localguard`, `scripts/finanlocal.sh`, `internal/proxy`, docs | `make test`. | eu, **depois da sua confirmação** | aguardando você |
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

### Riscos

| Risco | Efeito | Como fica |
|---|---|---|
| Computador antigo recebe uma transferência do celular novo | recusa o registro | o app já pula tipos desconhecidos e mostra "atualize a Rich do computador" (o texto deixa de falar só de notas) |
| Duas rodadas de testes no navegador ao mesmo tempo | erro de porta em uso | uma rodada de cada vez; o agente roda só as suítes dele |
| A queda do Painel no WebKit volta no CI | job vermelho | só liga o job depois de investigar a rodada local |
