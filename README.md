# finan_rich

Controle financeiro pessoal em Go: lançamentos, divisão do salário (50/30/20 ou a sua),
dívidas e um **consultor com IA** (Claude) que conversa sobre os seus números reais.

## Funcionalidades

- **Painel do mês**: entradas, saídas e saldo, quanto você guardou, divisão do salário
  (planejado x realizado por grupo), gastos por categoria e visão anual com gráfico.
- **Lançamentos**: entradas e saídas por categoria. O botão *Copiar do mês anterior* repete
  salário e contas fixas sem duplicar.
- **Categorias e divisão**: cada categoria pertence a um grupo (Renda, Necessidades,
  Desejos ou Investimentos). Os percentuais de cada grupo são configuráveis.
- **Dívidas**: saldo, juros ao mês e parcela mínima. Mostra quanto tempo e quanto de juros
  leva para quitar pagando só o mínimo, alerta quando o mínimo não cobre os juros e
  sugere a ordem de pagamento pelos métodos avalanche e bola de neve.
- **Consultor IA**: chat com streaming. A cada pergunta o app envia um retrato atualizado
  das suas finanças, e o histórico da conversa fica salvo no banco.

## Rodando

Pré-requisitos: Go 1.25+ e Docker (para o PostgreSQL).

```bash
cp .env.example .env      # coloque sua ANTHROPIC_API_KEY (opcional)
make run                  # sobe o Postgres no Docker e roda o app
```

Depois é só abrir http://127.0.0.1:8080. As tabelas são criadas automaticamente na
primeira execução. Sem `ANTHROPIC_API_KEY`, tudo funciona menos o chat.

Para rodar tudo em containers: `make docker`. Para fazer backup do banco: `make backup`.

A chave da API é gerada em https://platform.claude.com/ (o uso é cobrado por token).

## Estrutura

```
cmd/finanrich/        main: lê a config, conecta no banco, sobe o HTTP
internal/config/      variáveis de ambiente
internal/money/       valores em centavos (int64) e formatação "R$ 1.234,56"
internal/store/       PostgreSQL (pgx) + migrations SQL embutidas
internal/finance/     regras de negócio puras: resumo do mês, simulação de dívidas
internal/advisor/     integração com o Claude (SDK oficial da Anthropic)
internal/web/         handlers HTTP, templates html/template, CSS e o JS do chat
```

Decisões para facilitar a manutenção:

- **Só a stdlib + 2 dependências** (`pgx` e `anthropic-sdk-go`). As rotas usam o
  `http.ServeMux` do Go 1.22+, e as páginas são renderizadas no servidor. O único
  JavaScript é o do chat.
- **Dinheiro em centavos** (`BIGINT`) para evitar erro de arredondamento. Juros ficam em
  pontos-base (1,99% = 199).
- **Migrations**: para mudar o schema, crie `internal/store/migrations/002_algo.sql`.
  Elas rodam em ordem, uma vez cada, ao iniciar o app. Não edite uma que já rodou.
- **Regras em `finance/`** sem banco nem HTTP, então são fáceis de testar.

## Consultor IA

- Modelo padrão: `claude-opus-5-5` (troque com `CLAUDE_MODEL`).
- O system prompt está em `internal/advisor/advisor.go` e o retrato dos dados enviado a
  cada pergunta em `internal/advisor/snapshot.go`. Para o consultor enxergar algo novo
  (metas, por exemplo), basta incluir no snapshot.
- O histórico é guardado exatamente como foi enviado e reenviado sem alterações. Isso
  aproveita o cache de prompt (as perguntas seguintes ficam mais baratas). *Nova conversa*
  apaga o histórico.
- Se o filtro de segurança da API recusar uma pergunta por engano, a própria API tenta
  outro modelo automaticamente (`fallbacks: "default"`).

## Testes

```bash
make test
```

Os testes do consultor usam um servidor falso que imita a API da Anthropic, então não
precisam de chave nem geram custo.

## Segurança

O app foi feito para uso individual e **não tem login**. Por padrão ele escuta só em
`127.0.0.1`. Não o exponha na internet sem antes colocar autenticação (por exemplo, um
proxy reverso com senha).
