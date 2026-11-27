# Desenho

`PlanMonth` recebe o total de fixos pendentes calculado no domínio. Recorrências pausadas,
fora da vigência ou cujo lançamento gerado já existe, inclusive excluído, não entram.
Insights reutiliza o critério, mantendo sua separação de investimentos.

Mínimos pendentes são limitados ao saldo mais juros e descontam pagamentos lançados.
O orçamento extra não atribui pagamentos agregados a uma dívida sem vínculo: preserva
a reserva restante. O pagamento simulado nunca excede o orçamento nem o saldo com juros
do primeiro período; esse teto não é uma cotação do credor.

Simulações declaram taxa constante e primeiro pagamento no próximo mês. Metas contam
o aporte deste mês e mostram mês de chegada e quantidade de aportes na mesma frase.
O conhecimento distingue limites específicos de cartão de empréstimos genéricos.

Testes de domínio, retrato, ferramenta e resposta determinística cobrem as regras. A matriz
de orçamento verifica 48 combinações. Esse recorte não resolve reserva dos gastos futuros
do mês nem os demais bloqueantes do corpus; essas correções são de responsabilidade do Claude.
