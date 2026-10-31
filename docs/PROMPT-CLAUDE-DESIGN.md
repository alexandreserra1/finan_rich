# Prompt para o Claude Design: redesign completo do finan_rich

Quero redesenhar **todo o frontend do finan_rich**, incluindo site público, aplicativo
financeiro, dashboards, formulários, configurações e todos os estados de uso.

Preciso de um **produto completo e navegável**, com identidade visual consistente.
Não limite a entrega ao dashboard ou a uma landing page.

## 1. O produto

O finan_rich é um aplicativo de finanças pessoais brasileiro, com funcionamento offline
e um time de consultores com IA.

Ajuda a pessoa a responder:

- Quanto entrou, quanto saiu e quanto sobrou?
- Onde estou gastando demais?
- Quanto posso gastar até o fim do mês?
- Qual dívida devo pagar primeiro?
- Como atingir minhas metas?
- O que precisa da minha atenção agora?

O público é formado por pessoas comuns, inclusive quem está endividado ou começando a
organizar a vida financeira. A experiência deve transmitir clareza e controle, sem julgamento.

Toda interface deve usar **português do Brasil**, linguagem simples, valores em reais
e datas brasileiras.

## 2. Três formas de uso

Desenhe uma experiência consistente, respeitando estas diferenças:

### Site

- Login obrigatório com Google.
- Dados financeiros ficam no navegador e sincronizam com o Google Drive da pessoa.
- Sem login, nenhuma tela financeira deve aparecer.
- Sessão expirada pede novo login, preservando os dados.
- Sair salva no Drive antes de apagar os dados locais. Se não conseguir salvar, impede
  a saída e explica o motivo.

### Programa do computador

- Funciona sem conta, com dados no próprio computador.
- Pode disponibilizar acesso protegido por senha para outros aparelhos.
- Pode executar IA local pelo Ollama.

### App do celular

- Guarda os dados no aparelho e funciona offline.
- Pode funcionar sozinho ou sincronizar com o computador.
- Pareamento por QR code ou código.
- Pode usar IA na nuvem, no computador ou dentro do próprio celular, conforme disponibilidade.
- Tem bloqueio, notificações e recursos específicos do aparelho.

Não apresente opções exclusivas do computador ou do app como se estivessem disponíveis no site.

## 3. Direção visual

Crie uma identidade nova, com acabamento de produto financeiro profissional.

Direção inicial: **esmeralda, superfícies neutras, tipografia muito legível e números com
boa hierarquia**. Pode propor uma evolução dessa direção.

Quero:

- Tema claro, escuro e automático.
- Layout próprio para computador e celular.
- Densidade equilibrada: informações úteis sem parecer uma planilha apertada.
- Destaque para a situação financeira e a próxima ação.
- Gráficos fáceis de interpretar.
- Personagens integrados com discrição.
- Ícones consistentes.
- Animações curtas e funcionais.
- Estados de foco, seleção, carregamento e erro bem definidos.

Evite excesso de cartões iguais, efeitos decorativos, gradientes chamativos e textos pequenos.

Você pode reorganizar a navegação e os componentes, mas deve preservar todas as funcionalidades
descritas.

## 4. Navegação e estrutura global

Inclua acesso claro a:

1. Painel.
2. Lançamentos.
3. Contas e cartões.
4. Dívidas.
5. Metas.
6. Análise.
7. Categorias e orçamento.
8. Consultor.
9. Notas.
10. Importar.
11. Exportar e backup.
12. Ajustes.

No celular, priorize os destinos mais usados e organize os demais em uma área acessível.

Componentes globais:

- Navegação com identificação da tela atual.
- Seletor de mês nas telas que dependem dele.
- Ação rápida de novo lançamento.
- Sininho de avisos.
- Estado de sincronização e quantidade de alterações pendentes.
- Confirmações, mensagens de erro e ação Desfazer onde existir.
- Identificação da conta conectada, quando aplicável.

## 5. Site público, login e primeiro uso

### Landing page

Apresente:

- Proposta de valor.
- Prévia realista do aplicativo.
- Como funciona.
- Privacidade e localização dos dados.
- Funcionamento offline e suas condições.
- Consultor e personagens.
- Dívidas, metas, contas e importação.
- Diferenças entre site e aplicativo instalado.
- Perguntas frequentes.
- Política de privacidade.
- Ação principal para entrar com Google.
- Caminho para instalar o programa ou app.

Não invente depoimentos, número de usuários, certificações ou promessas de economia.

### Login

Desenhe:

- Entrar com Google no site.
- Login com senha no acesso remoto ao computador.
- Login indisponível porque o serviço ainda não foi configurado.
- Sessão expirada.
- Retorno do Google em processamento.
- Erro de autenticação com recuperação.

