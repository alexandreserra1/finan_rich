# Plano: acesso seguro ao site e IA que não para

Lista de trabalho combinada em 28/10/2026. Cada item sai com teste e é marcado
aqui quando entra na main. Vale para o computador, o site e o app do celular,
salvo onde está escrito.

## A. Segurança do site (login obrigatório)

O site pode ser aberto num computador de outra pessoa. Os dados ficam no
navegador (nenhum servidor nosso guarda dados), então a proteção é: nada do
app aparece sem login, e nenhum dado fica no navegador sem conta.

- [x] A1. Barreira de login: sem sessão Google, qualquer endereço do app (`/`,
  `/dividas`, `/lancamentos`…) mostra só a landing ou o login, nunca uma tela
  do app. Vale também para endereço digitado ou favorito.
- [x] A2. Tirar o "Usar sem conta" do site (landing, boas-vindas e Ajustes).
  No computador e no celular continua: o aparelho é da pessoa.
- [x] A3. Dados antigos de quem usou sem conta: ficam guardados, mas só
  aparecem depois de entrar (aí se juntam com o Drive).
- [x] A4. Sessão que expira: sem uso por um tempo, o site pede para entrar de
  novo. Sair continua salvando no Drive e apagando tudo do navegador.
- [x] A5. Enquanto o Google não está configurado na Vercel, o site mostra a
  landing e o aviso de que o login abre em breve (nenhuma tela do app).
- [x] A6. Testes: endereço direto sem login cai no login; com login entra;
  sair e recarregar volta à landing; nenhum "Usar sem conta" no site.
- Pendente com o Alexandre: cadastrar o app no Google e pôr
  `GOOGLE_CLIENT_ID` e `GOOGLE_CLIENT_SECRET` na Vercel (`docs/GOOGLE.md`).
- Feito em 28/10/2026 (commit desta marca). A3 ficou assim: os dados antigos de quem
  usou sem conta continuam no navegador, mas nada aparece sem entrar; depois do login, juntam
  com o Drive. O ícone da pantera: landing sem login, Painel com login.
- Celular: o app já tem o bloqueio com digital ou senha (`lock.js`); conferir
  se ele vale também na volta do segundo plano (ainda não conferido).

Por que não "criptografar a URL": o endereço não carrega segredo nenhum; os
dados estão no navegador. O que protege é a barreira de login, não ter dado
sem conta e a sessão que expira.

## B. IA que não para (gastando pouco)

A IA só escreve o texto: toda conta é do Go. A meta é que a falta de token
nunca deixe a pessoa sem resposta.

- [x] B1. Resposta sem IA dos personagens: se nenhuma IA responder (cota,
  erro, sem internet), o personagem responde com a conta pronta do Go num
  texto fixo, do jeito dele. Computador, site e celular.
- [x] B2. Respostas prontas para as sugestões ("Qual dívida pagar primeiro?",
  "O que vence esta semana?"…): o Go responde direto, sem gastar token.
- [x] B3. Registro de tokens por pedido no intermediário (sem guardar texto da
  pessoa): tokens de entrada, de saída, modelo e personagem, para medir o
  custo por pessoa.
- [x] B4. Vários provedores grátis no intermediário (Groq, Gemini, Cerebras,
  Cloudflare Workers AI, OpenRouter): quando um dá limite, passa para o
  próximo. Passo a passo das chaves em `docs/IA-GRATIS.md`.
- [x] B5. Relatórios do Otávio (semana) e do Tito (mês) sem pergunta ao vivo: feito sem IA, de
  propósito. O cartão "O que o time viu" do Painel (`finance.Briefings`, em Go, zero token) e as
  notificações do celular (Otávio às segundas, Tito no dia do salário) já entregam o relatório; um
  texto escrito pela IA por cima gastaria tokens para repetir os mesmos números. Se um dia
  valer, gerar uma vez por semana e guardar no aparelho.
- [x] B6. Menos token por pergunta: cada personagem recebe só a parte do
  resumo que usa; resposta guardada para a mesma pergunta com os mesmos dados
  no mesmo dia; histórico mais curto.
- [x] B7. Aviso claro quando o grátis acaba: "use sua chave grátis do Gemini"
  com o passo a passo (a opção já existe em Ajustes).
- [ ] B8. Teto de gasto com alarme (se um dia houver reserva paga): ao bater
  no teto, volta para a resposta sem IA. Só com os números do B3 na mão.
- [ ] B9. Decisão de produto (Alexandre): plano barato só para a IA sem limite,
  ou crédito de programas para startups e código aberto.
- B3 e B4 entraram em 28/10/2026 (feitos por um agente e revisados aqui): o proxy tenta os
  provedores na ordem de `FINAN_PROVIDERS`, com no máximo 12 tentativas por pergunta, e repete
  sem `stream_options` quando o provedor recusa o campo. Nenhum provedor foi chamado de verdade
  ainda: depende das chaves na Vercel (passo a passo em `docs/IA-GRATIS.md`).
- B1, B2 e B7 entraram em 28/10/2026 (feitos por um agente e revisados aqui): `advisor.Offline`
  responde com as contas do Go quando a IA falha, as sugestões de cada personagem que só precisam
  de contas são respondidas sem IA (`advisor.Suggestions`), e o limite do grátis (429) mostra o
  caminho para a chave da própria pessoa. A resposta sem IA tem o aviso em negrito no topo.
- Verificado com as APIs de verdade em 28/10/2026: o Groq pelo intermediário publicado (com e sem
  streaming, tokens lidos de respostas reais); os endereços de Gemini, Cerebras, OpenRouter e
  Cloudflare existem e recusam chave inválida (o Gemini com 400, tratado). Falta só uma resposta
  real de cada um deles, que depende das chaves na Vercel.
- Celular: B1, B2, B5 e B6 valem no app (nuvem e IA dentro do celular); a IA
  dentro do celular já recebe as contas prontas.

## C. Redesign Esmeralda (telas que faltam)

- [x] Consultor IA (com "Os números do app" na hora, antes da resposta da IA)
- [x] Ajustes (com a escolha "IA do finan_rich" ou "minha própria chave")
- [x] Boas-vindas (login)
