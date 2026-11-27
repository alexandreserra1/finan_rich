## Context

O Rich já usa OpenSpec, skills de entrevista e ADRs, AGENTS.md compartilhado e hooks.
Os originais são de invocação explícita; a pessoa autorizou adaptar esse comportamento.
Grill-with-docs delega para
grilling e domain-modeling; instalar só o arquivo principal deixaria o fluxo incompleto.

## Decisions

Manter a base oficial em .agents/skills e os links relativos do Claude. As duas skills
principais recebem metadados de invocação automática, descrições locais e guardas de
publicação; helpers e formatos não mudam. Preservar hashes originais e locais separados,
com registro explícito de cada adaptação, sem baixar atualizações em toda sessão.

O hook manda carregar as duas skills desde o início e aplicá-las conforme a tarefa,
sem aguardar um comando manual. Decisões abertas usam grill-with-docs; conversa decidida
que precisa de contrato usa to-spec. Correções triviais não ganham entrevistas extras.
Ativação automática não autoriza publicar ou presumir respostas da pessoa. No Codex
sem Skill tool, as dependências são lidas diretamente.
OpenSpec permanece o rastreador local; os exemplos de publicação e docs/adr do original
não substituem as permissões do projeto nem os ADRs existentes em docs/decisions.

## Validation

Testes falham sem o manifesto e o roteamento no hook. Depois, verificam os quatro nomes,
os links compartilhados, os arquivos auxiliares, os hashes, o registro de adaptações e
os metadados de ativação automática nos dois clientes.
A fixture de sessão usa o mesmo script dos hooks Claude e Codex. A validação estática
não prova obediência futura do modelo nem dispensa recarregar a descoberta do cliente.