### Primeiro uso

Fluxo curto com:

1. Renda e dia do salário.
2. Divisão do salário.
3. Dívidas existentes.
4. Primeira meta ou reserva de emergência.

Permita entender o progresso, voltar e pular etapas opcionais.

## 6. Painel principal

Organize o Painel para responder, nesta ordem:

**Minha situação → o que exige atenção → o que fazer → minha evolução.**

Inclua:

- Entradas do mês.
- Saídas do mês.
- Saldo do mês.
- Comparação com o mês anterior.
- Percentual da renda que já saiu.
- Taxa de economia.
- Fluxo de caixa anual: entradas, saídas e saldo.
- Divisão do salário: planejado versus realizado.
- Gastos por categoria e participação no total.
- Últimos lançamentos.
- Próximos vencimentos.
- Resumo das dívidas e acesso ao plano.
- Orçamentos próximos do limite ou estourados.
- Metas e progresso.
- Resumo do ano.
- “O que o time viu”: achados dos especialistas com ações.

Diferencie **saldo do mês**, **saldo das contas**, **dinheiro comprometido** e **patrimônio**.
Não trate esses conceitos como equivalentes.

Cada indicador precisa ter período, unidade e significado claros. Evite métricas decorativas
sem decisão associada.

## 7. Lançamentos e recorrências

Tela com:

- Listagem por período.
- Entradas, saídas e saldo.
- Busca por descrição.
- Filtros por categoria, tipo, faixa de valor e mês ou histórico completo.
- Total dos resultados filtrados.
- Criar, editar, apagar e desfazer exclusão.
- Copiar lançamentos do mês anterior.
- Identificação da conta ou cartão.
- Identificação das parcelas.

Formulário:

- Entrada ou saída.
- Valor.
- Descrição.
- Categoria.
- Data.
- Conta ou cartão opcional.
- Número de parcelas, quando aplicável.
- Repetir todo mês.

Recorrências:

- Listar receitas e despesas recorrentes.
- Mostrar valor, frequência, dia e categoria.
- Pausar, retomar e remover.
- Explicar que apagar uma ocorrência não precisa apagar a recorrência inteira.

Desenhe edição no computador e no celular, filtros ativos, lista vazia e busca sem resultados.

## 8. Contas e cartões

Tipos existentes:

- Conta corrente.
- Poupança.
- Carteira.
- Cartão de crédito.

Inclua:

- Cadastro e edição.
- Saldo inicial das contas.
- Saldo atual.
- Cartão com fechamento e vencimento.
- Valor em aberto.
- Fatura atual e próxima.
- Compras e parcelas de cada fatura.
- Patrimônio e evolução dos últimos 12 meses.
- Visualização dos gráficos em tabela.

Não invente transferências bancárias, pagamento real de boletos ou integração automática
com qualquer banco.

## 9. Dívidas e simulações

Esta tela deve ajudar a decidir, não apenas listar dívidas.

Cadastro:

- Nome.
- Credor.
- Saldo devedor.
- Juros mensais.
- Pagamento mínimo.
- Dia do vencimento.

Diagnóstico:

- Dívida mais cara.
- Juros que mais pesam.
- Risco de a dívida continuar crescendo.
- Oportunidades de liberar dinheiro com base no histórico.

Plano:

- Qual pagar primeiro.
- Valor total por mês.
- Valor extra ajustável por campo e controle deslizante.
- Prazo para quitar.
- Mês previsto de quitação.
- Total de juros.
- Economia comparada a pagar só o mínimo.
- Ordem de quitação.
- Gráfico de evolução do saldo.
- Calendário mensal com pagamento por dívida.

Mostre cenários em que o mínimo não cobre os juros e não existe previsão de quitação.

Inclua seleção de cortes de gastos e mostre o efeito de cada corte no prazo e nos juros.
Diferencie claramente **simular** de **registrar um pagamento**.

## 10. Metas e reserva

Inclua:

- Criar, editar e excluir meta.
- Nome, valor desejado, valor já guardado e prazo.
- Categoria vinculada, quando existir.
- Progresso em reais e percentual.
- Valor restante.
- Quanto guardar por mês.
- Situação: em dia, atrasada ou concluída.
- Registrar aporte.
- Simular mudança de prazo ou valor mensal no Consultor.
- Reserva de emergência.
- Resumo das metas no Painel.

Ao registrar aporte, explique quando ele também gera um lançamento.

## 11. Análise financeira

Crie uma tela analítica legível com:

