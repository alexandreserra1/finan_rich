---
name: rich-engineering
description: Aplicar em qualquer implementação, correção, revisão ou validação do Rich. Conecta contratos OpenSpec, skills existentes, gates, inspeção visual e correção limitada para Claude e Codex.
---

# Engenharia do Rich

1. Leia AGENTS.md e docs/engineering/README.md; preserve o estado de git status. Carregue
   também .agents/skills/to-spec/SKILL.md e .agents/skills/grill-with-docs/SKILL.md no início.
2. Consulte o contrato em openspec/specs e o histórico relevante. Para mudanças relevantes,
   use uma pasta própria em openspec/changes. Erros triviais não exigem proposta extensa.
3. Na definição, aplique grill-with-docs para decisões abertas ou to-spec para síntese de
   conversa decidida, automaticamente conforme o contexto e sem exigir comando manual.
   Consulte docs/agents/issue-tracker.md, triage-labels.md e domain.md. Carregue grilling e
   domain-modeling ao entrevistar. Mantenha OpenSpec como fonte única e confirme decisões,
   sem publicar fora do repositório. Para tarefas pequenas, fluxo curto, sem entrevista extra.
4. Reuse Ponytail, TDD, incremental-implementation, systematic debugging equivalente em
   debugging-and-error-recovery e as demais skills existentes em .claude/skills. Superpowers
   já instalado não deve criar fluxo concorrente. Regras do projeto prevalecem.
5. Delegue arquitetura, frontend, backend, QA, causa-raiz e conhecimento conforme o risco,
   lendo a persona integral. Não delegue o mesmo arquivo a duas pessoas.
6. Para frontend, use rich-playwright ou rich-devtools em perfil isolado. Navegue e interaja
   com as telas reais, examine console e rede; registre quatro larguras e temas relevantes.
   Respeite docs/engineering/design-quality.md. Nunca use dados financeiros reais.
7. Execute node scripts/engineering.mjs check affected antes de concluir. Corrija uma falha
   por vez, com hipótese e evidência; pare ao atingir cinco tentativas, quinze minutos ou
   repetição sem progresso. Consulte docs/engineering/testing-strategy.md.
8. Registre resultados e limites. Não faça push, merge, deploy ou migração destrutiva
   automática. Não declare validação visual, funcional ou integração real sem executá-la.

## Corpus financeiro

Em exemplos financeiros ou preparação LoRA, aplique
`.agents/skills/review-financial-corpus/SKILL.md` antes de propor liberação.
