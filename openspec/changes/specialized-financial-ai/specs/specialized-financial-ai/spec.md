## ADDED Requirements

### Requirement: Experimentos rastreáveis antes da adoção
O processo MUST registrar revisões de modelo, dados, contexto e avaliação antes de recomendar uma troca em produção.

#### Scenario: Candidato sem avaliação executada
- **WHEN** um modelo consta apenas como candidato
- **THEN** o relatório identifica a pendência e não o apresenta como aprovado ou treinado pela Rich

#### Scenario: Teste Go retorna zero com critérios incompletos
- **WHEN** o avaliador imprime uma nota parcial apesar de o processo Go terminar sem erro
- **THEN** o laboratório reprova a execução, preserva a evidência e não reduz os critérios

#### Scenario: Experimento interrompido ou concorrente
- **WHEN** uma execução excede os limites ou outro laboratório tenta usar os mesmos modelos
- **THEN** o laboratório interrompe ou recusa a execução, preserva diretórios separados e tenta descarregar somente seus modelos

### Requirement: Conhecimento fundamentado e atualizável
A recuperação MUST preservar origem, licença, versão e localização dos trechos e rejeitar documentos sem revisão exigida.

#### Scenario: Não há fonte suficiente
- **WHEN** os trechos recuperados não sustentam uma afirmação
- **THEN** a resposta informa a limitação e não inventa referência ou taxa atual

### Requirement: Fronteira financeira preservada
A integração MUST manter regras e validações financeiras em Go/Wasm e impedir execução não autorizada solicitada pelo modelo ou documento.

#### Scenario: Regra financeira defeituosa encontrada no corpus
- **WHEN** um exemplo reproduz um defeito nas regras ou textos do aplicativo
- **THEN** o laboratório registra a evidência e bloqueia o corpus sem ajustar apenas o alvo
- **AND** a correção do produto é especificada na capacidade advisor e revisada separadamente

#### Scenario: Documento tenta comandar uma ferramenta
- **WHEN** um trecho recuperado contém instruções para alterar dados ou revelar informações
- **THEN** ele é tratado como conteúdo não confiável e não concede permissão para executar ações

### Requirement: Revisão independente prevalece sobre triagem e autoavaliação
O laboratório MUST preservar os bloqueantes encontrados e MUST NOT liberar treinamento
por checks determinísticos ou por uma revisão sequencial que uma revisão posterior reprovou.

#### Scenario: Terceira rodada no mesmo contexto
- **WHEN** a revisão independente parallel_agents do contexto d9115ca5 reprova o corpus
- **THEN** o resultado vigente é blocked, preservando os 11 bloqueantes e 40 avisos
- **AND** o formulário humano antigo não é reaproveitado como aprovação de outro corpus

#### Scenario: Pausa de LoRA e divisão de responsabilidade
- **WHEN** a pessoa pausa o treino e atribui laboratório ao Codex e regras do app ao Claude
- **THEN** o laboratório mantém training_allowed falso e não baixa pesos, treina, publica modelo ou sobe dados
- **AND** a regeneração final espera o aviso de que as correções estão na main

### Requirement: Validadores verificam alvos e conversas reais
O controlador MUST verificar qualidade observável dos alvos, oferecer diagnóstico por
critério e vincular parecer ao código de preparação e ao pacote LoRA quando fornecido.

#### Scenario: Conversa sem chamada ou com vários turnos
- **WHEN** o personagem recebe ferramentas mas responde com dados já disponíveis, ou faz chamadas em turnos sucessivos
- **THEN** o validador aceita a conversa completa com resultados associados e sem chamadas pendentes

#### Scenario: Erro do executor
- **WHEN** a ferramenta devolve erro de argumentos, autorização ou execução
- **THEN** o alvo precisa reconhecer a falha, sem apresentá-la como resultado financeiro bem-sucedido

#### Scenario: Alvo com formato interno ou promessa indevida
- **WHEN** a resposta final expõe identificador interno, data ISO, molde Hoje no app ou falsa ação concluída
- **THEN** a triagem registra o problema sem tratar negação ou citação de risco como promessa do assistente