- Quanto ainda pode gastar por dia.
- Gastos variáveis acumulados.
- Projeção de fechamento do mês.
- Contas fixas consideradas.
- Taxa de economia e comparação com a meta.
- Ritmo de gastos por dia.
- Comparação com mês passado ou média de três meses.
- Categorias acima e abaixo da média.
- Orçamentos por categoria.
- Gastos por estabelecimento, frequência e participação.
- Assinaturas e contas fixas com custo mensal e anual.
- Maiores gastos do mês.
- Evolução mensal da taxa de economia.
- Oportunidades de redução de gastos.

Toda projeção deve ser identificada como estimativa. Sem histórico suficiente, explique
a limitação.

Gráficos devem ter legenda, valores acessíveis e alternativa em tabela.

## 12. Categorias, orçamento e divisão do salário

Inclua:

- Grupos Necessidades, Desejos e Investimentos.
- Divisão inicial 50/30/20, editável.
- Validação de que os percentuais somam 100%.
- Categorias de entrada e saída.
- Teto mensal opcional por categoria.
- Valor usado, restante e percentual consumido.
- Alertas ao atingir 80% e ultrapassar o teto.
- Bloqueio explicado ao tentar apagar categoria em uso.

Evite confundir orçamento planejado com gasto já realizado.

## 13. Consultor e time de personagens

Preserve estes personagens:

- **Jade, pantera:** visão geral e orientação.
- **Bento, urso:** dívidas e plano de quitação.
- **Luna, raposa:** vencimentos e orçamentos.
- **Tito, texugo:** metas e reserva.
- **Otávio, coruja:** análise dos gastos.

Não crie cinco aplicativos separados. São especialistas dentro da mesma experiência.

Tela do Consultor:

- Personagem atual e especialidade.
- Troca de personagem.
- Perguntas prontas por assunto.
- Histórico da conversa.
- Caixa de mensagem que cresce com o texto, inclusive no Safari.
- Tabelas e números legíveis.
- Estado da IA e origem do processamento.
- Limpar conversa.
- Salvar resposta como nota.
- Encaminhamento para outro especialista.

Comportamento obrigatório:

1. Ao perguntar, aparece primeiro **“Os números do app”**, com as contas feitas pelo Go.
2. Enquanto a IA trabalha: **“Os números já estão aí. A IA está escrevendo a explicação…”**
3. A explicação chega abaixo; o cartão numérico pode ser recolhido e reaberto.
4. Se a IA falhar, o cartão continua sendo uma resposta útil.
5. Perguntas prontas que dependem apenas de números funcionam sem chamar a IA.
6. Cartões e respostas permanecem ao recarregar; preservar também o estado aberto durante
   atualizações da tela.

A conversa pode propor:

- Criar, alterar ou apagar lançamento.
- Criar meta ou dívida.
- Registrar aporte.
- Registrar pagamento de dívida.

**Nenhuma alteração financeira acontece sem confirmação explícita em um cartão de ação.**
Mostre antes de confirmar o que será alterado e, depois, o resultado aplicado.

Diferencie:

- Resposta calculada pelo aplicativo.
- Explicação escrita pela IA.
- Projeção.
- Proposta ainda não confirmada.
- Alteração já aplicada.

## 14. Avisos e lembretes

Sininho presente nas telas do aplicativo:

- Contador.
- Lista de avisos com personagem, motivo e ação.
- Vencimentos.
- Dívida crescendo.
- Orçamento próximo do limite ou estourado.
- Gasto fora do padrão.
- Meta atrasada ou atingida.
- Estado sem avisos.
- “Limpar tudo”, sincronizado entre aparelhos.
- “Perguntar para…” levando ao especialista com a pergunta pronta.

No app móvel:

- Lembretes de vencimento.
- Resumo semanal do Otávio.
- Balanço das metas pelo Tito.
- Permissão de notificações e configuração dos lembretes.

## 15. Importação

Desenhe o fluxo completo:

**Escolher origem → selecionar conta/cartão → carregar → revisar → confirmar → conferir resultado.**

Origens existentes, conforme a plataforma:

- OFX.
- CSV.
- Fatura em PDF.
- Open Finance via conta da pessoa no Meu Pluggy.
- Notificações bancárias no Android.

Revisão:

- Data, descrição, valor, tipo e categoria.
- Seleção individual e em lote.
- Categoria sugerida pelo histórico ou descrição.
- Sugestão da IA quando disponível.
- Lançamento sem categoria.
- Duplicado já importado.
- Possível duplicado de lançamento manual.
- Pagamento de fatura que deve ficar desmarcado.
- Erros de arquivo ou formato.
- Resumo do que será importado.

Não apresente a conexão bancária como atualização contínua: a busca atual é acionada pela pessoa.

