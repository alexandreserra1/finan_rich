# Consultor grátis sem chave (intermediário na Vercel)

Para quem não quer criar conta em provedor de IA, o app pode oferecer a opção
**"finan_rich grátis (sem chave)"**. O celular manda a pergunta (já com os nomes trocados por
apelidos, ver [MOBILE.md](MOBILE.md)) para um intermediário pequeno, que repassa ao
[Groq](https://groq.com) com uma chave que fica só no servidor. Nenhuma chave vai dentro do
APK.

```
celular (anonimiza) ──HTTPS──> intermediário (Vercel, chave numa variável secreta) ──> Groq
```

- **Código**: `internal/proxy` (regras e limites, com testes). Duas entradas para o mesmo
  código: `api/index.go` (função da Vercel, com `vercel.json` e `.vercelignore`) e
  `cmd/finanproxy` (programa comum, para o Cloud Run ou qualquer servidor).
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
- Limite por aparelho (30 chamadas por hora, 80 por dia) e por rede (60 por hora, 200 por dia).
  Uma pergunta usa de 2 a 5 chamadas.
- O modelo e o tamanho máximo da resposta são escolhidos no servidor.
- Quando um modelo esgota a cota grátis (erro 429), tenta o próximo da lista: no Groq cada
  modelo tem a sua cota.
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

**Capacidade**: o plano grátis do Groq conta por conta (não por chave), com algo como 1.000
chamadas por dia por modelo e limites de tokens por minuto e por dia (confira em
console.groq.com/settings/limits). Com 4 modelos, dá algumas centenas de perguntas por dia
somando todo mundo. Para mais, cada pessoa pode usar a própria chave grátis em Ajustes.

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
