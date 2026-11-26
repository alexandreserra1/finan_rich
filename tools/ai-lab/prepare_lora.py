import argparse
import json
import os
from pathlib import Path
import tempfile

from evaluate import OUTPUT, ROOT, source_hashes
from prepare_dataset import digest, encode_rows, executor_failure, validate


DATA_FILES = {"train.jsonl", "valid.jsonl", "test.jsonl", "audit.jsonl"}


class NoThinkingTokenizer:
    def __init__(self, tokenizer):
        self.tokenizer = tokenizer

    def apply_chat_template(self, *args, **kwargs):
        kwargs["enable_thinking"] = False
        return self.tokenizer.apply_chat_template(*args, **kwargs)


def mask_lengths(tokens, prefix):
    if not prefix or tokens[:len(prefix)] != prefix or len(tokens) <= len(prefix):
        raise ValueError("Máscara desalinhada ou sem alvo de treinamento.")
    return len(tokens), len(prefix)


def targets(row):
    tools = row["tools"]
    messages = row["messages"]
    failed_calls = {message["tool_call_id"] for message in messages
                    if message["role"] == "tool"
                    and (message.get("error") is True or executor_failure(message.get("content", "")))}
    return [{"messages": messages[:index + 1], "tools": tools}
            for index, message in enumerate(messages) if message["role"] == "assistant"
            and not any(call["id"] in failed_calls for call in message.get("tool_calls", []))]


def read_bytes(path):
    if path.is_symlink() or not path.is_file() or path.stat().st_size > 8 * 1024 * 1024:
        raise ValueError("Arquivo ausente, link ou tamanho excessivo: " + str(path))
    return path.read_bytes()


def verify_files(directory, files):
    if not files or set(files) - DATA_FILES:
        raise ValueError("Manifesto contém nomes de arquivos não permitidos.")
    result = {}
    for name, expected in files.items():
        payload = read_bytes(directory / name)
        if digest(payload) != expected:
            raise ValueError("Hash divergente: " + name)
        result[name] = payload
    return result


def check_exports(splits, files):
    for split, rows in splits.items():
        expected = encode_rows([{"messages": row["messages"], "tools": row["tools"]} for row in rows])
        if files[split + ".jsonl"] != expected:
            raise ValueError("Exportação não corresponde à auditoria: " + split)


def load_corpus(directory):
    if directory.is_symlink() or not directory.resolve().is_relative_to(OUTPUT.resolve()):
        raise ValueError("Use uma exportação local dentro de .artifacts/ai-lab.")
    payload = read_bytes(directory / "manifest.json")
    manifest = json.loads(payload)
    if (set(manifest["files"]) != DATA_FILES or manifest.get("synthetic") is not True
            or manifest.get("training_allowed") is not False
            or manifest.get("status") != "revisao_humana_pendente"):
        raise ValueError("Estado inesperado do corpus inicial; não há liberação automática.")
    files = verify_files(directory, manifest["files"])
    rows = [json.loads(line) for line in files["audit.jsonl"].splitlines() if line]
    splits = validate(rows)
    check_exports(splits, files)
    if manifest["count"] != len(rows):
        raise ValueError("Contagem divergente no manifesto.")
    for split, records in splits.items():
        expected = {"count": len(records), "ids": [row["id"] for row in records],
                    "families": sorted({row["family"] for row in records})}
        if manifest["splits"].get(split) != expected:
            raise ValueError("Identificação ou famílias divergentes: " + split)
    return splits, digest(payload)


def build_files(splits):
    files, counts = {}, {}
    audit = []
    for split, records in splits.items():
        expanded = []
        for row in records:
            for target in targets(row):
                expanded.append(target)
                audit.append({"source_id": row["id"], "family": row["family"], "split": split,
                              "target": "chamada" if target["messages"][-1].get("tool_calls") else "resposta",
                              "index_in_split": len(expanded)-1, "review": row["review"],
                              **{key: row[key] for key in ("executor_error", "case_kind") if key in row}})
        files[split + ".jsonl"] = encode_rows(expanded)
        counts[split] = len(expanded)
    files["audit.jsonl"] = encode_rows(audit)
    return files, counts


def load_lora_bundle(directory):
    if directory.is_symlink() or not directory.resolve().is_relative_to(OUTPUT.resolve()):
        raise ValueError("Use um conjunto LoRA local dentro de .artifacts/ai-lab.")
    manifest = json.loads(read_bytes(directory / "manifest.json"))
    if (set(manifest["files"]) != DATA_FILES or manifest.get("training_allowed") is not False
            or manifest.get("status") != "revisao_humana_pendente"
            or manifest.get("mask_prompt") is not True or manifest.get("enable_thinking") is not False):
        raise ValueError("Conjunto LoRA sem as restrições esperadas.")
    files = verify_files(directory, manifest["files"])
    source, source_hash = load_corpus(ROOT / manifest["source_dataset"])
    expected, counts = build_files(source)
    if source_hash != manifest["source_manifest_sha256"] or files != expected or counts != manifest["splits"]:
        raise ValueError("Alvos LoRA não correspondem à fonte auditada.")
    return files, manifest


def main():
    parser = argparse.ArgumentParser(description="Prepara alvos LoRA sem iniciar treinamento.")
    parser.add_argument("dataset", type=Path)
    args = parser.parse_args()
    splits, manifest_hash = load_corpus(args.dataset)
    files, counts = build_files(splits)
    directory = Path(tempfile.mkdtemp(prefix="lora-data-", dir=OUTPUT))
    manifest = {"status": "revisao_humana_pendente", "training_allowed": False,
                "source_dataset": str(args.dataset.resolve().relative_to(ROOT)),
                "source_manifest_sha256": manifest_hash, "source_hashes": source_hashes(),
                "mask_prompt": True, "enable_thinking": False, "files": {}, "splits": counts}
    for name, payload in files.items():
        (directory / name).write_bytes(payload)
        manifest["files"][name] = digest(payload)
    (directory / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"directory": str(directory.relative_to(ROOT)), "splits": manifest["splits"],
                      "training_allowed": False}, ensure_ascii=False))


if __name__ == "__main__":
    os.umask(0o077)
    main()
