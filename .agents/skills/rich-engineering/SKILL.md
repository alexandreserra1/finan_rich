---
name: rich-engineering
description: Aplicar em qualquer implementação, correção, revisão ou validação do Rich. Conecta contratos OpenSpec, skills existentes, gates, inspeção visual e correção limitada para Claude e Codex.
---

# Engenharia do Rich

1. Leia AGENTS.md e docs/engineering/README.md; preserve o estado de git status.
2. Consulte o contrato em openspec/specs e o histórico relevante. Para mudanças relevantes,
   use uma pasta própria em openspec/changes. Erros triviais não exigem proposta extensa.
3. Reuse Ponytail, TDD, incremental-implementation, systematic debugging equivalente em
   debugging-and-error-recovery e as demais skills existentes em .claude/skills. Superpowers
   já instalado não deve criar fluxo concorrente. Regras do projeto prevalecem.
4. Delegue arquitetura, frontend, backend, QA, causa-raiz e conhecimento conforme o risco,
   lendo a persona integral. Não delegue o mesmo arquivo a duas pessoas.
5. Para frontend, use rich-playwright ou rich-devtools em perfil isolado. Navegue e interaja
   com as telas reais, examine console e rede; registre quatro larguras e temas relevantes.
   Respeite docs/engineering/design-quality.md. Nunca use dados financeiros reais.
6. Execute node scripts/engineering.mjs check affected antes de concluir. Corrija uma falha
   por vez, com hipótese e evidência; pare ao atingir cinco tentativas, quinze minutos ou
   repetição sem progresso. Consulte docs/engineering/testing-strategy.md.
7. Registre resultados e limites. Não faça push, merge, deploy ou migração destrutiva
   automática. Não declare validação visual, funcional ou integração real sem executá-la.
