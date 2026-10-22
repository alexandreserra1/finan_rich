# Consultor grátis sem chave (intermediário no Cloud Run)

Para quem não quer criar conta em provedor de IA, o app pode oferecer a opção
**"finan_rich grátis (sem chave)"**. O celular manda a pergunta (já com os nomes trocados por
apelidos, ver [MOBILE.md](MOBILE.md)) para um intermediário pequeno, que repassa ao
[Groq](https://groq.com) com uma chave que fica só no servidor. Nenhuma chave vai dentro do
APK.

```
celular (anonimiza) ──HTTPS──> finanproxy (Cloud Run, chave no Secret Manager) ──> Groq
```

- **Código**: `cmd/finanproxy` (programa) e `internal/proxy` (regras e limites, com testes).
- **Por que Groq**: plano grátis sem cartão, muito rápido, modelos abertos com ferramentas, e
  ele não usa os dados da API para treinar.
- **Por que Cloud Run**: escala a zero (só liga quando alguém pergunta); o intermediário não
  guarda dados, então não perde nada ao desligar. Cabe no plano grátis do Cloud Run.

## Proteções (o app é de código aberto: qualquer um acha o endereço)

- Só passa pedido com o formato do consultor (o prompt do finan_rich) ou o teste de conexão.
- Limite por aparelho (30 chamadas por hora, 80 por dia) e por rede (60 por hora, 200 por dia).
  Uma pergunta usa de 2 a 5 chamadas.
- O modelo e o tamanho máximo da resposta são escolhidos no servidor.
- Quando um modelo esgota a cota grátis (erro 429), tenta o próximo da lista: no Groq cada
  modelo tem a sua cota.
- No Cloud Run: no máximo 1 instância. No pior caso, a cota grátis do Groq acaba e o consultor
  grátis para até o dia seguinte (meia-noite UTC), **sem cobrança** — desde que a conta do Groq
  não tenha cartão.

**Capacidade**: o plano grátis do Groq conta por conta (não por chave), com algo como 1.000
chamadas por dia por modelo e limites de tokens por minuto e por dia (confira em
console.groq.com/settings/limits). Com 4 modelos, dá algumas centenas de perguntas por dia
somando todo mundo. Para mais, cada pessoa pode usar a própria chave grátis em Ajustes.

## Publicar (uma vez; depois, o mesmo comando atualiza)

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
4. Coloque o endereço em `FreeProxyURL` (`internal/llm/presets.go`) e publique uma versão: o
   app passa a oferecer "finan_rich grátis (sem chave)" em Ajustes, e o computador também.

Trocar a chave do Groq: `printf '%s' "NOVA" | gcloud secrets versions add groq-api-key --data-file=-`
e rode o script de novo. Ver uso e erros: *Cloud Run → finanproxy → Registros*.
