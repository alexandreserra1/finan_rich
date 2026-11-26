import hashlib
import json
import os
import re
from pathlib import Path
import subprocess
import tempfile
import unicodedata

from evaluate import ROOT, OUTPUT, source_hashes


def digest(data):
    return hashlib.sha256(data).hexdigest()


def plain(text):
    return "".join(char for char in unicodedata.normalize("NFKD", text.casefold())
                   if not unicodedata.combining(char))


def executor_failure(text):
    return bool(re.search(r"\b(nao deu\b|argumentos invalidos\b|ferramenta desconhecida\b|essa ferramenta nao e com voce\b)", plain(text)))


def acknowledges_failure(text):
    normalized = plain(text)
    for clause in re.split(r"[.!?;\n]|\bmas\b|\bporem\b", normalized):
        for match in re.finditer(r"\b(registrei|transferi|cadastro feito|pagamento (?:registrado|efetuado)|sucesso)\b", clause):
            prefix = clause[:match.start()].rsplit(",", 1)[-1]
            if not re.search(r"\b(?:nao|nunca|jamais|sem (?:dizer|afirmar|declarar))\b(?:\W+\w+){0,6}\W*$", prefix):
                return False
    return bool(re.search(r"\b(nao (?:foi possivel|consegui|deu|consigo)|erro|falha|falhou|argumentos invalidos|tente novamente)\b", normalized))


def validate_conversation(row):
    identifier, messages = row["id"], row["messages"]
    if (not isinstance(messages, list) or len(messages) < 3
            or [message.get("role") for message in messages[:2]] != ["system", "user"]
            or messages[-1].get("role") != "assistant"):
        raise ValueError("Conversa incompleta: " + identifier)
    definitions = {}
    if not isinstance(row["tools"], list):
        raise ValueError("Cardápio inválido: " + identifier)
    for tool in row["tools"]:
        function = tool["function"]
        if tool.get("type") != "function" or not function.get("name") or function["name"] in definitions:
            raise ValueError("Definição duplicada ou inválida: " + identifier)
        definitions[function["name"]] = function
    if "executor_error" in row and type(row["executor_error"]) is not bool:
        raise ValueError("executor_error precisa ser booleano: " + identifier)
    if "case_kind" in row and (not isinstance(row["case_kind"], str) or not row["case_kind"].strip()):
        raise ValueError("case_kind precisa ser texto: " + identifier)
    pending, seen = {}, set()
    last_role, failure_pending, had_error = None, False, False
    for index, message in enumerate(messages):
        role, calls = message.get("role"), message.get("tool_calls", [])
        content = message.get("content")
        if "content" in message and not isinstance(content, str):
            raise ValueError("Conteúdo precisa ser textual: " + identifier)
        if not isinstance(calls, list) or ("tool_calls" in message and role != "assistant"):
            raise ValueError("Chamada fora do assistente: " + identifier)
        if not calls and (not isinstance(content, str) or not content.strip()):
            raise ValueError("Mensagem vazia: " + identifier)
        if role == "system":
            if index != 0:
                raise ValueError("Sistema fora do início: " + identifier)
        elif role == "user":
            if pending or failure_pending or last_role not in {"system", "assistant"}:
                raise ValueError("Turno de usuário inválido: " + identifier)
        elif role == "assistant":
            if last_role not in {"user", "tool", "assistant"} or (pending and not calls):
                raise ValueError("Resposta antes dos resultados: " + identifier)
            if last_role == "assistant" and not (pending and calls):
                raise ValueError("Turno duplicado do assistente: " + identifier)
            if not calls and failure_pending:
                if not acknowledges_failure(content):
                    raise ValueError("Erro do executor tratado como sucesso: " + identifier)
                failure_pending = False
            for call in calls:
                function = call["function"]
                call_id, name = call.get("id"), function.get("name")
                if not isinstance(call_id, str) or not call_id or call_id in seen or name not in definitions:
                    raise ValueError("Chamada duplicada ou fora do cardápio: " + identifier)
                arguments, schema = function["arguments"], definitions[name]["parameters"]
                if not isinstance(arguments, dict):
                    raise ValueError("Argumentos precisam ser um objeto: " + identifier)
                properties = schema.get("properties", {})
                if set(arguments) - set(properties) or set(schema.get("required", [])) - set(arguments):
                    raise ValueError("Argumentos fora do contrato da ferramenta: " + identifier)
                for key, value in arguments.items():
                    expected = properties[key]
                    types = {"string": str, "integer": int, "number": (int, float),
                             "boolean": bool, "object": dict, "array": list, "null": type(None)}
                    expected_type = expected.get("type")
                    if expected_type and (expected_type not in types or not isinstance(value, types[expected_type])
                                          or (expected_type in {"integer", "number"} and type(value) is bool)):
                        raise ValueError("Tipo de argumento inválido: " + identifier)
                    if "enum" in expected and value not in expected["enum"]:
                        raise ValueError("Argumento fora das opções: " + identifier)
                pending[call_id] = name
                seen.add(call_id)
        elif role == "tool":
            call_id = message.get("tool_call_id")
            if call_id not in pending or message.get("name") != pending[call_id]:
                raise ValueError("Resultado sem chamada correspondente: " + identifier)
            del pending[call_id]
            if "error" in message and type(message["error"]) is not bool:
                raise ValueError("error precisa ser booleano: " + identifier)
            if message.get("error") is True or executor_failure(content):
                if row.get("executor_error") is not True:
                    raise ValueError("Erro do executor sem auditoria: " + identifier)
                failure_pending, had_error = True, True
        else:
            raise ValueError("Papel inválido: " + identifier)
        last_role = role
    if pending or failure_pending or bool(row.get("executor_error")) != had_error:
        raise ValueError("Chamada pendente ou auditoria de erro divergente: " + identifier)


