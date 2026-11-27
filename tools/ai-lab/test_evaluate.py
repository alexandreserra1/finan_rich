import json
import io
import subprocess
import unittest
from unittest.mock import patch

import evaluate


def event(action, output="", test="TestEval"):
    return json.dumps({"Action": action, "Output": output, "Test": test})


class EvaluationTests(unittest.TestCase):
    def test_aceita_somente_avaliacao_executada_sem_falhas(self):
        log = "\n".join([
            event("output", 'OK    "pergunta" []\nresposta'),
            event("output", "NOTA: 4 de 4 critérios (100%)"),
            event("pass"),
        ])
        self.assertTrue(evaluate.summarize(log, 0)["passed"])

    def test_reprova_criterios_mesmo_com_go_test_aprovado(self):
        log = "\n".join([
            event("output", 'FALHA "pergunta" [faltou fonte]'),
            event("output", "NOTA: 3 de 4 critérios (75%)"),
            event("pass"),
        ])
        self.assertFalse(evaluate.summarize(log, 0)["passed"])

    def test_reprova_skip_ausencia_de_nota_ou_saida_com_erro(self):
        self.assertFalse(evaluate.summarize(event("skip"), 0)["passed"])
        self.assertFalse(evaluate.summarize(event("pass"), 0)["passed"])
        log = event("output", "NOTA: 4 de 4 critérios (100%)")
        self.assertFalse(evaluate.summarize(log, 1)["passed"])

    def test_reprova_sem_casos_e_nota_vazia(self):
        log = event("output", "NOTA: 0 de 0 critérios (0%)")
        self.assertFalse(evaluate.summarize(log, 0)["passed"])

    def test_remove_configuracao_de_avaliacao_herdada(self):
        env = evaluate.local_environment({
            "PATH": "/bin", "FINAN_EVAL_PRESET": "groq",
            "FINAN_EVAL_KEY": "segredo-ficticio", "FINAN_EVAL_TEAM": "1",
            "FINAN_EVAL_ONLY": "nenhum",
        }, "qwen3.5:4b")
        self.assertEqual(env["FINAN_EVAL_PRESET"], "ollama")
        self.assertEqual(env["FINAN_EVAL_KEY"], "laboratorio-local-sem-segredo")
        self.assertNotIn("FINAN_EVAL_TEAM", env)
        self.assertEqual(env["FINAN_EVAL_MODEL"], "qwen3.5:4b")

    def test_rejeita_linha_incompleta_excessiva_do_stream(self):
        with self.assertRaises(ValueError):
            evaluate.read_event(io.BytesIO(b"x" * 65537))

    def test_smoke_tem_prazo_externo_ao_stream(self):
        with patch("evaluate.subprocess.run", side_effect=subprocess.TimeoutExpired("smoke", 95)) as run:
            with self.assertRaises(subprocess.TimeoutExpired):
                evaluate.bounded_smoke("qwen3.5:4b")
            self.assertEqual(run.call_args.kwargs["timeout"], 95)

    def test_limpeza_e_executada_mesmo_quando_avaliacao_falha(self):
        with patch("evaluate.metadata", return_value={"digest": "revisao"}), \
                patch("evaluate.unload") as unload, \
                patch("evaluate.bounded_smoke", side_effect=RuntimeError("falha controlada")):
            result = evaluate.run_model("qwen3.5:4b", "revisao", evaluate.OUTPUT)
            self.assertIn("falha controlada", result["error"])
            self.assertEqual(unload.call_count, 2)

    def test_falha_ao_descarregar_nao_e_aprovada(self):
        with patch("evaluate.unload", side_effect=RuntimeError("modelo ocupado")):
            result = evaluate.run_model("qwen3.5:4b", "revisao", evaluate.OUTPUT)
            self.assertIn("cleanup_error", result)

    def test_aguarda_descarregamento_assincrono_do_ollama(self):
        responses = [{"done": True}, {"models": [{"name": "qwen3.5:4b"}]}, {"models": []}]
        with patch("evaluate.request", side_effect=responses), patch("evaluate.time.sleep") as sleep:
            evaluate.unload("qwen3.5:4b")
            sleep.assert_called_once_with(0.05)

    def test_descarregamento_persistente_tem_prazo(self):
        with patch("evaluate.request", return_value={"models": [{"name": "qwen3.5:4b"}]}), \
                patch("evaluate.time.monotonic", side_effect=[0, 1, 6]), \
                patch("evaluate.time.sleep"):
            with self.assertRaises(RuntimeError):
                evaluate.unload("qwen3.5:4b")

    def test_hashes_incluem_manifesto_dominio_e_dependencias(self):
        hashes = evaluate.source_hashes()
        for path in ("tools/ai-lab/models.json", "go.mod", "go.sum"):
            self.assertIn(path, hashes)
        self.assertTrue(any(path.startswith("internal/domain/") for path in hashes))


if __name__ == "__main__":
    unittest.main()
