import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import prepare_lora
from prepare_dataset import digest, encode_rows
from test_prepare_dataset import sample, tool_sample


class LoRAPreparationTests(unittest.TestCase):
    def test_preserva_erro_explicito_e_metadados_de_auditoria(self):
        source = tool_sample()
        source.update(executor_error=True, case_kind="executor_error", profile="tito", app_answer="Falhou.")
        source["messages"][3].update(error=True, content="Argumentos inválidos.")
        source["messages"][-1]["content"] = "Não foi possível consultar."
        files, _ = prepare_lora.build_files({"train": [source]})
        audit = json.loads(files["audit.jsonl"].splitlines()[0])
        self.assertIs(audit["executor_error"], True)
        self.assertEqual(audit["case_kind"], "executor_error")
        final = json.loads(files["train.jsonl"].splitlines()[-1])
        self.assertIs(final["messages"][3]["error"], True)
    def test_extrai_cada_assistente_multivolta_e_classifica_chamada_real(self):
        source = tool_sample()
        source["messages"].extend([{"role": "user", "content": "E agora?"},
                                    {"role": "assistant", "content": "Confira os registros."}])
        self.assertEqual([len(row["messages"]) for row in prepare_lora.targets(source)], [3, 5, 7])
        offered = sample()
        offered["tools"] = source["tools"]
        files, counts = prepare_lora.build_files({"train": [offered, source]})
        audit = [json.loads(line) for line in files["audit.jsonl"].splitlines()]
        self.assertEqual([row["target"] for row in audit], ["resposta", "chamada", "resposta", "resposta"])
        self.assertEqual(counts["train"], 4)

    def test_separa_chamada_e_resposta_sem_mudar_ferramentas(self):
        source = tool_sample()
        rows = prepare_lora.targets(source)
        self.assertEqual(len(rows), 2)
        self.assertEqual(rows[0]["messages"], source["messages"][:3])
        self.assertEqual(rows[1]["messages"], source["messages"])
        self.assertEqual(rows[0]["tools"], source["tools"])
        self.assertNotIn("id", rows[0])

    def test_nao_duplica_conversa_sem_ferramenta(self):
        self.assertEqual(len(prepare_lora.targets(sample())), 1)

    def test_tokenizer_desliga_raciocinio_e_rejeita_override(self):
        class Tokenizer:
            def apply_chat_template(self, *args, **kwargs):
                return kwargs
        wrapped = prepare_lora.NoThinkingTokenizer(Tokenizer())
        self.assertIs(wrapped.apply_chat_template([])["enable_thinking"], False)
        self.assertIs(wrapped.apply_chat_template([], enable_thinking=True)["enable_thinking"], False)

    def test_mascara_exige_prefixo_exato_e_resposta_nao_vazia(self):
        self.assertEqual(prepare_lora.mask_lengths([1, 2, 3], [1, 2]), (3, 2))
        with self.assertRaises(ValueError):
            prepare_lora.mask_lengths([1, 3, 4], [1, 2])
        with self.assertRaises(ValueError):
            prepare_lora.mask_lengths([1, 2], [1, 2])

    def test_rejeita_manifesto_com_caminho_externo(self):
        with self.assertRaises(ValueError):
            prepare_lora.verify_files(Path("."), {"../../.env": "hash"})

    def test_rejeita_arquivo_alterado_ou_link(self):
        with tempfile.TemporaryDirectory() as folder:
            directory = Path(folder)
            path = directory / "train.jsonl"
            path.write_bytes(b"corpus")
            with self.assertRaises(ValueError):
                prepare_lora.verify_files(directory, {"train.jsonl": digest(b"outro")})
            path.unlink()
            path.symlink_to("ausente")
            with self.assertRaises(ValueError):
                prepare_lora.verify_files(directory, {"train.jsonl": digest(b"corpus")})

    def test_rejeita_exportacao_que_nao_corresponde_a_auditoria(self):
        rows = [sample("treino"), sample("validacao", "validacao", "valid"),
                sample("teste", "teste", "test")]
        split = {row["split"]: [row] for row in rows}
        files = {name + ".jsonl": encode_rows([{"messages": data[0]["messages"], "tools": []}])
                 for name, data in split.items()}
        files["train.jsonl"] = encode_rows([{"messages": [], "tools": []}])
        with self.assertRaises(ValueError):
            prepare_lora.check_exports(split, files)

    def test_confere_alvos_contra_a_fonte_e_rejeita_adulteracao(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            source = root / "corpus"
            bundle = root / "lora"
            source.mkdir()
            bundle.mkdir()
            records = [tool_sample(), sample("validacao", "validacao", "valid"),
                       sample("teste", "teste", "test")]
            splits = {row["split"]: [row] for row in records}
            source_files = {name + ".jsonl": encode_rows([{"messages": rows[0]["messages"],
                                                        "tools": rows[0]["tools"]}])
                            for name, rows in splits.items()}
            source_files["audit.jsonl"] = encode_rows(records)
            source_manifest = {"synthetic": True, "training_allowed": False,
                               "status": "revisao_humana_pendente", "count": 3,
                               "files": {name: digest(value) for name, value in source_files.items()},
                               "splits": {name: {"count": 1, "ids": [rows[0]["id"]],
                                                 "families": [rows[0]["family"]]} for name, rows in splits.items()}}
            for name, value in source_files.items():
                (source / name).write_bytes(value)
            (source / "manifest.json").write_text(json.dumps(source_manifest))
            files, counts = prepare_lora.build_files(splits)
            manifest = {"status": "revisao_humana_pendente", "training_allowed": False,
                        "mask_prompt": True, "enable_thinking": False, "source_dataset": "corpus",
                        "source_manifest_sha256": digest((source / "manifest.json").read_bytes()),
                        "splits": counts, "files": {name: digest(value) for name, value in files.items()}}
            for name, value in files.items():
                (bundle / name).write_bytes(value)
            (bundle / "manifest.json").write_text(json.dumps(manifest))
            with patch("prepare_lora.OUTPUT", root), patch("prepare_lora.ROOT", root):
                loaded, metadata = prepare_lora.load_lora_bundle(bundle)
                self.assertEqual(loaded, files)
                self.assertEqual(metadata["splits"], {"train": 2, "valid": 1, "test": 1})
                (bundle / "train.jsonl").write_bytes(b"alterado")
                manifest["files"]["train.jsonl"] = digest(b"alterado")
                (bundle / "manifest.json").write_text(json.dumps(manifest))
                with self.assertRaises(ValueError):
                    prepare_lora.load_lora_bundle(bundle)


if __name__ == "__main__":
    unittest.main()
