# Limitação de taxas e controle de abuso

## Escolha para a Rich

Os controles locais estão implementados neste PR e entram em vigor quando a nova
versão for implantada. A parte PostgreSQL continua uma base não ativada.

Usar token bucket para rajadas e manter os limites por hora e dia já existentes.
Não criar fila de pedidos de login ou IA: recusar com `429`, explicar a espera e
devolver `Retry-After` em segundos, arredondado para cima. Respostas de limitação
não podem ficar em cache.

Logout não pode ficar preso pelo limitador: mesmo ao responder `429`, o servidor
expira o cookie do navegador antes de recusar a chamada externa. A resposta informa
que a revogação no Google não foi executada; apagar cookie não equivale a revogar
o acesso de todos os aparelhos ou a permissão no provedor.

Três camadas têm responsabilidades diferentes:

1. Borda Vercel/CDN: recusar tráfego volumétrico antes do processo Go. Precisa de
   configuração no projeto real; um middleware não substitui WAF ou proteção DDoS.
2. Processo Go: token bucket por IP antes de ler o corpo de chat e OAuth, memória
   limitada e quotas existentes. Protege a instância e evita chamadas externas
   quando o pedido já foi recusado. Não é limite global distribuído.
3. Estado compartilhado: PostgreSQL persistente, reserva atômica por escopo e
   sujeito. A base SQL está testada, mas não ligada à Vercel: falta o projeto e
   a integração do BFF. Não alegar proteção global enquanto isso não existir.

Por enquanto PostgreSQL é a escolha para autenticação: já faz parte da arquitetura
aprovada e o volume esperado é pequeno. Não contratar Redis só para esse começo.
Se a medição futura mostrar contenção/latência, migrar o mesmo contrato para um
armazenamento dedicado com operação atômica, sem enfraquecer a política.

## O que já existe

O proxy mantém quotas separadas para IA e OAuth, por aparelho e IP, em janelas
de hora e dia. O computador limita tentativas de senha. Há limite de corpo,
timeout e limite de tentativas de provedores. Esses controles não devem ser
removidos ao acrescentar proteção de rajadas.

O identificador Bearer usado pelo proxy de IA não é prova de usuário ou cliente:
pode ser inventado. Limites de segurança não podem depender só dele. Identidade
de cliente será obtida da sessão do BFF, nunca de `X-User-ID` enviado livremente.

## Política inicial

Valores são um ponto de partida explícito, a ajustar com uso legítimo e métricas,
sem alterar silenciosamente os testes ou desproteger endpoints.

| Camada / escopo | Rajada máxima | Recarga |
|---|---:|---:|
| Go local, OAuth por IP | 10 pedidos | 1 token a cada 6 segundos |
| Go local, chat por IP | 5 pedidos | 1 token por segundo |
| PostgreSQL futuro, login por IP | 10 | 1 a cada 6 segundos |
| PostgreSQL futuro, login por conta | 5 | 1 a cada 12 segundos |
| PostgreSQL futuro, cadastro por IP | 5 | 1 a cada 720 segundos |
| PostgreSQL futuro, recuperação por IP | 5 | 1 a cada 720 segundos |
| PostgreSQL futuro, recuperação por conta | 3 | 1 a cada 1.200 segundos |
| PostgreSQL futuro, IA por usuário | 3 | 1 a cada 240 segundos |
| PostgreSQL futuro, IA por IP | 10 | 1 a cada 120 segundos |
| PostgreSQL futuro, IA global | 20 | 1 por minuto |
| PostgreSQL futuro, sync por usuário | 10 | 1 a cada 2 segundos |

Rajada não é uma janela fixa: um balde de login com cinco tokens permite cinco
tentativas imediatas e recupera uma a cada 12 segundos. Isso não equivale a
prometer no máximo cinco tentativas em toda janela móvel de um minuto.

IPs compartilhados por famílias e empresas exigem tolerância. IPv6 precisa ser
agrupado por prefixo para não permitir um balde novo a cada endereço da mesma rede.
Cabeçalhos de origem só são confiáveis atrás de uma borda conhecida que os
reescreva; publicar o proxy diretamente sem esse contrato permite falsificação.

## Estado compartilhado, ainda não ativado

