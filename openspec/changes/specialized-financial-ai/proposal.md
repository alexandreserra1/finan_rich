## Why

A pessoa quer uma IA própria da Rich, especializada em ajudar com gastos e finanças,
aproveitando modelos do Hugging Face, RAG e possível treinamento posterior. Pediu uma
documentação de arquitetura e execução antes de implementar.

Hoje existem conhecimento por assunto, ferramentas financeiras em Go, provedores locais
e de nuvem, anonimização e avaliações. Falta um experimento reproduzível que compare
essa base com recuperação documental e ajuste de modelo.

## What Changes

- Documentar o estado atual, os limites de confiança e uma arquitetura incremental.
- Propor modelos candidatos, preparação dos dados, avaliações, RAG e possível LoRA.
- Definir critérios de aprovação, tarefas e decisões ainda abertas.
- A etapa documental não instalou bibliotecas nem alterou o produto. O laboratório
  posterior avaliou o candidato e expôs defeitos no consultor. A implementação também
  alterou regras e textos do produto (plano mensal, recorrências, contexto e simulações).
  Essas correções foram separadas na mudança `advisor-month-plan-fixes`, capability
  `advisor`, PR #52. Não são requisitos da capacidade de laboratório.

Estado: **laboratório local autorizado**, demais etapas em proposta. Depois da entrega
documental, a pessoa autorizou preparar o laboratório e avaliar o Qwen3.5-4B. Isso não
autoriza a adoção definitiva do modelo, publicação, RAG em produção ou infraestrutura.

Depois do piloto, a pessoa autorizou preparar os exemplos supervisionados fictícios e
os conjuntos separados para um futuro ajuste LoRA. Essa etapa gera um corpus inicial
auditável, ainda sujeito a revisão humana e validação do runtime antes de treinar pesos.

Em 26/11/2026, a terceira rodada independente no mesmo contexto da segunda terminou
`blocked`: 11 bloqueantes e 40 avisos, em `parallel_agents`. Ela substitui o parecer
sequencial anterior como resultado vigente. O treino LoRA está pausado por decisão da
pessoa: sem baixar pesos, treinar, publicar modelos ou subir dados. `training_allowed`
continua falso. A regeneração final depende das correções do Claude na main e do aviso
da pessoa; depois o Claude fará a quarta revisão independente.

## Capabilities

### New Capabilities
- specialized-financial-ai: experimentação e validação da IA financeira, com isolamento
  de dados, rastreabilidade e integração às fronteiras existentes.

## Impact

Além da documentação, o laboratório isolado fica em `tools/ai-lab`, reutilizando os
testes do consultor sem trocar o provedor padrão. O histórico inclui correções no produto,
agora separadas na capacidade `advisor`; os demais defeitos do app cabem ao Claude.
Este recorte mantém ferramentas locais e contratos de dados/avaliação. Não pressupõe
migração de banco, serviço Python obrigatório nem adoção do candidato.

## Fora do escopo proposto

Treinar do zero; aprender automaticamente com conversas; compartilhar dados privados;
movimentar dinheiro; executar operações bancárias; recomendar compra e venda de ativos;
substituir as regras Go por respostas probabilísticas; prometer inferência gratuita ilimitada.
