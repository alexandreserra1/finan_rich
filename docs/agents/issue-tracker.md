# Rastreamento das especificações

O rastreador destas skills é local: `openspec/changes/<nome>/`. Não é necessário instalar
outro gerenciador, executar setup global nem criar issues no GitHub.

Antes de escrever, procure uma mudança relevante já aberta. Use a CLI local
`tools/engineering/node_modules/.bin/openspec` e os modelos do projeto. Nunca sobrescreva
uma proposta de outro trabalho nem crie um segundo SPEC.md ou uma cópia em `.scratch/`.

Ao usar `/to-spec`, registre apenas decisões já tomadas. Confirme com a pessoa os pontos
de teste antes de fechar a especificação. Preserve o conteúdo do modelo da skill dentro
dos artefatos existentes:

- Problema, solução e limites: `proposal.md`.
- Histórias, decisões, justificativas e estratégia de testes: `design.md`.
- Comportamentos verificáveis: `specs/<capacidade>/spec.md`.
- Etapas e verificações reproduzíveis: `tasks.md`.

A síntese não autoriza implementação, commit, push ou publicação. Se a pessoa pedir uma
issue no GitHub, confirme a autorização e o conteúdo antes de criar ou alterar algo
externo; referencie os artefatos locais, sem manter duas especificações concorrentes.

`ready-for-agent` é um estado local definido em [triage-labels.md](triage-labels.md).
As instruções e permissões de AGENTS.md prevalecem sobre a publicação sugerida no original.
