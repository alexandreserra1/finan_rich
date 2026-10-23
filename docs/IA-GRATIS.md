# Consultor grátis sem chave (intermediário na Vercel)

Para quem não quer criar conta em provedor de IA, o app pode oferecer a opção
**"finan_rich grátis (sem chave)"**. O celular manda a pergunta (já com os nomes trocados por
apelidos, ver [MOBILE.md](MOBILE.md)) para um intermediário pequeno, que repassa ao
[Groq](https://groq.com) com uma chave que fica só no servidor. Nenhuma chave vai dentro do
APK.

```
celular (anonimiza) ──HTTPS──> intermediário (Vercel, chave numa variável secreta) ──> Groq
```

- **Código**: `cmd/finanproxy` (programa) e `internal/proxy` (regras e limites, com testes).
  O mesmo programa roda na Vercel (`vercel.json` e `.vercelignore`) e no Cloud Run.
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
