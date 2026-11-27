## 1. Documentação desta entrega
- [x] 1.1 Inspecionar consultor, conhecimento, anonimização, provedores e avaliações existentes.
- [x] 1.2 Consultar fichas oficiais dos candidatos e referências de LoRA/MLX.
- [x] 1.3 Registrar arquitetura proposta, limites, contratos, riscos e sequência de implementação.
- [x] 1.4 Validar formato OpenSpec e links locais desta mudança: validação estrita aprovada, 15 links locais existentes e git diff --check sem erros.

## 2. Aprovação e laboratório — piloto local autorizado
- [ ] 2.1 Confirmar escopo, laboratório local, uso dos dados, revisão humana e critérios quantitativos.
  - Laboratório local e dados fictícios autorizados. Responsável pela revisão financeira
    e metas de adoção ainda pendentes; o piloto exige todos os critérios existentes, sem flexibilizá-los.
- [x] 2.2 Fixar revisão e licença dos modelos e runtime; conferir disco e compatibilidade antes de baixar pesos.
  - Ollama 0.30.10; manifesto de digests em tools/ai-lab/models.json. Candidato Q4_K_M;
    M1 Pro/16 GiB e 538 GiB livres observados antes do download. Licenças Apache verificadas.
- [x] 2.3 Criar ambiente isolado reproduzível, sem alterar dependências do aplicativo.
  - Python padrão em .artifacts/ai-lab/venv; doze testes locais aprovados, também ligados
    ao gate de engenharia e ao CI existente. Revisão estática
    aprovada após corrigir prazos, limpeza de modelos, integridade e isolamento dos relatórios.
- [ ] 2.4 Executar baseline e candidato com mesmos casos fictícios; registrar memória, latência, ferramentas e falhas.
  - Piloto executado: 30/32 critérios no Qwen2.5-7B e 27/32 no Qwen3.5-4B; sem falhas
    de limpeza na execução final. Evidências e limites em tools/ai-lab/RESULTADOS.md.
    Item permanece aberto para rastrear ferramentas, medir pico de memória e ampliar a
    avaliação. Nenhum candidato aprovado; falso negativo de formato identificado sem alterar a rubrica.

## 3. Corpus e RAG — depende do laboratório
- [ ] 3.1 Selecionar fontes permitidas, registrar manifesto e criar perguntas rotuladas com trechos esperados.
- [ ] 3.2 Implementar ingestão limitada, revisão, deduplicação e índice versionado; testar corrupção e atualização atômica.
- [ ] 3.3 Comparar recuperação lexical/semântica e custo; escolher apenas com resultados reproduzíveis.
- [ ] 3.4 Testar ausência de fonte, documento vencido, prompt injection e referências inexistentes.

## 4. Integração — depende de aprovação do RAG
- [ ] 4.1 Integrar primeiro no computador, preservando Provider.Chat, streaming, ferramentas e modo determinístico.
- [ ] 4.2 Definir topologia web/mobile e armazenamento antes de implementar ou distribuir o índice.
- [ ] 4.3 Testar políticas local/nuvem, contextos e logs sem vazamentos e falhas sem rede.
- [ ] 4.4 Executar make test e make e2e; validar interface nos navegadores e larguras definidos no desenho.
- [ ] 4.5 Testar reversão de modelo/prompt/índice sem perda de dados financeiros.

## 5. Treinamento opcional — só com lacuna medida
LoRA pausado por decisão explícita de 26/11/2026. Nada de baixar pesos, treinar,
publicar modelo ou subir dados. Terceira rodada independente: blocked, parallel_agents,
11 bloqueantes e 40 avisos no mesmo contexto d9115ca5; substitui a conclusão sequencial.

