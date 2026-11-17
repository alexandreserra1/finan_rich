# Problemas conhecidos

Consulte antes de corrigir a mesma área. Atualize o registro existente em vez de duplicá-lo.
Cada incidente deve conter esperado, observado, status, hipótese ou causa confirmada,
arquivos, correção, testes, regressões e prevenção. Nunca inclua credenciais.

## ENG-001 — Credenciais reais e consentimento bancário

- Esperado: importar transações mediante autorização, sem expor credenciais.
- Observado: a integração Pluggy pessoal existe; não há gratuidade comercial multicliente
  confirmada nem provedor substituto contratado.
- Status: pendência externa; não é defeito corrigido.
- Causa: requisitos comerciais e regulatórios não equivalem a uma API pública gratuita.
- Fontes e próximos passos: [OPEN-FINANCE.md](../OPEN-FINANCE.md).
- Teste possível: fixtures e importação OFX/CSV; não provar contas reais sem consentimento.

## ENG-002 — Login/Drive e cadastro sem Google

- Esperado: entrar e sincronizar sem perder acesso nem dados.
- Observado histórico: erro 403 do Drive e configuração inválida de Client Secret.
- Status: há diagnóstico e recuperação no código; causa do 403 pessoal não confirmada.
- Não confundir: corrigir secret informado pela pessoa não comprova escopo, consentimento
  ou API do Drive habilitada. Testes locais usam Google simulado.
- Cadastro Supabase: adaptador e esquema existem, mas projeto, credenciais e SMTP ainda
  precisam ser configurados. Não anunciar cadastro em produção com base no adaptador.
- Próximo passo: obter somente código de erro sanitizado e verificar configuração com a
  pessoa; nunca pedir que envie senha ou secret ao chat.

## ENG-003 — Aprovação desatualizada no novo gate

- Esperado: uma nova falha invalida qualquer aprovação anterior, mesmo sem editar o código.
- Observado na revisão inicial: a evidência era gravada somente no sucesso.
- Causa confirmada: faltava estado de execução e invalidação antes dos testes.
- Correção: `scripts/engineering.mjs` grava running, passed ou failed; o Stop exige passed
  e o hash atual, incluindo o índice e o conteúdo de trabalho.
- Teste: `aprovação real é invalidada por nova falha com o mesmo hash` simula uma dependência
  externa indisponível depois da aprovação, reprova e confirma que Stop pede correção.
- Regressões examinadas: índice parcialmente preparado, remoções e compactação de sessão.
- Status: corrigido e coberto por teste local; CI nova ainda não foi publicada.

## ENG-004 — Risco de processos órfãos ao exceder prazo

- Esperado: uma verificação com limite não deixa seu servidor e subprocessos rodando.
- Causa identificada na revisão: timeout somente no filho direto não cobre seus descendentes.
- Correção: gates longos usam grupo de processos próprio e encerram o grupo ao exceder prazo,
  saída ou receber interrupção. A prova MCP tem encerramento com prazo e escalonamento.
- Teste: filho e neto ignoram SIGTERM em fixture; timeout encerra ambos e confirma o PID do neto.
- Limite: o controle completo de grupos foi validado no macOS e é usado no Linux da CI;
  a mesma garantia para árvores de processos no Windows não foi validada.
- Status: corrigido localmente; não altera nem encerra processos iniciados pela pessoa.

## Modelo para novos incidentes

Use identificador estável, data e status. Separe hipótese de causa confirmada. Informe
comando reproduzível, evidência, correção, testes executados, possíveis regressões e risco
restante. Referencie a mudança OpenSpec e arquivos relevantes, sem copiar a arquitetura.
