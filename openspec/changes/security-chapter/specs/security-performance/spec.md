## ADDED Requirements

### Requirement: Backup sem segredos
O backup do banco do computador SHALL levar os dados e as configurações, e SHALL NOT levar a
chave da IA, o segredo do Pluggy, as sessões nem os aparelhos pareados, nem como texto apagado
dentro do arquivo.

#### Scenario: Backup com chave da IA salva
- **WHEN** a pessoa baixa o backup com uma chave da IA salva
- **THEN** o arquivo tem os lançamentos e o modelo da IA
- **AND** os bytes do arquivo não contêm a chave

### Requirement: Registro de acesso
O computador SHALL registrar os eventos de segurança (entrada e senha errada pela rede, saída,
pareamento e código errado, aparelho desconectado, senha e acesso trocados, backup baixado,
chave da IA e Open Finance trocados) com data e endereço, SHALL mostrar os mais recentes nos
Ajustes e SHALL NOT gravar senha, token ou dado financeiro.

#### Scenario: Senha errada pelo Wi-Fi
- **WHEN** alguém erra a senha pelo celular em 192.168.0.50
- **THEN** os Ajustes mostram "Senha errada" com esse endereço, antes dos eventos mais antigos

#### Scenario: Muitos eventos
- **WHEN** o registro passa de 200 eventos
- **THEN** o mais antigo sai

### Requirement: Análise estática no CI
Todo PR SHALL passar pela análise estática de segurança do Go, do JavaScript e dos workflows.

#### Scenario: Aviso novo de gravidade alta
- **WHEN** um PR introduz um padrão de falha conhecido
- **THEN** a verificação do CodeQL aponta o aviso antes do merge