Migração proposta: `internal/identity/migrations/002_rate_limits.sql`.

- `rich_private.rate_policies` contém políticas; clientes não escolhem capacidade
  ou velocidade nem conseguem editar políticas.
- `rich_private.rate_buckets` contém escopo, HMAC do sujeito, saldo e instante.
  Nunca usar e-mail, token, senha ou IP cru como chave persistida. Hash simples
  de e-mail permite dicionário; usar HMAC com segredo de servidor compartilhado.
  Login usa e-mail normalizado conforme o provedor. O sujeito do orçamento global
  é uma constante do servidor, nunca um identificador escolhido pelo cliente.
- `consume_rate_limit` bloqueia a linha, usa o relógio do banco depois do bloqueio,
  recarrega, limita à capacidade e consome atomicamente. Recuo do relógio não
  devolve tokens nem antecipa a última atualização.
- O papel `rich_ratelimit` só executa a função. Não lê tabelas de buckets,
  políticas, sessões ou dados de clientes. A função não executa SQL dinâmico,
  fixa o `search_path` e não fica executável por `PUBLIC`.
- O contador não será uma tabela disponível na Data API do navegador.

Cada operação do BFF deve reservar IP, conta e orçamento na mesma transação,
sempre em ordem fixa para evitar deadlock. Em caso de negativa, não realizar a
operação externa. Não guardar uma transação aberta durante chamada de e-mail/IA.
Pedidos distribuídos consultam a mesma base; contadores locais são apenas uma
defesa adicional e não substituem essa reserva.

Para barrar criação infinita de chaves, a borda e o limite de IP devem ocorrer
antes de criar uma chave de conta. Antes de ativar, configurar expiração/limpeza
em lotes apenas de buckets cujo tempo ocioso já recompõe toda a capacidade.
Apagar um bucket ainda restrito faria o atacante ganhar uma rajada nova.

## Falhas e custos

- Sem estado compartilhado disponível, login, cadastro, recuperação e operações
  externas de custo devem falhar fechadas com `503`, não virar acesso ilimitado.
  Páginas públicas e dados offline não precisam desse serviço para abrir.
- Limitar chamadas não é um teto financeiro em dinheiro. Antes de habilitar
  provedores pagos, exigir orçamento global persistente, limites do provedor,
  limite de tentativas e concorrência de saída. Token bucket local não garante
  custo total quando existem várias instâncias.
- `429` do provedor deve causar espera/backoff limitado e respeito ao prazo,
  não retry imediato infinito. Nunca enfileirar milhares de pedidos na memória.
- Login precisa de controle por conta e IP, sem bloqueio permanente que permita
  derrubar a conta de outra pessoa. Recuperação mantém resposta neutra para não
  revelar quais e-mails estão cadastrados.
- O Supabase tem endpoints públicos próprios: proteger apenas o BFF não bloqueia
  chamadas diretas com chave pública. Configurar também limites, confirmação,
  CAPTCHA quando necessário e orçamento de e-mails no provedor.
- Métricas agregadas de recusas, espera e falhas são suficientes no começo.
  Não registrar e-mails, senhas ou tokens para depurar limitação.

## Validação e limites

O contrato PostgreSQL foi executado em banco real isolado: capacidade, recarga
fracionada, teto após ociosidade, relógio recuado, permissões e 20 conexões
concorrentes. Com capacidade cinco, exatamente cinco reservas passaram e 15
foram recusadas. O teste está em `scripts/test-identity-db.sh` e roda no CI.

Isso valida a reserva no banco, não uma distribuição já instalada no projeto.
Configurar WAF e a conexão compartilhada ainda é necessário. Não foram feitos
ataques de carga contra produção nem testes com credenciais de clientes.

`go test -race ./internal/proxy` passou, incluindo concorrência, memória cheia,
IPv6, recarga, relógio recuado, espera e negação antes do corpo/upstream. Foram
preservadas as quotas e o teto de tentativas dos provedores. A regressão de logout
limitado falhou antes da correção e passou depois, para rajada e quota.

Referências: [HTTP 429 e Retry-After](https://www.rfc-editor.org/rfc/rfc6585.html#section-4),
[prevenção de brute force](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html#protect-against-automated-attacks).
