## Why

Duas lacunas do Consultor. Primeira: "quanto gastei com mercado?", "...em setembro?" e "...mês
passado?" só tinham conta pronta para lugar e mês atual; sem IA, a resposta caía no resumo do
mês. Segunda: a avaliação com IA real do começo do zero (`TestEvalOnboarding`, nova) mostrou
que o modelo grátis listava as 4 perguntas de uma vez e dizia "conta criada" ou "deixei pronta"
sem chamar a ferramenta (nenhum cartão aparecia). Também perguntava a categoria inventando
nomes, e às vezes o provedor recusava a resposta por JSON quebrado na ferramenta.

## What Changes

- Gasto por categoria (pelo nome ou por uma palavra marcante: "internet" → Contas da casa) e
  por mês ("mês passado", "em setembro", "março de 2026"), pronto para a IA (Otávio e Lia) e
  na resposta sem IA. O lugar mais longo ("mercado extra") vence a categoria.
- Começo do zero mais confiável:
  - com valores na mensagem, o 1º passo obriga a ferramenta (`tool_choice: "required"`);
  - o resultado lembra a IA dos outros itens da mesma mensagem;
  - instruções: uma pergunta por vez, sem perguntar a categoria.
- A categoria que falta ou não existe sai das regras da importação (`importer.SuggestCategory`).
- Afirmação falsa de cartão ("deixei pronto" sem ferramenta): uma nova tentativa com aviso; se
  repetir, a resposta diz que não conseguiu.
- JSON quebrado na ferramenta (erro do provedor no meio da resposta): uma nova tentativa.
- `TestEvalOnboarding`: avaliação de vários turnos com IA real, desligada sem chave (10 de 10).

## Capabilities

### Modified Capabilities
- advisor: gasto por categoria e mês, começo do zero confiável com o modelo grátis.

## Impact

`internal/advisor`, `internal/llm` (`ToolChoice`), `internal/importer` (`SuggestCategory`).
Sem mudança de tela nem de dados. O `tool_choice` passa pelo intermediário sem mudança.