## 16. Notas, exportação e backup

### Notas

- Criar, editar, apagar, buscar e fixar.
- Mostrar data de atualização.
- Salvar resposta do Consultor como nota.
- Exportar nota em PDF ou texto.
- Explicar quando as notas entram no contexto da IA.

### Exportação

- Selecionar mês, ano ou histórico completo.
- Excel com abas.
- Relatório em PDF.
- CSV por conjunto de dados.
- Preparando arquivo, concluído e erro.
- Geração local, inclusive sem internet.

### Backup

- Exportar JSON.
- Restaurar com explicação do efeito sobre os dados existentes.
- Erros de arquivo inválido.
- Backup completo do banco no computador.
- Diferenciar sincronização de backup.

## 17. Ajustes

Organize por assunto:

- Aparência.
- Conta Google e Drive.
- Sincronização, última atualização e pendências.
- Computador pareado.
- Parear por QR code ou código.
- Desconectar ou trocar computador.
- Usar somente neste celular.
- Acesso pelo Wi-Fi e senha no computador.
- Aparelhos conectados e revogação.
- Orientação de acesso fora de casa.
- IA do finan_rich.
- Chave própria de provedor.
- IA local no computador.
- Modelo dentro do aparelho.
- Testar conexão, baixar modelo e acompanhar progresso.
- Modelo indisponível ou incompatível.
- Open Finance.
- Lembretes.
- Bloqueio por senha ou biometria, conforme suporte.
- Localização dos dados, exportação e backup.
- Sessão, sair e fechar o programa, conforme plataforma.

Esconda opções incompatíveis com o ambiente. Segredos e chaves nunca devem aparecer em
histórico, URLs ou mensagens de diagnóstico.

## 18. Métricas e consistência

Use dados de demonstração coerentes entre todas as telas.

Regras importantes:

- Saldo do mês = entradas − saídas.
- A taxa de economia considera saldo mais o valor investido, dividido pela renda.
- Investimentos já registrados como saída não podem ser subtraídos duas vezes.
- Patrimônio é diferente do saldo mensal.
- Faturas, parcelas, dívidas e aportes precisam manter consistência entre telas.
- Sem renda ou histórico, não invente percentuais ou comparações.
- Diferencie zero, dado ausente e cálculo indisponível.
- Não invente score financeiro, rentabilidade, saldo bancário conectado ou promessa de resultado.

Inclua exemplos de mês positivo, mês negativo, dívida cara, orçamento estourado, meta atrasada
e usuário começando do zero.

## 19. Estados e acessibilidade

Para os principais fluxos, desenhe:

- Vazio.
- Carregando.
- Sucesso.
- Erro.
- Sem conexão.
- Alterações aguardando sincronização.
- Sincronizando.
- Falha de sincronização com dados preservados.
- Sessão expirada.
- IA indisponível.
- Limite gratuito atingido.
- Ação aguardando confirmação.
- Ação confirmada.
- Formulário inválido.
- Operação indisponível naquele aparelho.

Acessibilidade:

- Contraste adequado nos dois temas.
- Foco visível e navegação por teclado.
- Rótulos completos nos campos.
- Erros associados aos campos.
- Áreas de toque confortáveis.
- Informação além da cor.
- Gráficos com tabela.
- Respeito a movimento reduzido.
- Modais com foco controlado.
- Confirmação de ações destrutivas.
- Layout utilizável em telas pequenas e com ampliação de texto.

## 20. Entrega esperada

Entregue:

1. Mapa de navegação e inventário de todas as telas.
2. Sistema visual com cores, tipografia, espaçamento e componentes.
3. Todas as telas principais em alta fidelidade.
4. Versões para computador e celular.
5. Temas claro e escuro.
6. Protótipo navegável com os fluxos principais completos.
7. Estados vazios, erros, offline e carregamento.
8. Biblioteca de componentes reutilizáveis.
9. Especificação para implementação: comportamento, responsividade e acessibilidade.
10. Checklist relacionando cada requisito à tela ou fluxo que o atende.

Pode trabalhar por etapas, começando pelo sistema visual e Painel, mas mantenha a entrega
completa como objetivo. Não encerre após a primeira tela.

O produto real usa **Go para regras financeiras, WebAssembly no navegador, JavaScript sem
framework e Capacitor no celular**. O design deve ser viável nessa arquitetura. Se o
protótipo usar outra tecnologia, deixe clara a separação entre demonstração e implementação real.

Melhorias novas podem ser propostas separadamente, identificadas como sugestões. Não trate
recursos futuros como funcionalidades disponíveis.

**Comece propondo a arquitetura de informação e a direção visual; em seguida, desenvolva
o protótipo completo.**
