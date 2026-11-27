import argparse
from contextlib import contextmanager
import importlib.metadata
import json
import math
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from evaluate import OUTPUT, ROOT, source_hashes
from prepare_dataset import digest
from prepare_lora import NoThinkingTokenizer, load_lora_bundle, mask_lengths


MODEL_REFERENCE = Path(__file__).parent / "runtime-model.json"


def runtime_versions():
    versions = {}
    for line in (Path(__file__).parent / "requirements-mlx.lock").read_text().splitlines():
        name, expected = line.split("==")
        actual = importlib.metadata.version(name)
        if actual != expected:
            raise ValueError("Versão do runtime divergente: " + name)
        versions[name] = actual
    return versions


def tokenizer_files():
    reference = json.loads(MODEL_REFERENCE.read_text())
    folder = OUTPUT / "qwen35-tokenizer"
    verified = {}
    for name, expected in reference["files"].items():
        if Path(name).name != name:
            raise ValueError("Nome de arquivo do tokenizer inválido.")
        path = folder / name
        if path.is_symlink() or not path.is_file() or path.stat().st_size > 32 * 1024 * 1024:
            raise ValueError("Arquivo do tokenizer ausente ou excessivo: " + name)
        payload = path.read_bytes()
        if digest(payload) != expected:
            raise ValueError("Hash do tokenizer divergente: " + name)
        verified[name] = payload
    return verified, reference


@contextmanager
def tokenizer_snapshot():
    files, reference = tokenizer_files()
    with tempfile.TemporaryDirectory(prefix="verified-tokenizer-", dir=OUTPUT) as temporary:
        folder = Path(temporary)
        for name, payload in files.items():
            (folder / name).write_bytes(payload)
        yield folder, reference


def check_adapter_paths(paths):
    if set(paths) != {"lora_a", "lora_b"}:
        raise ValueError("Gradientes fora dos adaptadores esperados.")


def numerical_lora_smoke():
    import mlx.core as mx
    import mlx.nn as nn
    import mlx.optimizers as optim
    from mlx_lm.tuner.lora import LoRALinear
    from mlx.utils import tree_flatten

    if not mx.metal.is_available():
        raise RuntimeError("Metal não está disponível neste ambiente.")
    mx.set_memory_limit(256 * 1024 * 1024)
    mx.set_cache_limit(32 * 1024 * 1024)
    mx.reset_peak_memory()
    mx.random.seed(42)
    linear = nn.Linear(8, 8, bias=False)
    linear.freeze()
    layer = LoRALinear.from_base(linear, r=2, scale=1.0)
    base_weight = mx.array(linear.weight)
    inputs = mx.ones((2, 8))
    expected = mx.zeros((2, 8))

    def loss(model):
        return mx.mean(mx.square(model(inputs) - expected))

    initial = float(loss(layer).item())
    value_and_grad = nn.value_and_grad(layer, loss)
    optimizer = optim.SGD(learning_rate=0.01)
    for step in range(3):
        value, gradients = value_and_grad(layer)
        check_adapter_paths([name for name, value in tree_flatten(gradients)])
        optimizer.update(layer, gradients)
        mx.eval(layer.parameters(), optimizer.state, value)
    final = float(loss(layer).item())
    if not math.isfinite(final) or final >= initial or not mx.array_equal(base_weight, layer.linear.weight).item():
        raise ValueError("Smoke LoRA não reduziu a perda ou alterou a camada-base.")
    return {"scope": "camada aleatória 8x8; não é treinamento do Qwen", "steps": 3,
            "initial_loss": initial, "final_loss": final, "base_unchanged": True,
            "peak_memory_bytes": mx.get_peak_memory(), "adapters_exported": False}


