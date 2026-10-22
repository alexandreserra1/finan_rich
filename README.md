# finan_rich

Controle financeiro pessoal que roda no seu computador: lançamentos, divisão do salário
(50/30/20 ou a sua), dívidas e um **consultor com IA grátis** que conversa sobre os seus
números reais. Seus dados ficam num arquivo no seu PC, sem conta e sem nuvem.

- **Painel do mês**: entradas, saídas, saldo, quanto você guardou, divisão do salário
  (planejado x realizado), gastos por categoria e visão anual.
- **Avisos no painel**: dívida vencendo, dívida que cresce, grupo acima da meta do mês,
  gasto fora do padrão e meta atrasada ou atingida. No app do celular, lembrete de
  vencimento das dívidas mesmo sem internet.
- **Lançamentos** por categoria, com *Repetir todo mês* (salário, aluguel, assinaturas entram
  sozinhos), *Copiar do mês anterior* e busca com filtros (texto sem ligar para acento,
  categoria, tipo, faixa de valor, um mês ou todos) que soma o que achou.
- **Orçamento por categoria**: um teto por mês em cada categoria, com barra no painel e
  aviso aos 80% e ao estourar.
- **Primeiro uso guiado**: na primeira vez, o painel mostra um passo a passo (quanto ganha,
  divisão do salário, dívidas e uma meta).
- **Metas** com valor e prazo: quanto já juntou e quanto guardar por mês para chegar lá.
- **Análise de gastos**: gastei onde (iFood, Uber, mercado…), ritmo do mês contra o mês
  passado com a projeção de como fecha, quanto ainda dá para gastar por dia, categorias
  contra a sua média, assinaturas com o custo anual e onde dá para economizar
  ([docs/ANALISE.md](docs/ANALISE.md)).
- **Importar extrato** do banco (OFX, CSV ou a fatura do cartão em PDF: Nubank, Itaú, BB,
  Bradesco, Inter, Caixa...):
  sugere a categoria de cada lançamento pelo seu histórico e pela descrição (iFood, Uber,
  Netflix...), aprende com as suas correções, não duplica o que já foi importado, avisa o
  que parece já lançado à mão e deixa de fora o pagamento da fatura do cartão. Para o que
  sobrar sem categoria, a IA pode sugerir (recebe só as descrições).
- **Dívidas**: quanto tempo e quanto de juros para quitar pagando o mínimo, alerta quando o
  mínimo não cobre os juros e a ordem de pagamento pelos métodos avalanche e bola de neve.
- **Notas**: metas, combinados e ideias, com busca e notas fixadas. O consultor lê as mais
  recentes para entender seus planos.
- **Exportar**: planilha do Excel (.xlsx) com abas de resumo, lançamentos, categorias,
  divisão do salário, dívidas e notas; relatório em PDF; CSV; e cada nota em PDF ou texto.
  Os arquivos são gerados no próprio aparelho, mesmo sem internet.
