# Consultor grátis sem chave (intermediário na Vercel)

Para quem não quer criar conta em provedor de IA, o app pode oferecer a opção
**"finan_rich grátis (sem chave)"**. O celular manda a pergunta (já com os nomes trocados por
apelidos, ver [MOBILE.md](MOBILE.md)) para um intermediário pequeno, que repassa a provedores
de IA grátis ([Groq](https://groq.com), Gemini, Cerebras, OpenRouter e Cloudflare Workers AI)
com chaves que ficam só no servidor. Nenhuma chave vai dentro do APK.

```
celular (anonimiza) ──HTTPS──> intermediário (Vercel, chaves em variáveis secretas) ──> Groq
                                                                   (cota acabou?) ──> Gemini ──> Cerebras ──> OpenRouter ──> Cloudflare
```

- **Código**: `cmd/api` (programa) e `internal/proxy` (regras e limites, com testes).
  O mesmo programa roda na Vercel (`vercel.json`; ela procura o programa em `cmd/api`) e no
  Cloud Run. Na Vercel ele também serve o site do finan_rich (as telas do app, pacote `site`,
  compilado com `-tags sitebundle`): tudo que não é rota da IA abre o site.
- **Por que Groq**: plano grátis sem cartão, muito rápido, modelos abertos com ferramentas, e
  ele não usa os dados da API para treinar.
- **Por que Vercel**: plano grátis (Hobby) sem cartão, login com o GitHub e publicação a cada
  push na main, sem instalar nada. Sem cartão na Vercel e no Groq, nenhum ataque vira conta:
  no pior caso a cota grátis acaba e o consultor grátis para até o dia seguinte. O intermediário
  não guarda dados, então não perde nada quando a Vercel desliga a função.
- **Limites da Vercel**: o plano Hobby é só para uso pessoal, sem fins comerciais. Se o
  finan_rich passar a cobrar, é preciso o plano Pro ou o Cloud Run (abaixo). A resposta pode
  chegar de uma vez em vez de aparecer aos poucos.

## Proteções (o app é de código aberto: qualquer um acha o endereço)

- Só passa pedido com o formato do consultor (o prompt do finan_rich) ou o teste de conexão.
- Limite por aparelho (15 chamadas por hora, 40 por dia, perto de 10 perguntas) e por rede
  (30 por hora, 100 por dia). Uma pergunta usa de 2 a 5 chamadas.
- O modelo e o tamanho máximo da resposta são escolhidos no servidor.
- O intermediário pergunta ao Groq quais modelos existem (`/models`, guardado por 1 hora) e usa,
  na ordem, os da lista de `internal/proxy` que ainda estão lá e depois os outros de conversa
  (até 6). Quando um modelo esgota a cota grátis (429), cai ou foi desligado (400/404), tenta o
  próximo: no Groq cada modelo tem a sua cota. Assim um modelo desligado pelo Groq não derruba o
  consultor nem pede versão nova do app. A lista pode ser trocada pela variável
  `FINAN_PROXY_MODELS`.
- Os `gpt-oss` vêm primeiro e vão com `reasoning_effort: low` (menos tokens de raciocínio,
  resposta mais rápida). Neles o Groq guarda sozinho o começo repetido dos pedidos, e o que vem do
  cache não conta no limite: as instruções e as ferramentas são iguais para todo mundo e, numa
  mesma pergunta, cada chamada repete a anterior.
- O IP de quem chama é o que a Vercel põe no `X-Forwarded-For` (no Cloud Run, o que o Google
  acrescenta no fim; o começo pode ser inventado); pedido sem identificação de aparelho conta como um aparelho daquela rede.
  Corpo enviado devagar é cortado em 15 s, e o pedido aceita até 128 KB. O limitador guarda
  até 100 mil aparelhos/IPs por dia; cheio, recusa gente nova até o dia seguinte (nunca zera
  quem já está barrado). Cada rede cria no máximo 5 aparelhos novos por dia (os seguintes
  dividem o limite da rede), e endereços IPv6 contam por /64: assim não dá para lotar o
  limitador inventando aparelhos ou trocando de endereço.
- Na Vercel, a contagem fica na memória da função e zera quando ela é trocada por outra; o teto
  de verdade é a cota grátis do Groq.
- No Cloud Run, roda com uma conta de serviço própria (`finanproxy@`), que só lê a chave do Groq.
- Não precisa de proxy reverso: a Vercel e o Cloud Run já têm o próprio front-end. No Cloud Run, o Google tem o front-end do Google na frente (TLS, fila,
  concorrência). Teste de carga com Groq falso: 2.000 pedidos simultâneos, todos atendidos.
- No Cloud Run: no máximo 1 instância. No pior caso, a cota grátis do Groq acaba e o consultor
  grátis para até o dia seguinte (meia-noite UTC), **sem cobrança** — desde que a conta do Groq
  não tenha cartão.

**Capacidade**: o plano grátis do Groq conta por conta (não por chave): perto de 1.000 chamadas e
200 mil tokens por dia por modelo, e 8 mil tokens por minuto (confira em
console.groq.com/settings/limits). Cada chamada do consultor leva uns 4 mil tokens (os números
do mês vão junto) e uma pergunta usa de 2 a 5 chamadas. Sem o cache, isso dá algumas dezenas de
perguntas por dia somando todo mundo; com o cache, a maior parte se repete e não conta, e a conta
sobe para perto de 200. O Groq não garante o cache, então vale olhar o uso real em
console.groq.com depois de alguns dias. Para mais, cada pessoa pode usar a própria chave grátis
em Ajustes.

## Vários provedores grátis (um esgota, passa para o próximo)

Cada provedor tem a própria cota grátis. O intermediário tenta os modelos de um provedor (na
ordem: os preferidos dele que ainda existem e depois os outros de conversa que o `/models`
dele listar, até 6, lista guardada por 1 hora) e, quando todos dão limite (429), caem (5xx) ou
foram desligados (400/404), passa para o próximo provedor. Chave errada ou sem permissão
(401/403) e provedor que nem responde pulam direto para o próximo, sem gastar tentativa nos
outros modelos dele. Só o último erro chega à pessoa.

Cada provedor só entra se a variável da chave existir na Vercel. Só com `GROQ_API_KEY`, tudo
continua como antes.

| Provedor | Variável | Endereço (formato OpenAI) | Modelos preferidos |
|---|---|---|---|
| Groq | `GROQ_API_KEY` | `https://api.groq.com/openai/v1` | `openai/gpt-oss-120b`, `qwen/qwen3.8-27b`, `openai/gpt-oss-20b` (ou `FINAN_PROXY_MODELS`) |
| Gemini | `GEMINI_API_KEY` | `https://generativelanguage.googleapis.com/v1beta/openai` | `gemini-3.8-flash`, `gemini-flash-latest`, `gemini-3.5-flash-lite` |
| Cerebras | `CEREBRAS_API_KEY` | `https://api.cerebras.ai/v1` | `gpt-oss-120b`, `qwen-3.8-27b` |
| OpenRouter | `OPENROUTER_API_KEY` | `https://openrouter.ai/api/v1` | `openrouter/free` (só modelos `:free`; os outros cobram) |
| Cloudflare Workers AI | `CLOUDFLARE_API_TOKEN` e `CLOUDFLARE_ACCOUNT_ID` | `https://api.cloudflare.com/client/v4/accounts/<id>/ai/v1` | `@cf/zai-org/glm-4.7-flash` (a Cloudflare não lista modelos no formato OpenAI, então vale só esta lista) |

- **Ordem**: a variável `FINAN_PROVIDERS` (padrão `groq,gemini,cerebras,openrouter,cloudflare`).
  Nome desconhecido ou sem chave é ignorado.
- Os modelos `gpt-oss` (de qualquer provedor) vão com `reasoning_effort: low`; os outros, sem.
- Os provedores grátis podem guardar ou usar os pedidos de forma diferente do Groq (o Gemini
  grátis pode usar os dados para melhorar o produto; no OpenRouter depende do modelo). Os nomes
  já chegam trocados por apelidos, mas vale ler os termos de cada um antes de ligar.

### Criar as contas grátis e pôr as chaves na Vercel

Não cole nenhuma chave em chat, issue ou no repositório. Para cada chave: no projeto
**finan-rich-ia** da Vercel, *Settings → Environment Variables → Add*, nome da variável, valor
da chave, ambiente *Production*, **Save**. Depois de criar todas, *Deployments → (último) → ⋯ →
Redeploy*: a Vercel só lê as variáveis novas numa publicação nova.

1. **Groq**: console.groq.com, entre com o Google (sem cartão), *API Keys → Create API Key*.
   Variável `GROQ_API_KEY`.
2. **Gemini**: aistudio.google.com, entre com o Google, *Get API key → Create API key* (pode
   criar um projeto novo). Sem cartão, fica no plano grátis. Variável `GEMINI_API_KEY`.
3. **Cerebras**: cloud.cerebras.ai, crie a conta (sem cartão), menu *API Keys → Generate API
   Key*. Variável `CEREBRAS_API_KEY`.
4. **OpenRouter**: openrouter.ai, entre (Google ou GitHub), *Keys → Create Key*. Não precisa
   pôr crédito: o intermediário só usa modelos `:free`. Variável `OPENROUTER_API_KEY`.
5. **Cloudflare Workers AI**: dash.cloudflare.com, crie a conta (sem cartão; o plano grátis
   dá uma cota diária de "neurons"). O **Account ID** aparece em *Workers AI* (ou na barra
   lateral da página inicial da conta): variável `CLOUDFLARE_ACCOUNT_ID`. Depois *Workers AI →
   Use REST API → Create a Workers AI API Token* (ou *My Profile → API Tokens → Create Token*,
   modelo "Workers AI"): variável `CLOUDFLARE_API_TOKEN`. Precisa das duas.

Para conferir: depois do Redeploy, faça uma pergunta no app e veja em *Logs* as linhas
`ia {...}` (abaixo), com o nome do provedor que respondeu. Na partida, o programa também
escreve `finanproxy ouvindo em ... (provedores: groq, gemini, ...)` (só os nomes; as chaves
nunca vão para o log).

## Troca antes de a cota acabar

O Groq informa em cada resposta quanto falta da cota daquele modelo
(`x-ratelimit-remaining-requests` por dia e `x-ratelimit-remaining-tokens` por minuto). Quando
sobram menos de 20 pedidos no dia ou menos de 6 mil tokens no minuto (uma pergunta), aquele
modelo vai para o fim da fila até a hora informada em `x-ratelimit-reset-*`, e as próximas
perguntas vão para o próximo modelo ou provedor (o computador local por último), sem esperar o
429. Um 429 com `retry-after` faz o mesmo. A pessoa não vê a troca; só a resposta pode demorar
mais quando cai no computador local. Se todos os outros falharem, o modelo cansado ainda é tentado.

## Registro de tokens por pedido

Cada resposta de um provedor vira uma linha no log, para medir o custo por pessoa e por
personagem. Exemplo:

```
ia {"provedor":"groq","modelo":"openai/gpt-oss-120b","status":200,"entrada":4120,"saida":310,"ms":1840,"personagem":"bento","aparelho":"3f9a0c12be47"}
```

- `entrada` e `saida`: tokens do campo `usage` da resposta. No streaming, o intermediário pede
  `stream_options.include_usage` ao Groq e ao Gemini (o OpenRouter manda sozinho) e lê o último
  evento depois de repassar tudo, sem atrasar a pessoa. `-1` quer dizer que o provedor não
  informou (ou que foi uma tentativa que falhou e passou para o próximo).
- `status`: o HTTP do provedor; `0` quando ele nem respondeu. Uma pergunta que pulou modelos
  tem uma linha por tentativa (ex.: um 429 e depois o 200).
- `personagem`: o nome do personagem nas instruções ("Você é o Bento…"), `consultor` (sem
  personagem), `categorize` (categorização de extrato) ou `ping` (teste de conexão).
- `aparelho`: um resumo (hash) do id do aparelho, que dá para contar pedidos por aparelho mas
  não volta ao id; vazio quando o pedido não tem id.
- Nunca vai para o log o texto da pessoa, a resposta da IA, a chave do provedor ou o id cru
  do aparelho.

**Como ver na Vercel**: projeto *finan-rich-ia → Logs*, e escreva `ia {` na busca para ver
só essas linhas (ou, no terminal, `vercel logs --environment production --query 'ia {' --since 1h`).
O Go escreve o log na saída de erro, então a Vercel mostra essas linhas com nível *Error*:
é normal. No plano Hobby a Vercel guarda só **1 hora** de log; para juntar os números do mês,
copie de tempos em tempos ou use um plano com retenção maior (Pro guarda 1 dia).

## Publicar na Vercel (uma vez; depois, cada push na main atualiza)

1. **Groq**: crie a conta em console.groq.com (login com Google, sem cartão) e gere uma chave
   em *API Keys*. Não cole a chave em chat nem no repositório.
2. **Vercel**: entre em vercel.com com o GitHub (plano Hobby, sem cartão). *Add New → Project*,
   escolha o repositório `finan_rich` e, antes de publicar, abra *Environment Variables* e crie
   `GROQ_API_KEY` com a chave do Groq. Clique em *Deploy*. O `vercel.json` já diz o que fazer;
   não precisa mudar nada nas outras opções.
3. Abra `https://<seu-projeto>.vercel.app/healthz`: deve aparecer `ok`.
4. Coloque o endereço (`https://<seu-projeto>.vercel.app`) em `FreeProxyURL`
   (`internal/llm/presets.go`) e publique uma versão: o app passa a oferecer "finan_rich grátis
   (sem chave)" em Ajustes, e o computador também.

Trocar a chave do Groq: *Settings → Environment Variables* no projeto da Vercel e depois
*Deployments → Redeploy*. Ver uso e erros: aba *Logs* do projeto.

## Alternativa: Cloud Run (Google)

Para uso comercial ou para não depender da Vercel. Pede cartão no Google, mesmo no plano
grátis.

1. **Groq**: crie a conta em console.groq.com (login com Google, sem cartão) e gere uma chave
   em *API Keys*. Não cole a chave em chat nem no repositório.
2. **Google Cloud**: crie um projeto em console.cloud.google.com e ligue o faturamento (o
   Google pede cartão mesmo para o plano grátis). Em *Faturamento → Orçamentos e alertas*, crie
   um orçamento de R$ 1 com alerta por e-mail.
3. Instale o [gcloud](https://cloud.google.com/sdk/docs/install), entre com
   `gcloud auth login` e rode na pasta do projeto:

   ```bash
   PROJECT=id-do-seu-projeto bash scripts/deploy-proxy.sh
   ```

   Na primeira vez ele liga as APIs, cria o repositório de imagens e pede a chave do Groq (que
   vai direto para o Secret Manager, sem aparecer na tela). No fim mostra o endereço, algo como
   `https://finanproxy-xxxx-uc.a.run.app`.
4. Coloque o endereço em `FreeProxyURL`, como no passo 4 da Vercel.

Trocar a chave do Groq: `printf '%s' "NOVA" | gcloud secrets versions add groq-api-key --data-file=-`
e rode o script de novo. Ver uso e erros: *Cloud Run → finanproxy → Registros*.

## Seu computador como provedor (sem cota de tokens)

Um computador com o Ollama pode emprestar a IA ao intermediário: entra por último na ordem
padrão (`local` em `FINAN_PROVIDERS`), sem cota de tokens, uma pergunta por vez. Medido num Mac
M1 Pro com o `qwen2.5:7b-instruct`: de 13 a 35 s por pergunta (com o resumo só das seções de cada
personagem; o Painel e o Consultor mostram as contas do app na hora, antes da IA). Ele vai **sem as ferramentas**
(modelo pequeno escrevia o pedido como texto); as contas prontas do Go já vão nos dados.

**Jeito rápido:** instale o Tailscale (App Store ou versão standalone) e entre com a sua conta;
depois rode `scripts/finanlocal.sh`. Ele gera a chave em `~/.config/finan_rich/local.key` (fora
do repositório), compila o porteiro, deixa o Ollama e o porteiro ligando sozinhos ao entrar no
Mac, abre o túnel, mostra o endereço e copia a chave para a área de transferência (nunca mostra
na tela). Falta só pôr as duas variáveis na Vercel. `scripts/finanlocal.sh --off` desliga tudo.

Passo a passo à mão:

1. `ollama pull qwen2.5:7b-instruct` e `OLLAMA_CONTEXT_LENGTH=8192 ollama serve`.
2. Gere uma senha longa (`openssl rand -base64 32`) e guarde **fora do repositório**.
3. `FINAN_LOCAL_KEY=<senha> go run ./cmd/finanlocal`: o porteiro escuta só neste computador
   (`127.0.0.1:11435`), pede a senha, só deixa passar a conversa e sempre com o modelo escolhido
   (o Ollama não tem senha; nunca exponha a porta 11434).
4. Túnel com endereço fixo e grátis: Tailscale Funnel (`tailscale funnel 11435`), que dá
   `https://<computador>.<rede>.ts.net`.
5. Na Vercel: `FINAN_LOCAL_URL` = `https://<computador>.<rede>.ts.net/v1` e `FINAN_LOCAL_KEY` =
   a mesma senha; Redeploy. Só o computador, sem nenhuma chave de nuvem, também funciona.

Nunca vão para o GitHub: a senha, o endereço do túnel e o `dist/finanlocal` (a pasta `dist/` é
ignorada). Com o computador desligado, o intermediário passa para o próximo provedor ou a tela
mostra a resposta sem IA.
