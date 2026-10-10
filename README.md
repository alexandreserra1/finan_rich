# finan_rich

Controle financeiro pessoal que roda no seu computador: lançamentos, divisão do salário
(50/30/20 ou a sua), dívidas e um **consultor com IA grátis** que conversa sobre os seus
números reais. Seus dados ficam num arquivo no seu PC, sem conta e sem nuvem.

- **Painel do mês**: entradas, saídas, saldo, quanto você guardou, divisão do salário
  (planejado x realizado), gastos por categoria e visão anual.
- **Lançamentos** por categoria. O botão *Copiar do mês anterior* repete salário e contas fixas.
- **Dívidas**: quanto tempo e quanto de juros para quitar pagando o mínimo, alerta quando o
  mínimo não cobre os juros e a ordem de pagamento pelos métodos avalanche e bola de neve.
- **Consultor IA**: tire dúvidas sobre dívidas, cortes de gastos e reserva de emergência.
  Por padrão usa modelos abertos (Qwen, DeepSeek, Llama...) rodando no seu PC pelo Ollama.

## Como usar

### 1. Baixe o finan_rich

**Windows**: baixe o `.zip` de Windows na página de
[Releases](https://github.com/alexandreserra1/finan_rich/releases), extraia e dê dois
cliques em `finanrich.exe`. Se o Windows avisar sobre "editor desconhecido", clique em
*Mais informações → Executar assim mesmo*.

**macOS e Linux**: no Terminal, rode:

```bash
curl -fsSL https://raw.githubusercontent.com/alexandreserra1/finan_rich/main/scripts/install.sh | sh
finanrich
```

O navegador abre sozinho em http://127.0.0.1:8080. Para fechar, feche a janela do
terminal (ou aperte Ctrl+C).

### 2. (Opcional) Ative o consultor com IA

A opção recomendada é grátis, sem cadastro e privada:

1. Instale o **Ollama**: https://ollama.com/download
2. No finan_rich, abra **Configurações** e clique em **Baixar modelo agora** (download único
   de ~2,5 GB do modelo `qwen3:4b`).

Pronto. O resto do app funciona mesmo sem IA.

Qual modelo escolher no Ollama:

| Memória do PC | Modelo sugerido | Observação |
|---|---|---|
| 8 GB | `qwen3:4b` (padrão) | Bom em português, leve |
| 16 GB | `qwen3:8b` | Respostas melhores |
| 32 GB ou mais | `qwen3:14b` | O melhor dos três |

Não quer instalar nada? Em **Configurações** dá para usar uma IA na nuvem:

| Provedor | Custo | Chave |
|---|---|---|
| Groq | Plano grátis | Grátis em console.groq.com |
| OpenRouter | Modelos `:free` grátis | Grátis em openrouter.ai |
| DeepSeek | Centavos por conversa | platform.deepseek.com |
| Outro compatível com OpenAI | Depende | LM Studio, vLLM, OpenAI... |
| Anthropic Claude | Pago | platform.claude.com |

Na nuvem, o resumo das suas finanças é enviado ao provedor junto com cada pergunta.

### Seus dados e backup

Tudo fica no arquivo `finan.db`, cujo caminho aparece no terminal ao abrir o app:

- **Windows**: `%AppData%\finan_rich\finan.db`
- **macOS**: `~/Library/Application Support/finan_rich/finan.db`
- **Linux**: `~/.config/finan_rich/finan.db`

Para fazer backup, copie esse arquivo com o app fechado.

### Docker (alternativa)

```bash
docker compose up -d     # http://localhost:8080
```

O app procura o Ollama instalado no seu sistema. No Mac, prefira o Ollama nativo: dentro
do Docker ele não usa a GPU. Veja os comentários em `docker-compose.yml`.

## Desenvolvimento

Requer Go 1.26+.

```bash
make run     # roda o app
make test    # gofmt + go vet + testes
make build   # executáveis de todos os sistemas em dist/
```

### Estrutura

```
cmd/finanrich/      main: flags, abre o navegador, sobe o servidor
internal/config/    flags e variáveis de ambiente
internal/money/     valores em centavos (int64) e formatação "R$ 1.234,56"
internal/store/     SQLite (database/sql) + migrations embutidas
internal/finance/   regras de negócio puras: resumo do mês, simulação de dívidas
internal/llm/       interface Provider + Ollama, APIs compatíveis com OpenAI e Anthropic
internal/advisor/   prompt do consultor e retrato financeiro enviado à IA
internal/web/       handlers HTTP, templates, CSS e JS
```

As dependências apontam para dentro: `web` usa `advisor`, `llm`, `finance` e `store`;
`finance` e `llm` não conhecem HTTP nem banco.

### Decisões

- **Um executável só.** SQLite com driver em Go puro (`modernc.org/sqlite`, sem CGO), e
  templates, CSS e JS embutidos com `embed`. Compila para qualquer sistema sem dependências.
- **Poucas dependências**: o driver SQLite e o SDK da Anthropic. O resto é stdlib: rotas com
  `http.ServeMux` e páginas com `html/template`. O único JavaScript é o do chat e o das
  configurações.
- **Dinheiro em centavos** para evitar arredondamento. Juros em pontos-base (1,99% = 199).
- **Migrations**: para mudar o schema, crie `internal/store/migrations/002_algo.sql`. Elas
  rodam em ordem, uma vez cada, ao abrir o app. Nunca edite uma que já foi publicada.
- **Segurança**: sem login, pois é uso individual. Por isso o app só escuta em `127.0.0.1` e
  recusa formulários vindos de outros sites (CSRF).

### Como estender

- **Novo provedor de IA**: implemente `llm.Provider` (um método `Chat`), registre em
  `llm.New` e, se quiser que apareça na tela, adicione um `llm.Preset`.
- **A IA enxergar algo novo** (metas, por exemplo): inclua no texto de
  `internal/advisor/snapshot.go`.
- **Nova página**: crie `internal/web/templates/nome.html` com `{{define "content"}}`,
  um handler e a rota em `server.go`. Arquivos com `_` no início são parciais compartilhados.

### Testes

`make test` roda tudo sem rede: o banco é SQLite em memória e os provedores de IA são
testados contra servidores falsos que imitam o Ollama e as APIs da OpenAI e da Anthropic.

### Publicar uma versão

```bash
git tag v1.0.0 && git push origin v1.0.0
```

O GitHub Actions roda os testes e o GoReleaser publica os executáveis em Releases.
