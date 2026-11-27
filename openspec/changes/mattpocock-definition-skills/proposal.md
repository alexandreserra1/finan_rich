## Why

A pessoa pediu to-spec e grill-with-docs no fluxo compartilhado do Claude e do Codex.
A instalação deve incluir dependências, preservar as permissões e evitar outro sistema
de especificação concorrendo com o OpenSpec. A pessoa confirmou depois que as duas
skills devem ficar habilitadas automaticamente desde o início, sem comando manual.

## What Changes

- Instalar as duas skills oficiais e somente suas dependências grilling e domain-modeling.
- Manter fonte única, commit fixado, hashes e licença MIT, com links para Claude.
- Integrar o roteamento, o hook de sessão e a configuração local de domínio e rastreamento.
- Habilitar a invocação automática conforme o contexto, com carga orientada no início.
- Testar descoberta, integridade e rastreabilidade das adaptações, sem publicar issues.

## Capabilities

### Modified Capabilities
- engineering: definição e síntese compartilhadas nos dois clientes.

## Impact

Somente ferramentas e instruções de desenvolvimento. Sem dependências do produto,
alteração de confiança, migrações ou mudanças no aplicativo.
