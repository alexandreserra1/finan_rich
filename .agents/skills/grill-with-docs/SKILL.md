---
name: grill-with-docs
description: Esclarecer automaticamente requisitos, decisões ou desenho ainda abertos no Rich, com perguntas fundamentadas no código e registro de termos e ADRs confirmados. Usar quando houver ambiguidade relevante, sem exigir comando manual.
disable-model-invocation: false
---

Carregue estas instruções desde o início da sessão. Aplique a entrevista automaticamente
quando houver decisões relevantes em aberto, não em toda mensagem ou correção trivial.
Consulte os fatos no código antes de perguntar. Siga docs/agents/domain.md e preserve
o OpenSpec como fonte única. No Codex sem Skill tool, leia as duas dependências completas.
Confirme decisões com a pessoa, sem repetir o que já foi decidido nem presumir respostas.
Não publique, faça commit, push, merge ou deploy sem autorização específica.

Call the Skill tool twice, for "grilling" and "domain-modeling".
