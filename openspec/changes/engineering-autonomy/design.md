## Context

O servidor e os testes já existem. Ferramentas de engenharia ficam isoladas em
tools/engineering, com lockfile. A configuração global dos agentes permanece intacta.

## Goals / Non-Goals

Conectar o que existe e provar o funcionamento. Não criar framework de aplicação,
serviço de agentes permanente, integração bancária fictícia ou novo design system.

## Decisions

OpenSpec atende ao brownfield e gera roteamento para Claude e Codex. Spec-kit não será
instalado em paralelo. Superpowers já está habilitado no Claude; o Codex reusa as skills
equivalentes presentes. O Stop hook pede correção ao agente atual; não inicia outro LLM.
O pre-commit verifica blobs do índice, não substitui testes funcionais. CI executa a suíte.
MCPs usam Chrome isolado, sem perfil pessoal, telemetria ou consulta de URLs ao CrUX.

## Risks / Trade-offs

Hooks dependem da confiança nativa e de uma nova sessão para carregar. Git hooks são
locais e podem ser ignorados com --no-verify; a CI não depende deles. Screenshots novos
são evidências, não baselines aprovados. Não há processos autônomos após fechar a sessão.
