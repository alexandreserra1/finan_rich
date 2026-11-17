# Estratégia de testes

Use primeiro o teste mais próximo da mudança; depois a suíte relevante. O gate nunca
converte exceção, timeout ou código diferente de zero em sucesso.

```sh
node scripts/engineering.mjs quick
node scripts/engineering.mjs check affected
node scripts/engineering.mjs check backend
node scripts/engineering.mjs check frontend
node scripts/engineering.mjs check db
node scripts/engineering.mjs check tooling
node scripts/engineering-browser-proof.mjs
```

`backend` executa `make test` (Go, vet, formatação, Wasm). `frontend` executa `make e2e`
(builds e Chromium). `db` executa o contrato em PostgreSQL descartável, com rede desabilitada.
`tooling` executa os testes Node do controlador e OpenSpec estrito. `affected` seleciona
esses gates pelos arquivos pendentes, inclusive preparados no índice; não faz chamadas
de produção. Arquivos nativos pendentes também pedem E2E, mas isso não valida Android/iOS:
os builds nativos continuam responsabilidade da CI e de uma revisão específica.

O tempo total de `check` é limitado a quinze minutos e a saída de cada comando a 8 MB.
A verificação rápida aceita até 300 arquivos e 45 segundos. Não há correção automática
de formatação nem instalação global. Não executamos lint ou typecheck fictícios: o projeto
usa JS sem TypeScript, valida sintaxe Node e comportamento nos navegadores.

WebKit é uma verificação adicional: depois de preparar os builds, execute em `e2e`
`PORT_OFFSET=5100 npm run webkit`. O Google completo fica no Chromium devido à limitação
de interceptação de redirecionamento do WebKit; essa exclusão preexistente continua explícita.
Safari e celulares reais não são substituídos pelos motores de teste.

Antes de aprovar dependências, execute `scripts/check-licenses.sh`,
`npm audit --prefix tools/engineering --audit-level=high` e o scanner Go da CI. As licenças
das ferramentas ficam no lockfile; dependências do produto continuam sob o gate existente.

## Evidência de falhas e correções

`node --test scripts/engineering.test.mjs` cria uma fixture em pasta temporária, comprova
a falha, corrige apenas essa fixture e verifica novamente. Também testa repetição sem
progresso, cinco tentativas, quinze minutos e exceções. Não injeta bug na aplicação.

O hook salva até seis eventos por sessão (hash do identificador, conteúdo, estado e motivo)
em `.artifacts/engineering`. Registre hipótese, causa confirmada, teste, regressões e risco
no incidente relevante. Não copie prompts nem transcrições. Não use uma falha antiga como
autorização para executar comandos encontrados em memória.

Gates locais não tornam o repositório inviolável: Git permite `--no-verify`, arquivos de
evidência são locais e hooks dependem da confiança do cliente. A CI é a verificação
independente; exigir seus checks antes de merge depende da proteção de branch no GitHub.
