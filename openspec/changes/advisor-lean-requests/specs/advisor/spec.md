## ADDED Requirements

### Requirement: Ferramentas só quando a pergunta pede
O pedido à IA SHALL levar as ferramentas do personagem só quando a pergunta pede uma ação, e só
as de buscar, alterar e apagar quando a pergunta busca nos lançamentos. Conselho e conversa SHALL
ir sem ferramentas e sem as instruções sobre elas. Se a IA disser que deixou um cartão pronto sem
ter ferramentas, o app SHALL tentar de novo uma vez com as ferramentas do personagem.

#### Scenario: Conselho
- **WHEN** a pessoa pergunta "o que eu faço com meu dinheiro?" e a pergunta vai para a IA
- **THEN** o pedido vai sem ferramentas, numa chamada só

#### Scenario: Ação
- **WHEN** a pessoa pede "lança 30 de cinema"
- **THEN** o pedido leva as ferramentas do personagem e a proposta vira um cartão

#### Scenario: Ação não reconhecida
- **WHEN** o pedido foi sem ferramentas e a IA responde "deixei pronto para você confirmar"
- **THEN** o app pede de novo, com as ferramentas do personagem
