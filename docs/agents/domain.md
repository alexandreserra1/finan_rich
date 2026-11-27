# Vocabulário e decisões do Rich

O projeto usa um glossário de domínio único, `GLOSSARY.md`. Crie-o somente quando houver
o primeiro termo confirmado, com definições curtas em português. Não invente termos nem
use o glossário para guardar especificações, detalhes de implementação ou transcrições.

Os ADRs existentes ficam em `docs/decisions/`, com nomes `ADR-NNN-titulo.md`. Preserve
essa pasta, a numeração e o formato existentes. Não crie uma pasta concorrente `docs/adr/`
por causa do exemplo de uma skill. Reuse `documentation-and-adrs` e os modelos existentes.
Só proponha ADR para decisão difícil de reverter, surpreendente sem contexto e com uma
troca real entre alternativas. A instalação das skills não cria decisões ou glossários.

`/grill-with-docs` carrega as duas dependências locais: `grilling` conduz a entrevista e
`domain-modeling` registra termos e decisões conforme forem confirmados. No Codex sem
Skill tool, leia os três SKILL.md completos. Não substitua essa carga por uma entrevista
genérica: confira os arquivos realmente produzidos quando houver termos ou ADRs aprovados.

Consulte primeiro o código e a documentação. Pergunte à pessoa apenas decisões que
dependem dela, com recomendações claras e perguntas em português, sem emojis. Registre
termos confirmados durante a conversa; só implemente depois do entendimento confirmado.
Para fatos duráveis já existentes, consulte ai-memory sem duplicar suas páginas.