### Requirement: Medir antes de ajustar pesos
O laboratório MUST reservar casos com perfis e frases novos para comparar respostas
determinísticas, Qwen2.5-7B com prompt atual e variações locais de prompt sem treinar.

#### Scenario: Escolha de prompt sem contaminar teste
- **WHEN** variações são escolhidas com o conjunto de desenvolvimento
- **THEN** o teste reservado é usado somente na comparação final com acertos por critério e falhas por categoria
- **AND** LoRA só volta com erro residual demonstrado e nova aprovação explícita da pessoa

### Requirement: Dados privados não viram corpus compartilhado
O laboratório e a recuperação pública MUST usar somente materiais permitidos, sem extratos, perguntas ou conversas reais coletados automaticamente.

#### Scenario: Treinamento supervisionado
- **WHEN** exemplos são preparados para LoRA
- **THEN** têm origem permitida, revisão e separação de treino e teste, sem inclusão automática de dados dos usuários

#### Scenario: Preparação do corpus fictício inicial
- **WHEN** o gerador prepara os exemplos supervisionados
- **THEN** resultados financeiros vêm das ferramentas Go existentes, conversas preservam o vínculo chamada/resultado e exemplos mantêm origem sintética e revisão pendente

#### Scenario: Variações do mesmo cenário
- **WHEN** duas perguntas pertencem à mesma família de cenário
- **THEN** ficam no mesmo conjunto e o exportador rejeita famílias cruzadas ou perguntas normalizadas duplicadas

#### Scenario: Exportação anterior à revisão humana
- **WHEN** o corpus inicial é exportado sem revisão humana concluída
- **THEN** o manifesto informa training_allowed falso, registra hashes e contagens e não declara o dataset aprovado para treinamento

#### Scenario: Supervisão de chamada e resposta com máscara de prefixo
- **WHEN** o runtime usa somente a última mensagem do assistente como alvo
- **THEN** chamada de ferramenta e resposta final viram alvos separados no mesmo conjunto, sem aplicar perda às saídas da ferramenta ou às mensagens do usuário

#### Scenario: Template descoberto fora do manifesto
- **WHEN** a pasta de cache contém um template adicional não verificado
- **THEN** o preflight usa uma cópia exclusiva somente dos arquivos cobertos pelos hashes e não renderiza o template extra

#### Scenario: Máscara ou prazo incompatível
- **WHEN** o prefixo não corresponde exatamente aos tokens da conversa ou a verificação excede seu deadline
- **THEN** o preflight reprova a verificação sem liberar treino nem truncar silenciosamente o alvo

### Requirement: Preservação do funcionamento sem modelo
A integração MUST preservar respostas determinísticas, escolhas de privacidade e funcionamento financeiro offline existentes.

#### Scenario: Modelo ou índice indisponível
- **WHEN** uma dependência de IA falha ou excede o prazo
- **THEN** o aplicativo informa a limitação sem bloquear os registros financeiros nem fingir uma resposta fundamentada

### Requirement: Adoção e publicação condicionadas
A entrega MUST exigir avaliações e autorização específica antes de mudar produção, publicar pesos ou contratar infraestrutura.

#### Scenario: Revisão orquestrada do corpus
- **WHEN** o corpus é proposto para liberação
- **THEN** um orquestrador verifica integridade e regras, reúne pareceres financeiro, segurança e qualidade vinculados ao mesmo corpus/contrato/código e bloqueia inconsistências

#### Scenario: Revisão assistida por IA concluída
- **WHEN** checks e pareceres técnicos estão aprovados
- **THEN** o relatório distingue revisão por IA de revisão humana e mantém a liberação do treinamento como decisão separada, sem ativar treino ou produção

#### Scenario: Resultado local satisfatório
- **WHEN** o laboratório passa nos critérios locais
- **THEN** isso não autoriza deploy nem comprova capacidade para vários usuários no site e celular