- **Consultor IA**: tire dúvidas sobre dívidas, cortes de gastos, metas e reserva de
  emergência; pergunte "quanto gastei com iFood este ano?"; salve respostas como nota.
  Ele busca lançamentos de qualquer período por conta própria e faz o que você pedir por
  mensagem: lançar ("lança 30 reais do cinema"), corrigir ("o Uber de ontem foi 32,50"),
  apagar, criar metas, cadastrar dívidas, guardar dinheiro numa meta ("guardei 500 na
  viagem") e pagar dívidas ("paguei 500 do cartão", que baixa o saldo e lança o gasto). Cada mudança vira um cartão com um botão
  (**Adicionar**, **Alterar**, **Apagar**): nada muda sem você tocar nele, e os cartões
  ficam na conversa. Também dentro do app do celular, inclusive **sem computador**: o
  celular chama uma IA na nuvem com plano grátis (Groq, Gemini) com os nomes de lojas,
  dívidas e metas trocados por apelidos antes de sair, e a chave guardada cifrada.
  Por padrão usa modelos abertos (Qwen, DeepSeek, Llama...) rodando no seu PC pelo Ollama.

## Como usar

### 1. Instale o finan_rich (um comando)

**Windows**: abra o **PowerShell** (menu Iniciar → digite *PowerShell*) e cole:

```powershell
irm https://raw.githubusercontent.com/alexandreserra1/finan_rich/main/scripts/install.ps1 | iex
```

**macOS e Linux**: abra o **Terminal** e cole:

```bash
curl -fsSL https://raw.githubusercontent.com/alexandreserra1/finan_rich/main/scripts/install.sh | sh
```

Pronto: o finan_rich aparece no menu Iniciar e na Área de Trabalho (Windows), em
Aplicativos e no Launchpad (macOS) ou no menu de aplicativos (Linux). Ele abre em janela
própria, como um programa (usa o Edge, Chrome, Brave ou Chromium que você tiver; sem
nenhum deles, abre no navegador padrão). Para fechar: **Ajustes → Fechar o finan_rich**.

- **Atualizar**: rode o mesmo comando de novo. Seus dados ficam onde estão. Quando sai
  versão nova, o finan_rich avisa no topo da tela, já com o comando. Para isso ele consulta
  uma vez por dia o endereço `github.com/.../releases/latest` (nenhum dado seu vai junto);
  `FINAN_UPDATE_CHECK=0` desliga.
- **Desinstalar** (os dados ficam): no Terminal,
  `curl -fsSL https://raw.githubusercontent.com/alexandreserra1/finan_rich/main/scripts/install.sh | sh -s -- --uninstall`;
  no PowerShell,
  `& ([scriptblock]::Create((irm https://raw.githubusercontent.com/alexandreserra1/finan_rich/main/scripts/install.ps1))) -Uninstall`.
- **Prefere um gerenciador de pacotes?**
  - Homebrew (macOS e Linux):
    `brew tap alexandreserra1/finan_rich https://github.com/alexandreserra1/finan_rich && brew trust alexandreserra1/finan_rich && brew install --cask finan_rich`
    (atualiza com `brew upgrade`; pelo brew o programa abre pelo terminal, com `finanrich`).
  - Scoop (Windows):
    `scoop bucket add finan_rich https://github.com/alexandreserra1/finan_rich; scoop install finan_rich`
    (cria o atalho no menu Iniciar; atualiza com `scoop update finan_rich`).
- O instalador confere o arquivo baixado com o `checksums.txt` da versão antes de instalar.
  Instalando pelo terminal, o Windows e o macOS não mostram o aviso de "editor
  desconhecido" (que só aparece em arquivo baixado pelo navegador).
- Prefere baixar à mão? Os arquivos estão em
  [Releases](https://github.com/alexandreserra1/finan_rich/releases): extraia e abra o
  `finanrich` (`finanrich.exe` no Windows).

### 2. (Opcional) Ative o consultor com IA

A opção recomendada é grátis, sem cadastro e privada:

1. Instale o **Ollama**: https://ollama.com/download
2. No finan_rich, abra **Configurações** e clique em **Baixar modelo agora** (download único
   de ~6,6 GB do modelo `qwen3.5:9b`, da Alibaba).

Pronto. O resto do app funciona mesmo sem IA.

Qual modelo escolher no Ollama:

| Memória do PC | Modelo | Observação |
|---|---|---|
| 16 GB ou mais | `qwen3.5:9b` (padrão) | Raciocina melhor |
| 8 GB | `qwen3.5:4b` | Bom em português, mais leve |
| Pouca memória | `qwen3.5:2b` | Mais simples, mas roda em quase tudo |

Quer o mais inteligente sem pesar no PC? Escolha **Ollama Cloud** em Configurações: modelos
gigantes (Qwen3.5, Kimi, GLM) nos servidores do Ollama, grátis com uma conta e limite de
uso. Rode `ollama signin` uma vez para entrar.

**Como o consultor acerta as contas:** modelos pequenos entendem bem a situação, mas erram
aritmética. Por isso o app faz todas as contas (juros de cada dívida, sobra depois das
parcelas, reserva de emergência, prazo para quitar) e entrega prontas ao modelo, que só
interpreta e aconselha. Em testes com dados reais, isso levou o `qwen3.5:9b` de "erra quase
toda conta" para acertar prazos e valores. Planos de divisão livres, montados pelo próprio
modelo, ainda podem ter somas erradas: confira os números antes de seguir.

Não quer instalar nada? Em **Configurações** dá para usar uma IA na nuvem:

| Provedor | Custo | Chave |
|---|---|---|
| Groq | Plano grátis | Grátis em console.groq.com |
| OpenRouter | Modelos `:free` grátis | Grátis em openrouter.ai |
| DeepSeek | Centavos por conversa | platform.deepseek.com |
| Outro compatível com OpenAI | Depende | LM Studio, vLLM, OpenAI... |
| Anthropic Claude | Pago | platform.claude.com |

Na nuvem, o resumo das suas finanças é enviado ao provedor junto com cada pergunta.

### Sem internet (modo offline)

Cada aparelho (computador ou celular) guarda uma cópia dos seus dados. Sem conexão com o
computador onde o finan_rich roda, o app continua funcionando: você consulta, lança,
edita e apaga normalmente. As alterações ficam numa fila e são enviadas sozinhas quando a
conexão volta; o indicador no topo mostra *Sincronizado*, *Offline · N pendente(s)* etc.
Se o mesmo item for alterado em dois aparelhos, vale a alteração mais recente.

O que precisa do computador ligado: o **Consultor IA** (a IA roda nele) e os **Ajustes**.

- **No computador**, com o app instalado (botão *Instalar app*), ele abre mesmo com o
  finan_rich fechado.
- **No celular pelo Wi-Fi** (`http://192.168...`), os dados ficam no celular e o app aguenta
  a conexão cair enquanto estiver aberto. Para **abrir o app sem conexão** e sincronizar
  **de fora de casa**, use o Tailscale (grátis): veja *Ajustes → Usar no celular sem
  conexão*. Ele dá ao seu computador um endereço `https://` acessível só pelos seus aparelhos.

### No celular

**App Android** (novo): baixe o `.apk` da [última versão](https://github.com/alexandreserra1/finan_rich/releases/latest),
instale e, no computador, vá em **Ajustes → App do celular → Parear um celular**. Aponte a
câmera para o QR code e confirme no app. O app abre sem internet, guarda os dados no
celular e sincroniza com o computador pelo Wi-Fi. Detalhes em [docs/MOBILE.md](docs/MOBILE.md).

**Pelo navegador** (Android ou iPhone, sem instalar nada): o finan_rich roda no seu
computador, e o celular acessa pelo Wi-Fi de casa:

1. No computador, abra **Ajustes → Acesso pelo celular**, crie uma senha e clique em
   **Ligar acesso pelo celular**.
2. Aponte a câmera do celular para o **QR code** que aparece (ou digite o endereço,
   algo como `http://192.168.0.10:8080`) e entre com a senha.
3. Para ter o ícone na tela inicial: no Android (Chrome), menu ⋮ → *Adicionar à tela
   inicial*; no iPhone (Safari), Compartilhar → *Adicionar à Tela de Início*.

A interface se adapta ao celular (menu no rodapé, listas em cartões). Os dados e a IA
continuam no computador, que precisa ficar ligado com o finan_rich aberto. No computador
a senha não é pedida.

### Como aplicativo no computador

O instalador (passo 1) já cria o ícone e abre o finan_rich em janela própria. Se você
baixou à mão, dá para fazer o mesmo no Chrome ou no Edge: **Instalar app** no topo da
página (ou o ícone de instalar na barra de endereço).

### Seus dados e backup

Tudo fica no arquivo `finan.db`, cujo caminho aparece no terminal ao abrir o app:

- **Windows**: `%AppData%\finan_rich\finan.db`
- **macOS**: `~/Library/Application Support/finan_rich/finan.db`
- **Linux**: `~/.config/finan_rich/finan.db`

Para fazer backup, use **Exportar e backup** (no menu Ajustes ou em `/exportar`):

- **Backup (.json)**: todos os seus registros num arquivo, em qualquer aparelho, mesmo sem
  internet. *Restaurar backup* traz de volta tudo que estava nele (inclusive o que foi
  apagado depois) e mantém o que você criou depois; os outros aparelhos recebem na
  sincronização.
- **Banco completo (.db)**: só no computador, inclui também a configuração da IA e a conversa
  com o consultor. Para restaurar, feche o app e troque o arquivo `finan.db` por ele.

Para analisar no Python: `pd.read_excel("finan_rich_2026.xlsx", sheet_name="Lançamentos")`
ou `pd.read_csv("finan_rich_lancamentos_2026.csv", sep=";", decimal=",")`.

### Docker (alternativa)

```bash
docker compose up -d     # http://localhost:8080
```

O app procura o Ollama instalado no seu sistema. No Mac, prefira o Ollama nativo: dentro
do Docker ele não usa a GPU. Veja os comentários em `docker-compose.yml`.

### Num servidor (acesso de qualquer lugar)

Para usar fora de casa, rode o finan_rich num servidor (uma VPS, por exemplo) e **defina
uma senha** com `FINAN_PASSWORD`: com ela, todo acesso pede login, inclusive o local.

```bash
FINAN_PASSWORD='uma-senha-forte' docker compose up -d
```

Use HTTPS na frente (um proxy reverso como Caddy, ou o Tailscale para acessar só dos seus
aparelhos, sem expor nada na internet). Num servidor geralmente não há GPU para o Ollama:
use um provedor na nuvem (Ollama Cloud, Groq, OpenRouter...) em Ajustes.

## Desenvolvimento

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): como o projeto se organiza, cada pasta e
  arquivo, e como as peças se conectam (sincronização, WebAssembly, consultor IA).
- [docs/ROADMAP.md](docs/ROADMAP.md): próximos passos, incluindo o app mobile.

Requer Go 1.26+ e, para os testes do navegador, Node 22+.

```bash
make run     # gera as regras para o navegador (finance.wasm) e roda o app
make test    # gofmt, vet, testes do Go e das regras como WebAssembly
make e2e     # testes no navegador: offline e sincronização entre aparelhos
make build   # executáveis de todos os sistemas em dist/
```

O `finance.wasm` não fica no git: é gerado por `make wasm` (ou `go generate ./internal/web`).
Sem ele o app não sobe e avisa o que fazer.

### Estrutura

```
cmd/finanrich/      main: flags, abre o navegador, sobe o servidor
cmd/finanwasm/      as regras compiladas para WebAssembly (roda no navegador)
internal/domain/    modelos (lançamento, dívida...), validação e Ledger — Go puro
internal/finance/   regras de negócio puras: resumo do mês, dívidas, reserva, views
internal/money/     valores em centavos (int64) e formatação "R$ 1.234,56"
internal/report/    exportação: .xlsx, CSV, PDF e backup .json, em Go puro (roda no aparelho)
internal/importer/  importação de extratos OFX/CSV: leitura, categoria sugerida e duplicados
internal/wasmapi/   ponte JSON entre o navegador e as regras
internal/store/     SQLite: registros sincronizáveis, chat, sessões, configurações
internal/llm/       interface Provider + Ollama, APIs compatíveis com OpenAI e Anthropic
internal/advisor/   prompt do consultor e retrato financeiro enviado à IA
internal/auth/      senha (PBKDF2), tokens de sessão e limite de tentativas
internal/web/       HTTP, /api/sync, pareamento, templates, service worker e o app do aparelho (static/js)
tools/wasmbuild/    gera static/finance.wasm (go generate ./internal/web)
tools/mobilebuild/  gera mobile/www, as telas que vão dentro do app do celular
mobile/             app Android (Capacitor): veja docs/MOBILE.md
e2e/                testes no navegador: offline, sincronização entre aparelhos e app do celular
```

### Como funciona o modo offline

```
 aparelho (navegador)                               computador (finanrich)
 ┌──────────────────────────────┐   /api/sync    ┌──────────────────────────┐
 │ telas em JS  ← regras em Go  │ ─────────────▶ │ SQLite: records          │
 │ (static/js)    (finance.wasm)│ ◀───────────── │ (documentos JSON + rev)  │
 │ IndexedDB: dados + fila      │  envia a fila, │ Consultor IA (Ollama)    │
 │ service worker: abre offline │  recebe o novo │                          │
 └──────────────────────────────┘                └──────────────────────────┘
```

- Todo registro tem um **UUID** criado no aparelho (funciona offline), `updatedAt` e
  `deleted` (apagar também sincroniza). Em conflito, vence o `updatedAt` maior — a mesma
  regra no servidor (`store.ApplyChanges`) e no aparelho (`static/js/db.js`).
- O servidor numera cada gravação (`rev`); o aparelho pede "o que mudou depois da rev N".
- As regras (resumo, dívidas, validação) existem **uma vez**, em Go, e rodam no servidor e no
  navegador (WebAssembly). Os testes delas rodam nos dois (`make test`).

### Decisões

- **Um executável só.** SQLite com driver em Go puro (`modernc.org/sqlite`, sem CGO), e
  templates, CSS e JS embutidos com `embed`. Compila para qualquer sistema sem dependências.
- **Poucas dependências**: o driver SQLite e o SDK da Anthropic. O resto é stdlib: rotas com
  `http.ServeMux` e páginas com `html/template`. O único JavaScript é o do chat e o das
  configurações.
- **Dinheiro em centavos** para evitar arredondamento. Juros em pontos-base (1,99% = 199).
- **Migrations**: para mudar o schema, crie `internal/store/migrations/005_algo.sql`. Elas
  rodam em ordem, uma vez cada, ao abrir o app. Nunca edite uma que já foi publicada.
- **Segurança**: no próprio computador não há login. Por padrão o app só escuta em
  `127.0.0.1`. Ao ligar o acesso pelo celular, ele passa a escutar na rede e exige senha de
  quem vem de outro aparelho (PBKDF2-SHA256, sessões com cookie `HttpOnly`, limite de
  tentativas). Também recusa formulários de outros sites (CSRF) e não pode ser aberto
  dentro de outro site (`X-Frame-Options`).
- **PWA**: manifesto e service worker deixam o app instalável. O service worker só guarda
  os arquivos estáticos em cache; os dados sempre vêm do servidor.

### Como estender

- **Novo provedor de IA**: implemente `llm.Provider` (um método `Chat`), registre em
  `llm.New` e, se quiser que apareça na tela, adicione um `llm.Preset`.
- **Novo tipo de registro** (ex.: metas): crie o tipo em `internal/domain` com `Meta` e
  `Validate`, registre em `domain.DecodeRecord` e no `CHECK` de `records`, e monte a tela
  em `static/js/pages/`.
- **A IA enxergar algo novo** (metas, por exemplo): inclua no texto de
  `internal/advisor/snapshot.go`.
- **Nova página**: crie `internal/web/templates/nome.html` com `{{define "content"}}`,
  um handler e a rota em `server.go`. Arquivos com `_` no início são parciais compartilhados.

### Testes

`make test` roda tudo sem rede: o banco é SQLite em memória e os provedores de IA são
testados contra servidores falsos que imitam o Ollama e as APIs da OpenAI e da Anthropic.
As regras de negócio rodam duas vezes: compiladas normalmente e como WebAssembly no Node.

`make e2e` abre um Chromium de verdade e testa o ciclo completo: lançar online, desligar o
servidor, reabrir o app sem conexão, lançar e recarregar, religar e conferir que tudo
sincronizou, e um segundo aparelho recebendo (e apagando) os dados do primeiro. Também
testa o app do celular: pareamento por código, sincronização com token, uso offline e
aparelho desconectado no computador.

### Publicar uma versão

```bash
git tag v1.0.0 && git push origin v1.0.0
```

Ou pelo site: **Actions → Release → Run workflow**, informando a versão. O GitHub Actions
roda os testes, cria a tag e o GoReleaser publica os executáveis em Releases. Em seguida o
APK do Android é compilado e anexado à mesma versão (veja [docs/MOBILE.md](docs/MOBILE.md)
para assiná-lo com a chave do projeto).

## Licença

[MIT](LICENSE): pode usar, modificar e distribuir livremente, inclusive comercialmente, mantendo o aviso de copyright.

Todas as dependências (do programa e do app do celular) também são código aberto; a lista
e as licenças estão em [docs/LICENSES.md](docs/LICENSES.md), e o CI confere a cada mudança.
Como o app chega ao celular sem pagar loja (IzzyOnDroid, F-Droid e iPhone pelo Safari):
[docs/DISTRIBUICAO.md](docs/DISTRIBUICAO.md).
