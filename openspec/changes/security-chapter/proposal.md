## Why

O capítulo de segurança do livro de desenho de sistemas pede três coisas que a Rich ainda não
fazia: backup sem segredos (o `backup.db` levava a chave da IA e o segredo do Pluggy em texto
puro), registro de auditoria dos acessos (quem entrou pela rede, senhas erradas, pareamentos) e
análise estática de segurança no CI. O resto do capítulo já estava coberto ou não se aplica, e
isso não estava escrito em lugar nenhum.

## What Changes

- `store.BackupTo` tira do backup a chave da IA, o segredo do Pluggy, as sessões e os
  aparelhos pareados, sem deixar o texto apagado no arquivo.
- Registro de acesso no computador (`access_log`, migração 010), mostrado nos Ajustes, com os
  últimos 200 eventos de segurança e o endereço de quem fez.
- Workflow do CodeQL (Go, JavaScript e workflows) a cada PR, na main e toda segunda.
- `docs/SEGURANCA.md`: o capítulo mapeado para a Rich, com a classificação dos dados, a LGPD,
  o que não se aplica e as decisões pendentes.
- A política de privacidade diz que a pergunta à IA grátis é processada nos Estados Unidos.

## Capabilities

### Modified Capabilities
- security-performance: backup sem segredos, registro de acesso e análise estática.

## Impact

`internal/store` (migração nova, só cria tabela), `internal/web` (registro dos eventos, Ajustes),
`.github/workflows/codeql.yml`, docs e `site/privacidade.html`. Sem dependência nova no app.
Quem restaurar um backup novo digita de novo a chave da IA e o segredo do Open Finance e pareia
o celular de novo.
