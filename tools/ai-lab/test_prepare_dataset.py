import copy
import unittest

import prepare_dataset


def sample(identifier="exemplo", family="grupo", split="train"):
    return {"id": identifier, "family": family, "split": split,
            "synthetic": True, "review": "pendente",
            "messages": [{"role": "system", "content": "Instruções."},
                         {"role": "user", "content": "Pergunta fictícia " + identifier},
                         {"role": "assistant", "content": "Resposta fictícia."}], "tools": []}


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
    def test_deduplicacao_usa_pergunta_atual_e_contexto_do_perfil(self):
        rows = [sample("treino"), sample("validacao", "validacao", "valid"), sample("teste", "teste", "test")]
        for index, row in enumerate(rows):
            row["messages"][1]["content"] = "Olá, quero organizar o mês."
            row["messages"] += [{"role": "user", "content": "<dados_financeiros>Perfil " + str(index) + "</dados_financeiros> Quanto gastei?"},
                                {"role": "assistant", "content": "Resposta contextualizada."}]
        self.assertEqual(len(prepare_dataset.validate(rows)["valid"]), 1)

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
