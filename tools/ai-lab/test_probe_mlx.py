from contextlib import redirect_stdout
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import probe_mlx
from prepare_dataset import digest


class MLXProbeTests(unittest.TestCase):
    def test_gradientes_aceitam_somente_os_dois_adaptadores(self):
        probe_mlx.check_adapter_paths(["lora_a", "lora_b"])
        with self.assertRaises(ValueError):
            probe_mlx.check_adapter_paths(["lora_a", "lora_b", "linear.weight"])
        with self.assertRaises(ValueError):
            probe_mlx.check_adapter_paths([])

    def test_processo_da_verificacao_tem_prazo_externo(self):
        with patch("probe_mlx.subprocess.run", side_effect=subprocess.TimeoutExpired("probe", 120)) as run:
            with self.assertRaises(subprocess.TimeoutExpired):
                probe_mlx.bounded_probe(["conjunto-ficticio"])
            self.assertEqual(run.call_args.kwargs["timeout"], 120)

    def test_template_extra_nao_entra_no_snapshot_verificado(self):
        with tempfile.TemporaryDirectory() as folder:
            directory = Path(folder)
            source = directory / "qwen35-tokenizer"
            source.mkdir()
            (source / "chat_template.jinja").write_bytes(b"template revisado")
            (source / "additional_chat_templates").mkdir()
            (source / "additional_chat_templates" / "tool_use.jinja").write_bytes(b"template extra")
            reference = directory / "reference.json"
            reference.write_text(json.dumps({"files": {"chat_template.jinja": digest(b"template revisado")}}))
            with patch("probe_mlx.OUTPUT", directory), patch("probe_mlx.MODEL_REFERENCE", reference):
                with probe_mlx.tokenizer_snapshot() as (snapshot, metadata):
                    self.assertEqual({path.name for path in snapshot.iterdir()}, {"chat_template.jinja"})
                    self.assertEqual((snapshot / "chat_template.jinja").read_bytes(), b"template revisado")
                self.assertFalse(snapshot.exists())

    def test_timeout_grava_relatorio_bloqueado(self):
        with tempfile.TemporaryDirectory() as folder:
            directory = Path(folder)
            with patch("probe_mlx.OUTPUT", directory), patch("probe_mlx.ROOT", directory), \
                    patch("probe_mlx.bounded_probe", side_effect=subprocess.TimeoutExpired("probe", 120)), \
                    redirect_stdout(io.StringIO()):
                self.assertEqual(probe_mlx.execute_probe(["ficticio"]), 1)
            report = json.loads(next(directory.glob("mlx-probe-*/report.json")).read_text())
            self.assertEqual(report["status"], "blocked")
            self.assertIs(report["training_allowed"], False)


if __name__ == "__main__":
    unittest.main()