- [ ] 5.1 Demonstrar erros persistentes que justifiquem LoRA e aprovar o experimento.
- [ ] 5.2 Revisar dataset permitido; separar treino, validação e teste sem contaminação.
  - [x] 5.2.1 Implementar orquestrador OpenSpec compartilhado entre os clientes, executar
    checks e consolidar três eixos de revisão, registrando execução mista e limites por cota.
    Resultado real blocked: pagamento sugerido acima da quitação e premissas incompatíveis
    no contexto. Treinamento continua desabilitado; veja review-findings.md.
  - [x] 5.2.2 Corrigir as causas financeiras com expectativas independentes, regenerar
    o contexto e revisar antes de propor liberação; não ajustar apenas os alvos para passar.
    Regressões Go e matriz de 48 combinações aprovadas; corpus regenerado e pareceres
    antigos rejeitados. Correções do produto separadas no PR #52, capability advisor.
    A segunda revisão sequencial foi substituída pela terceira independente, blocked;
    essa caixa histórica não representa aprovação do corpus. Veja review-findings.md.
  - [x] 5.2.3 Corrigir controlador, qualidade, triagem, encadeamento e vínculo do pacote LoRA,
    com testes que reproduzam os achados; não rebaixar bloqueantes.
    Testes de regressão e integração Go/Python aprovados; isso não fecha os achados
    semânticos da terceira rodada nem libera o corpus. Contexto inclui pacote quando fornecido.
  - [ ] 5.2.4 Refazer gerador com pedidos reais, livros coerentes, injeção indireta,
    contrapontos às recusas e cobertura por split; aguardar correções do app na main.
  - [ ] 5.2.5 Após aviso da pessoa, regenerar e produzir pacote; parar para quarta revisão
    independente do Claude. Formulário novo só sem bloqueantes, com nome e hash novos.
  - [ ] 5.2.6 Comparar respostas do app, Qwen2.5-7B e prompts locais em avaliação reservada;
    relatar acertos por critério e falhas por categoria sem treino.
    Comparador app real/replay e esquema estrito implementados em reservedeval; nenhuma
    inferência executada. Faltam conversor, coleta Ollama, casos/rubrica finais e revisão.
  - Preparação inicial autorizada e executada: 24 exemplos fictícios em 12 famílias,
    com 16/4/4 exemplos por conjunto; ferramentas e contexto gerados pelo Go existente.
    Fonte em tools/ai-lab/corpusgen, exportador com validações e manifesto de hashes.
    Revisão humana/semântica e ampliação de cobertura pendentes; training_allowed permanece falso.
    Consulte tools/ai-lab/DATASET.md. Isso não conclui a revisão nem inicia treinamento.
- [ ] 5.3 Confirmar suporte do runtime ao modelo num piloto limitado por tempo e memória.
  - [x] 5.3.1 Preparar 40 alvos derivados dos 24 exemplos e conferir máscaras/prefixos
    com o tokenizer oficial fixado, sem mudar famílias entre conjuntos.
  - [x] 5.3.2 Instalar ambiente experimental MLX isolado e executar smoke numérico
    de três passos numa camada aleatória 8x8, sem pesos Qwen ou adaptadores exportados.
  - [ ] 5.3.3 Concluir revisão humana/licenças e validar memória/tempo do modelo completo.
    Os exemplos corrigidos chegam a 4.403 tokens; não truncar para forçar a execução. Consulte tools/ai-lab/LORA.md.
- [ ] 5.4 Comparar base, base+RAG e adaptador+RAG; revisar segurança e utilidade com pessoa responsável.
- [ ] 5.5 Verificar exportação/quantização no runtime do app e repetir avaliações após conversão.

## 6. Operação multiusuário — decisão externa pendente
- [ ] 6.1 Aprovar hospedagem, capacidade, custo e limites; não presumir serviço gratuito ilimitado.
- [ ] 6.2 Validar filas/concorrência, cancelamento, quotas e observabilidade sem conteúdo privado.
- [ ] 6.3 Obter autorização específica antes de publicar modelo, fazer deploy ou mudar provedor padrão.

Uma caixa marcada comprova somente a tarefa descrita. Documentação validada não significa
que RAG, treinamento ou infraestrutura estejam implementados.

## 7. Correções da quarta revisão independente
- [x] 7.1 Conferir achados no código e preservar o resultado blocked da quarta rodada.
- [x] 7.2 Usar ID da compra consultada e mostrar data/valores de edição/exclusão.
- [x] 7.3 Separar atraso de vencimentos futuros e capturar os cartões reais do executor.
- [x] 7.4 Cobrir comportamentos nos três conjuntos, renda no onboarding e injeções variadas.
- [x] 7.5 Bloquear duplicatas exatas/mascaradas e não contar histórico como contraponto.
- [x] 7.6 Conferir chamadas, argumentos, erros e propostas na avaliação reservada e no hash.
- [x] 7.7 Concluir validação integrada e gerar pacote para a quinta revisão sem modelos.
- [ ] 7.8 Receber os três pareceres independentes do Claude; reavaliar desenho se reprovado.

LoRA continua pausado; quinta revisão não equivale a aprovação humana nem autorização de treino.
