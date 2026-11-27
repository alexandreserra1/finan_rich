package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alexandreserra1/finan_rich/internal/advisor"
	"github.com/alexandreserra1/finan_rich/internal/domain"
	"github.com/alexandreserra1/finan_rich/internal/money"
)

func proposalAnswer(sample scenario, args map[string]any, proposals []advisor.Proposal) string {
	book, today := scenarioLedger(sample), profileToday(sample.profile)
	index := sample.profile / 2
	amount := func(key string) string {
		value, err := money.Parse(args[key].(string))
		if err != nil {
			panic(err)
		}
		return money.Format(value)
	}
	operation := sample.rounds[len(sample.rounds)-1][0].Name
	var answer string
	switch operation {
	case "propor_lancamento":
		if len(proposals) != 1 {
			panic("lançamento sem cartão único")
		}
		var transaction domain.Transaction
		if err := json.Unmarshal(proposals[0].Record, &transaction); err != nil {
			panic(err)
		}
		for _, category := range book.Categories {
			if category.ID == transaction.CategoryID {
				args["categoria"] = category.Name
			}
		}
		answer = fmt.Sprintf([]string{"Preparei **%s de gasto em %s**, na categoria %s, para %s.", "Ficou para revisar a compra em %s: **%s**, em %s, com categoria %s.", "O gasto de **%s** será atribuído a %s (%s), na data %s."}[index], amount("valor"), args["descricao"], args["categoria"], today.Format("02/01/2006"))
		if index == 1 {
			answer = fmt.Sprintf("Ficou para revisar a compra em %s: **%s**, em %s, com categoria %s.", args["descricao"], amount("valor"), today.Format("02/01/2006"), args["categoria"])
		}
	case "criar_meta":
		answer = fmt.Sprintf([]string{"A meta **%s** tem objetivo de %s, ainda sem prazo definido.", "Quer começar **%s**, com alvo de %s? O cartão não estabelece uma data limite.", "Para **%s**, o objetivo proposto é %s; falta escolher o prazo, se desejar."}[index], args["nome"], amount("valor"))
	case "criar_divida":
		answer = fmt.Sprintf([]string{"O cadastro de **%s** prevê saldo de %s, juros de %s%% ao mês e parcela mínima de %s.", "Revise **%s**: o saldo informado é %s, a taxa mensal é %s%% e o mínimo é %s.", "A nova dívida **%s** ficou com %s de saldo; cada mês tem taxa de %s%% e mínimo de %s."}[index], args["nome"], amount("saldo"), args["juros_mes"], amount("parcela_minima"))
	case "registrar_aporte":
		answer = fmt.Sprintf([]string{"Bom avanço: **%s** para a meta %s ficou pronto no cartão, sem alterar o saldo ainda.", "Sua meta pode dar mais um passo: o aporte de **%s** em %s está para revisar.", "Mais perto de %s: há um aporte de **%s** aguardando sua conferência."}[index], amount("valor"), args["meta"])
		if index == 2 {
			answer = fmt.Sprintf("Mais perto de %s: há um aporte de **%s** aguardando sua conferência.", args["meta"], amount("valor"))
		}
	case "registrar_pagamento":
		if len(proposals) != 2 {
			panic("pagamento sem os dois cartões esperados")
		}
		answer = fmt.Sprintf([]string{"O pagamento de **%s** em %s preparou dois cartões: baixa do saldo e lançamento do gasto em Pagamento de dívidas.", "São dois cartões para revisar o pagamento de **%s** em %s: um reduz a dívida, outro registra a saída em Pagamento de dívidas.", "Ao confirmar os dois cartões de **%s** para %s, você baixa a dívida e inclui o gasto na categoria Pagamento de dívidas."}[index], amount("valor"), args["divida"])
	case "criar_conta_fixa":
		answer = fmt.Sprintf([]string{"A conta **%s de %s**, todo dia %v, ficou pronta para revisão; a antiga continua cadastrada.", "Sua nova conta **%s** terá valor de %s no dia %v; o cadastro anterior não foi excluído.", "Para **%s**, ficou preparado o gasto mensal de %s, com vencimento no dia %v; nenhum cadastro antigo será removido."}[index], args["descricao"], amount("valor"), args["dia"])
	case "alterar_lancamento", "apagar_lancamento":
		transaction := currentPurchase(book, today.Format("2006-01"))
		parsed, err := time.Parse(domain.DateLayout, transaction.Date)
		if err != nil {
			panic(err)
		}
		date := parsed.Format("02/01/2006")
		if operation == "alterar_lancamento" {
			answer = fmt.Sprintf([]string{"A compra **%s**, de %s, passaria de %s para %s.", "A correção preparada é de **%s para %s**, na compra %s em %s.", "Revise a compra de %s: **%s** tinha valor de %s; o novo será %s."}[index], transaction.Description, date, money.Format(transaction.AmountCents), amount("valor"))
			if index == 1 {
				answer = fmt.Sprintf("A correção preparada é de **%s para %s**, na compra %s em %s.", money.Format(transaction.AmountCents), amount("valor"), transaction.Description, date)
			} else if index == 2 {
				answer = fmt.Sprintf("Revise a compra de %s: **%s** tinha valor de %s; o novo será %s.", date, transaction.Description, money.Format(transaction.AmountCents), amount("valor"))
			}
		} else {
			answer = fmt.Sprintf([]string{"A exclusão preparada é da compra **%s de %s**, em %s.", "Ficou para confirmar a remoção de **%s**, compra em %s de %s.", "Quer retirar **%s de %s**, lançado em %s? O cartão aponta essa compra."}[index], transaction.Description, money.Format(transaction.AmountCents), date)
			if index == 1 {
				answer = fmt.Sprintf("Ficou para confirmar a remoção de **%s**, compra em %s de %s.", money.Format(transaction.AmountCents), date, transaction.Description)
			} else if index == 2 {
				answer = fmt.Sprintf("Quer retirar **%s de %s**, lançado em %s? O cartão aponta essa compra.", money.Format(transaction.AmountCents), transaction.Description, date)
			}
		}
	default:
		panic("proposta sem resposta: " + operation)
	}
	return answer + []string{" Confira e confirme no app; nada foi gravado.", " A mudança só é salva após sua confirmação no app.", " Falta sua confirmação para aplicar; os dados atuais foram preservados."}[index]
}
