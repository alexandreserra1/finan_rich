import copy
import unittest

import prepare_dataset


def sample(identifier="exemplo", family="grupo", split="train"):
    return {"id": identifier, "family": family, "split": split,
            "synthetic": True, "review": "pendente",
            "messages": [{"role": "system", "content": "Instruções."},
                         {"role": "user", "content": "Pergunta fictícia " + identifier},
                         {"role": "assistant", "content": "Resposta fictícia " + identifier + "."}], "tools": []}


def tool_sample():
    row = sample()
    row["tools"] = [{"type": "function", "function": {"name": "consulta", "parameters": {
        "properties": {"mes": {"type": "string"}}, "required": ["mes"]}}}]
    row["messages"][2:2] = [
        {"role": "assistant", "tool_calls": [{"id": "chamada", "function": {
            "name": "consulta", "arguments": {"mes": "2026-10"}}}]},
        {"role": "tool", "tool_call_id": "chamada", "name": "consulta", "content": "Resultado fictício."},
    ]
    return row


class DatasetTests(unittest.TestCase):
    def test_deduplicacao_cobre_todos_assistentes_textuais_inclusive_historico(self):
        for masked in (False, True):
            rows = [sample("treino"), sample("validacao", "validacao", "valid")]
            for index, row in enumerate(rows):
                amount = 100 + index if masked else 100
                row["messages"][1:1] = [{"role": "user", "content": "Minha dúvida antiga."},
                    {"role": "assistant", "content": f"Seu saldo disponível é R$ {amount},00."}]
            with self.subTest(masked=masked):
                findings = prepare_dataset.duplicate_findings(rows)
                self.assertTrue(any(item["issue"] == "duplicata_resposta" for item in findings))

    def test_excecao_curta_exige_recusa_inteira_sem_dinheiro_ou_datas(self):
        for text in ("Não há atrasos; saldo disponível R$ 100,00", "Não posso consultar. Seu saldo é R$ 100,00.",
                     "Não tenho acesso em outubro.", "Não consigo consultar em 2026-10-22."):
            first, second = sample(), sample("outra", "outra", "test")
            first["messages"][-1]["content"] = second["messages"][-1]["content"] = text
            with self.subTest(text=text):
                self.assertTrue(prepare_dataset.duplicate_findings([first, second]))

    def mutation_sample(self, operation="alterar_lancamento"):
        row = tool_sample()
        row["tools"][0]["function"]["name"] = "buscar_lancamentos"
        row["messages"][2]["tool_calls"][0]["function"]["name"] = "buscar_lancamentos"
        row["messages"][3].update(name="buscar_lancamentos", content="- [origem-42] Compra de R$ 20.")
        row["tools"].append({"type": "function", "function": {"name": operation, "parameters": {
            "properties": {"id": {"type": "string"}}, "required": ["id"]}}})
        row["messages"][4:4] = [
            {"role": "assistant", "tool_calls": [{"id": "alteracao", "function": {
                "name": operation, "arguments": {"id": "origem-42"}}}]},
            {"role": "tool", "tool_call_id": "alteracao", "name": operation, "content": "Pronto para confirmar."}]
        return row

    def test_id_de_alteracao_e_exclusao_precisa_vir_de_busca_anterior(self):
        for operation in ("alterar_lancamento", "apagar_lancamento"):
            row = self.mutation_sample(operation)
            prepare_dataset.validate_conversation(row)
            for identifier in ("origem-4", "inexistente"):
                changed = copy.deepcopy(row)
                changed["messages"][4]["tool_calls"][0]["function"]["arguments"]["id"] = identifier
                with self.subTest(operation=operation, identifier=identifier), self.assertRaisesRegex(ValueError, "origem"):
                    prepare_dataset.validate_conversation(changed)

    def test_busca_errada_futura_ou_de_turno_anterior_nao_autoriza_id(self):
        for variant in ("outra_ferramenta", "mesma_rodada", "outro_turno", "erro"):
            row = self.mutation_sample()
            if variant == "outra_ferramenta":
                row["tools"][0]["function"]["name"] = "consulta"
                row["messages"][2]["tool_calls"][0]["function"]["name"] = "consulta"
                row["messages"][3]["name"] = "consulta"
            elif variant == "mesma_rodada":
                row["messages"][2]["tool_calls"].extend(row["messages"].pop(4)["tool_calls"])
            elif variant == "outro_turno":
                row["messages"][4:4] = [{"role": "assistant", "content": "Achei a compra."},
                                         {"role": "user", "content": "Agora outra compra."}]
            else:
                row["executor_error"] = True
                row["messages"][3]["content"] = "não deu: [origem-42]"
            with self.subTest(variant=variant), self.assertRaisesRegex(ValueError, "origem"):
                prepare_dataset.validate_conversation(row)

    def test_pergunta_atual_igual_entre_splits_vaza_mesmo_com_outro_perfil(self):
        rows = [sample("treino"), sample("validacao", "validacao", "valid"), sample("teste", "teste", "test")]
        for index, row in enumerate(rows):
            row["messages"][1]["content"] = "Olá, quero organizar o mês."
            row["messages"] += [{"role": "user", "content": "<dados_financeiros>Perfil " + str(index) + "</dados_financeiros> Quanto gastei?"},
                                {"role": "assistant", "content": "Resposta contextualizada."}]
        with self.assertRaisesRegex(ValueError, "Duplicata entre conjuntos"):
            prepare_dataset.validate(rows)

    def test_detecta_pergunta_e_alvo_separadamente_inclusive_moldes(self):
        for field in ("pergunta", "resposta"):
            for masked in (False, True):
                rows = [sample("treino"), sample("validacao", "validacao", "valid"), sample("teste", "teste", "test")]
                index = 1 if field == "pergunta" else -1
                text = "O pagamento no Banco Azul ficou em R$ 200,00 para outubro de 2026; confira os dados antes de confirmar."
                rows[0]["messages"][index]["content"] = text
                rows[1]["messages"][index]["content"] = text if not masked else text.replace("Banco Azul", "Banco Verde").replace("200,00", "350,00").replace("outubro", "março").replace("2026", "2027")
                rows[0]["dedup_names"], rows[1]["dedup_names"] = ["Banco Azul"], ["Banco Verde"]
                with self.subTest(field=field, masked=masked), self.assertRaisesRegex(ValueError, "Duplicata entre conjuntos.*" + field):
                    prepare_dataset.validate(rows)

    def test_recusas_e_continuidade_curtas_nao_sao_vazamento_substantivo(self):
        for answer in ("Qual é o saldo?", "Não posso revelar senhas.", "Certo, vamos continuar."):
            rows = [sample("treino"), sample("validacao", "validacao", "valid"), sample("teste", "teste", "test")]
            for row in rows:
                row["messages"][-1]["content"] = answer
            with self.subTest(answer=answer):
                prepare_dataset.validate(rows)

    def test_mascara_nomes_estruturados_sem_metadado_manual(self):
        for source in ("ledger", "arguments", "proposals"):
            first, second = sample(), sample("outro", "outra", "test")
            for row, name in ((first, "Banco Azul"), (second, "Banco Verde")):
                row["messages"][-1]["content"] = f"A dívida com {name} tem saldo de R$ 200,00 e parcela mínima de R$ 50,00 para este mês."
                if source == "ledger":
                    row["ledger"] = {"debts": [{"name": name}]}
                elif source == "proposals":
                    row["proposals"] = [{"record": {"name": name}}]
                else:
                    row["messages"].insert(2, {"role": "assistant", "tool_calls": [{"function": {
                        "name": "criar_divida", "arguments": {"nome": name}}}]})
            with self.subTest(source=source):
                self.assertTrue(prepare_dataset.duplicate_findings([first, second]))

    def test_nomes_literais_nao_mascaram_pedaços_de_outras_palavras(self):
        self.assertEqual(prepare_dataset.normalized_content("Azulão e Banco Azul (sul).", ["Banco Azul"]),
                         "azulao e nome sul")

    def test_id_de_alteracao_invalido_rejeitado_sem_typeerror(self):
        row = self.mutation_sample()
        row["messages"][4]["tool_calls"][0]["function"]["arguments"]["id"] = {"id": "origem-42"}
        with self.assertRaises(ValueError):
            prepare_dataset.validate_conversation(row)

    def test_molde_financeiro_curto_nao_e_excecao_de_recusa_ou_continuidade(self):
        rows = [sample("treino"), sample("validacao", "validacao", "valid"), sample("teste", "teste", "test")]
        for index, row in enumerate(rows):
            row["messages"][-1]["content"] = f"O total foi R$ {index + 10},00."
        with self.assertRaisesRegex(ValueError, "Duplicata entre conjuntos"):
            prepare_dataset.validate(rows)

    def test_mesma_pergunta_com_e_sem_taxas_permitida_so_no_mesmo_split(self):
        rows = [sample("com"), sample("sem", "outra"), sample("validacao", "validacao", "valid"), sample("teste", "teste", "test")]
        for row, context in zip(rows, ("Taxas datadas", "Sem taxas")):
            row["messages"][1]["content"] = f"<dados_financeiros>{context}</dados_financeiros> Qual a Selic informada?"
        prepare_dataset.validate(rows)
        rows[1]["split"] = "valid"
        with self.assertRaisesRegex(ValueError, "Duplicata entre conjuntos"):
            prepare_dataset.validate(rows)

    def test_historico_repetido_nao_e_pergunta_ou_alvo_atual(self):
        rows = [sample("treino"), sample("validacao", "validacao", "valid"), sample("teste", "teste", "test")]
        for row in rows:
            row["messages"][1:1] = [{"role": "user", "content": "Olá."},
                                      {"role": "assistant", "content": "Qual é o saldo?"}]
        prepare_dataset.validate(rows)

    def test_aceita_consulta_que_falhou_sem_confirmar_resultado(self):
        row = tool_sample()
        row["executor_error"] = True
        row["messages"][3]["content"] = "não deu: informe quantos dias olhar (1 a 31)"
        row["messages"][-1]["content"] = "A consulta falhou: escolha um prazo válido; nenhum vencimento foi confirmado."
        self.assertEqual(len(prepare_dataset.validate([
            row, sample("validacao", "validacao", "valid"), sample("teste", "teste", "test")
        ])["train"]), 1)

    def test_sem_erro_nao_reconhece_falha_do_executor(self):
        for text in ["Sem erro, registrei o pagamento.", "Não houve erro, registrei o pagamento."]:
            row = tool_sample()
            row["executor_error"] = True
            row["messages"][3]["content"] = "argumentos inválidos: informe o mês"
            row["messages"][-1]["content"] = text
            with self.subTest(text=text), self.assertRaises(ValueError):
                prepare_dataset.validate([row, sample("validacao", "validacao", "valid"),
                                          sample("teste", "teste", "test")])

    def validate_row(self, row):
        return prepare_dataset.validate([row, sample("validacao", "validacao", "valid"),
                                         sample("teste", "teste", "test")])

    def test_aceita_cardapio_sem_chamada_e_multivolta(self):
        row = sample()
        row["tools"] = tool_sample()["tools"]
        row["messages"][2:2] = [{"role": "assistant", "content": "Qual mês?"},
                                  {"role": "user", "content": "Outubro."}]
        self.assertEqual(len(self.validate_row(row)["train"]), 1)

    def test_aceita_duas_chamadas_seguidas_e_resultados_fora_de_ordem(self):
        row = tool_sample()
        second = copy.deepcopy(row["messages"][2])
        second["tool_calls"][0]["id"] = "segunda"
        result = copy.deepcopy(row["messages"][3])
        result["tool_call_id"] = "segunda"
        row["messages"].insert(3, second)
        row["messages"].insert(4, result)
        self.validate_row(row)

    def test_aceita_duas_chamadas_na_mesma_mensagem_e_rodadas_separadas(self):
        for batched in (False, True):
            row = tool_sample()
            call = copy.deepcopy(row["messages"][2]["tool_calls"][0])
            call["id"] = "segunda"
            result = copy.deepcopy(row["messages"][3])
            result["tool_call_id"] = "segunda"
            if batched:
                row["messages"][2]["tool_calls"].append(call)
                row["messages"].insert(4, result)
            else:
                row["messages"][4:4] = [{"role": "assistant", "tool_calls": [call]}, result]
            with self.subTest(batched=batched):
                self.validate_row(row)

    def test_rejeita_conteudo_nao_textual_inclusive_em_chamada(self):
        for index in range(5):
            row = tool_sample()
            row["messages"][index]["content"] = {"texto": "inválido"}
            with self.subTest(index=index), self.assertRaises(ValueError):
                self.validate_row(row)

    def test_aceita_erro_explicito_e_campos_raiz_auxiliares(self):
        row = tool_sample()
        row.update(executor_error=True, case_kind="executor_error", profile="tito", app_answer="Falhou.")
        row["messages"][3].update(error=True, content="Serviço indisponível.")
        row["messages"][-1]["content"] = "Não foi possível consultar. Tente novamente."
        self.validate_row(row)
        row["messages"][-1]["content"] = "Ocorreu uma falha, mas registrei o pagamento."
        with self.assertRaises(ValueError):
            self.validate_row(row)

    def test_rejeita_vinculo_duplicado_orfao_e_chamada_pendente(self):
        for field, value in [("tool_call_id", "outra"), ("name", "outra")]:
            row = tool_sample()
            row["messages"][3][field] = value
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.validate_row(row)
        for index in (2, 3):
            row = tool_sample()
            row["messages"].insert(index, copy.deepcopy(row["messages"][index]))
            with self.subTest(index=index), self.assertRaises(ValueError):
                self.validate_row(row)

    def test_erro_executor_exige_metadado_e_reconhecimento(self):
        for error in ["não deu: informe quantos dias olhar", "argumentos inválidos: JSON",
                      "ferramenta desconhecida: consulta", "essa ferramenta não é com você",
                      "não deu para calcular"]:
            row = tool_sample()
            row["messages"][3]["content"] = error
            with self.subTest(error=error):
                with self.assertRaises(ValueError):
                    self.validate_row(row)
                row["executor_error"] = True
                with self.assertRaises(ValueError):
                    self.validate_row(row)
                row["messages"][-1]["content"] = "Não foi possível consultar. Confira os argumentos e tente novamente."
                self.validate_row(row)

    def test_rejeita_argumentos_obrigatorios_enum_e_inteiro(self):
        for arguments in [{}, {"mes": 1}, {"mes": "outro"}, {"mes": True}]:
            row = tool_sample()
            row["tools"][0]["function"]["parameters"]["properties"]["mes"] = {"type": "integer", "enum": [1, 2]}
            row["messages"][2]["tool_calls"][0]["function"]["arguments"] = arguments
            if type(arguments.get("mes")) is int:
                self.validate_row(row)
            else:
                with self.subTest(arguments=arguments), self.assertRaises(ValueError):
                    self.validate_row(row)

    def test_aceita_corpus_sintetico_com_tres_conjuntos(self):
        rows = [sample("treino"), sample("validacao", "grupo-v", "valid"),
                sample("teste", "grupo-t", "test")]
        self.assertEqual(set(prepare_dataset.validate(rows)), {"train", "valid", "test"})

    def test_rejeita_mesma_familia_em_conjuntos_diferentes(self):
        with self.assertRaises(ValueError):
            prepare_dataset.validate([sample(), sample("outro", split="test")])

    def test_rejeita_duplicata_de_pergunta_normalizada(self):
        first = sample()
        second = sample("outro", "outro", "test")
        second["messages"][1]["content"] = "  PERGUNTA   FICTÍCIA exemplo  "
        with self.assertRaises(ValueError):
            prepare_dataset.validate([first, second])

    def test_rejeita_dados_nao_sinteticos(self):
        row = sample()
        row["synthetic"] = False
        with self.assertRaises(ValueError):
            prepare_dataset.validate([row])

    def test_rejeita_ferramenta_sem_chamada_anterior(self):
        row = sample()
        row["messages"].insert(2, {"role": "tool", "tool_call_id": "inexistente", "content": "123"})
        with self.assertRaises(ValueError):
            prepare_dataset.validate([row])

    def test_rejeita_id_duplicado(self):
        with self.assertRaises(ValueError):
            prepare_dataset.validate([sample(), copy.deepcopy(sample())])

    def test_valida_contrato_e_encadeamento_da_ferramenta(self):
        row = tool_sample()
        other = [sample("validacao", "validacao", "valid"), sample("teste", "teste", "test")]
        self.assertEqual(len(prepare_dataset.validate([row] + other)["train"]), 1)
        invalid = copy.deepcopy(row)
        invalid["messages"][2]["tool_calls"][0]["function"]["arguments"] = {"mes": 202610}
        with self.assertRaises(ValueError):
            prepare_dataset.validate([invalid] + other)

    def test_rejeita_resultado_da_ferramenta_que_nao_seja_texto(self):
        row = tool_sample()
        row["messages"][3]["content"] = {"resultado": "não é texto"}
        with self.assertRaises(ValueError):
            prepare_dataset.validate([row, sample("validacao", "validacao", "valid"),
                                      sample("teste", "teste", "test")])

    def test_rejeita_chamada_pendente_na_resposta_final(self):
        row = tool_sample()
        row["messages"][-1]["tool_calls"] = copy.deepcopy(row["messages"][2]["tool_calls"])
        with self.assertRaises(ValueError):
            prepare_dataset.validate([row, sample("validacao", "validacao", "valid"),
                                      sample("teste", "teste", "test")])
    def test_rejeita_argumento_desconhecido(self):
        invalid = tool_sample()
        invalid["messages"][2]["tool_calls"][0]["function"]["arguments"] = {"mes": "2026-10", "desconhecido": "valor"}
        with self.assertRaises(ValueError):
            prepare_dataset.validate([invalid, sample("validacao", "validacao", "valid"),
                                      sample("teste", "teste", "test")])


if __name__ == "__main__":
    unittest.main()