def check(directory):
    before = source_hashes()
    files, manifest = load_lora_bundle(directory)
    bundle_hash = digest((directory / "manifest.json").read_bytes())
    versions = runtime_versions()
    from transformers import AutoTokenizer
    from mlx_lm.models.qwen3_5 import ModelArgs
    from mlx_lm.tuner.datasets import ChatDataset

    counts = {}
    lengths = []
    with tokenizer_snapshot() as (folder, reference):
        ModelArgs.from_dict(json.loads((folder / "config.json").read_text()))
        tokenizer = NoThinkingTokenizer(AutoTokenizer.from_pretrained(folder, local_files_only=True, trust_remote_code=False))
        for split in ("train", "valid", "test"):
            rows = [json.loads(line) for line in files[split + ".jsonl"].splitlines() if line]
            counts[split] = len(rows)
            for row in rows:
                tokens, offset = ChatDataset([row], tokenizer, mask_prompt=True).process(row)
                prefix = tokenizer.apply_chat_template(row["messages"][:-1], tools=row["tools"],
                                                       add_generation_prompt=True, return_dict=False)
                size, expected_offset = mask_lengths(tokens, prefix)
                if offset != expected_offset or size > 8192:
                    raise ValueError("Máscara incompatível ou exemplo acima de 8192 tokens.")
                lengths.append(size)
    licenses = []
    for name in versions:
        metadata = importlib.metadata.metadata(name)
        licenses.append({"package": name, "license": metadata.get("License-Expression") or
                         metadata.get("License", "")[:200],
                         "classifiers": metadata.get_all("Classifier", [])})
    smoke = numerical_lora_smoke()
    after_files, after_manifest = load_lora_bundle(directory)
    tokenizer_files()
    if before != source_hashes() or after_files != files or after_manifest != manifest:
        raise ValueError("Entradas ou fontes mudaram durante a verificação.")
    return {"status": "compatibilidade_verificada_treino_bloqueado", "training_allowed": False,
            "dataset": str(directory.resolve().relative_to(ROOT)), "splits": counts,
            "lora_manifest_sha256": bundle_hash,
            "source_dataset_sha256": manifest["source_manifest_sha256"], "runtime": versions,
            "tokenizer": reference, "max_tokens": max(lengths), "min_tokens": min(lengths),
            "all_prefixes_aligned": True, "enable_thinking": False,
            "numerical_smoke": smoke, "model_weights_loaded": False,
            "full_model_training_validated": False, "license_policy_status": "revisao_pendente",
            "licenses": licenses, "source_hashes": source_hashes()}


def main():
    parser = argparse.ArgumentParser(description="Verifica template e Metal sem treinar o Qwen.")
    parser.add_argument("dataset", type=Path)
    args = parser.parse_args()
    OUTPUT.mkdir(parents=True, exist_ok=True)
    directory = Path(tempfile.mkdtemp(prefix="mlx-probe-", dir=OUTPUT))
    try:
        report = check(args.dataset)
    except Exception as error:
        report = {"status": "blocked", "training_allowed": False, "error": str(error)}
    return save_report(report, directory)


def save_report(report, directory=None):
    OUTPUT.mkdir(parents=True, exist_ok=True)
    directory = directory or Path(tempfile.mkdtemp(prefix="mlx-probe-", dir=OUTPUT))
    (directory / "report.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"report": str((directory / "report.json").relative_to(ROOT)),
                      "status": report["status"]}, ensure_ascii=False))
    return 1 if report["status"] == "blocked" else 0


def bounded_probe(arguments):
    return subprocess.run([sys.executable, __file__, "--worker", *arguments],
                          cwd=ROOT, text=True, capture_output=True, timeout=120)


def execute_probe(arguments):
    try:
        result = bounded_probe(arguments)
    except subprocess.TimeoutExpired:
        return save_report({"status": "blocked", "training_allowed": False,
                            "error": "Verificação excedeu 120 segundos; processo interrompido."})
    sys.stdout.write(result.stdout)
    sys.stderr.write(result.stderr)
    return result.returncode


if __name__ == "__main__":
    os.umask(0o077)
    if len(sys.argv) > 1 and sys.argv[1] == "--worker":
        del sys.argv[1]
        sys.exit(main())
    sys.exit(execute_probe(sys.argv[1:]))