def validate(rows):
    groups, identifiers, questions = {}, set(), set()
    splits = {name: [] for name in ("train", "valid", "test")}
    for row in rows:
        identifier, family, split = row["id"], row["family"], row["split"]
        if not identifier or identifier in identifiers or not family or split not in splits:
            raise ValueError("Identificação, família ou conjunto inválido.")
        if row.get("synthetic") is not True or row.get("review") != "pendente":
            raise ValueError("O corpus inicial aceita somente exemplos fictícios com revisão pendente.")
        if family in groups and groups[family] != split:
            raise ValueError("Família presente em mais de um conjunto: " + family)
        groups[family] = split
        identifiers.add(identifier)
        messages = row["messages"]
        validate_conversation(row)
        current = next(message["content"] for message in reversed(messages) if message["role"] == "user")
        context, separator, question = current.rpartition("</dados_financeiros>")
        if not separator:
            context, question = "", current
        normalized = " ".join(re.sub(r"[^\w\s]", " ", plain(question)).split())
        identity = (normalized, digest(context.encode()))
        if identity in questions:
            raise ValueError("Pergunta repetida, inclusive após normalização: " + identifier)
        questions.add(identity)
        splits[split].append(row)
    if any(not values for values in splits.values()):
        raise ValueError("Treino, validação e teste precisam ter exemplos.")
    return splits


def encode_rows(rows):
    return ("\n".join(json.dumps(row, ensure_ascii=False, sort_keys=True) for row in rows) + "\n").encode()


def main():
    before = source_hashes()
    generated = subprocess.run(["go", "run", "./tools/ai-lab/corpusgen"], cwd=ROOT,
                               capture_output=True, timeout=60, check=True)
    rows = [json.loads(line) for line in generated.stdout.splitlines() if line]
    splits = validate(rows)
    if before != source_hashes():
        raise ValueError("Fontes mudaram durante a preparação; corpus não exportado.")
    OUTPUT.mkdir(parents=True, exist_ok=True)
    directory = Path(tempfile.mkdtemp(prefix="dataset-", dir=OUTPUT))
    manifest = {"status": "revisao_humana_pendente", "training_allowed": False,
                "synthetic": True, "count": len(rows), "source_hashes": before,
                "format": "messages-tools-conversacional", "files": {}, "splits": {},
                "origin": "Cenários fictícios escritos no projeto; resultados calculados pelas ferramentas Go.",
                "limitations": "Corpus inicial pequeno; não aprovado como orientação financeira ou dataset de produção."}
    for split, records in splits.items():
        payload = encode_rows([{"messages": row["messages"], "tools": row["tools"]} for row in records])
        path = directory / (split + ".jsonl")
        path.write_bytes(payload)
        path.chmod(0o600)
        manifest["files"][path.name] = digest(payload)
        manifest["splits"][split] = {"count": len(records), "ids": [row["id"] for row in records],
                                     "families": sorted({row["family"] for row in records})}
    audit = encode_rows(rows)
    (directory / "audit.jsonl").write_bytes(audit)
    (directory / "audit.jsonl").chmod(0o600)
    manifest["files"]["audit.jsonl"] = digest(audit)
    target = directory / "manifest.json"
    target.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    target.chmod(0o600)
    print(json.dumps({"directory": str(directory.relative_to(ROOT)), "count": len(rows),
                      "splits": {key: len(value) for key, value in splits.items()},
                      "training_allowed": False}, ensure_ascii=False))


if __name__ == "__main__":
    os.umask(0o077)
    main()
