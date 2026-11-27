import hashlib
import json
import os
import re
from pathlib import Path
import subprocess
import tempfile
import unicodedata
from decimal import Decimal, InvalidOperation

from evaluate import ROOT, OUTPUT, source_hashes


def digest(data):
    return hashlib.sha256(data).hexdigest()


def plain(text):
    return "".join(char for char in unicodedata.normalize("NFKD", text.casefold())
                   if not unicodedata.combining(char))


def normalized_content(text, names=()):
    normalized = plain(text)
    for name in sorted(names, key=lambda value: (-len(value), plain(value))):
        normalized = re.sub(r"(?<!\w)" + re.escape(plain(name)) + r"(?!\w)", " nome ", normalized)
    if names:
        normalized = re.sub(r"\bnome\b(?:\s+nome\b)+", "nome", normalized)
    normalized = re.sub(r"\b(?:janeiro|fevereiro|marco|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)\b", "mes", normalized)
    normalized = re.sub(r"\d+(?:[.,/-]\d+)*", "numero", normalized)
    return " ".join(re.sub(r"[^\w\s]", " ", normalized).split())


def non_substantive(text):
    return bool(re.search(r"\b(?:nao (?:posso|tenho|vou|consigo|existe|ha)|preciso (?:de|que)|"
                          r"qual e o saldo|lucro certo|sem risco|sinal de golpe|"
                          r"qual (?:mes|prazo)|vamos continuar)\b", plain(text))
                or acknowledges_failure(text)
                or re.fullmatch(r"\s*(?:certo|entendi|ok|sim|nao|obrigad[oa])[.!\s]*", plain(text)))


def duplication_names(row):
    names = row.get("dedup_names", [])
    if not isinstance(names, list) or any(not isinstance(name, str) or not name.strip() for name in names):
        raise ValueError("dedup_names precisa ser uma lista de nomes: " + row["id"])
    names = set(names)
    records = [record for collection in ("transactions", "goals", "debts", "recurring")
               for record in (row.get("ledger") or {}).get(collection, [])]
    records += [item.get("record") for item in row.get("proposals") or [] if isinstance(item, dict)]
    for record in records:
        if isinstance(record, dict):
            names.update(value for key, value in record.items()
                         if key in {"name", "description", "creditor"} and isinstance(value, str) and value.strip())
    for message in row["messages"]:
        for call in message.get("tool_calls", []):
            arguments = call["function"].get("arguments", {})
            if isinstance(arguments, dict):
                names.update(value for key, value in arguments.items()
                             if key in {"nome", "descricao", "credor", "meta", "divida", "texto"}
                             and isinstance(value, str) and value.strip())
    return names


def simple_refusal_or_continuation(text):
    normalized = plain(text).strip().rstrip(".!?")
    if re.search(r"\d|r\$|[%$;:.!?]|\b(?:reais|centavos|saldo|atrasos|janeiro|fevereiro|marco|abril|maio|junho|julho|agosto|setembro|outubro|novembro|dezembro)\b", normalized):
        return normalized == "qual e o saldo"
    return bool(re.fullmatch(
        r"(?:certo(?:, vamos continuar)?|vamos continuar|entendi|ok|sim|nao|obrigad[oa]|"
        r"nao (?:posso|consigo|vou) (?:revelar|consultar|informar|garantir|prometer|acessar)(?: [a-z]+)*|"
        r"nao tenho (?:acesso|dados|informacoes)(?: [a-z]+)*|qual (?:mes|prazo))", normalized))


def textual_assistant_targets(row):
    failed_calls = {message["tool_call_id"] for message in row["messages"]
                    if message["role"] == "tool" and (message.get("error") is True
                    or executor_failure(message.get("content", "")))}
    return [message for message in row["messages"] if message["role"] == "assistant"
            and message.get("content") and not any(call["id"] in failed_calls
            for call in message.get("tool_calls", []))]


def duplicate_findings(rows):
    findings, seen = [], {}
    for row in rows:
        names = duplication_names(row)
        question = next(message["content"] for message in reversed(row["messages"]) if message["role"] == "user")
        question = question.rsplit("</dados_financeiros>", 1)[-1]
        texts = [("pergunta", question)] + [("resposta", message["content"])
                                           for message in textual_assistant_targets(row)]
        for field, text in texts:
            exact = " ".join(re.sub(r"[^\w\s]", " ", plain(text)).split())
            if not exact or (len(exact.split()) <= 12 and simple_refusal_or_continuation(text)):
                continue
            masked = normalized_content(text, names)
            for mode, value in (("exata", exact), ("molde", masked)):
                key = (field, mode, value)
                previous = seen.setdefault(key, row)
                if previous["split"] != row["split"]:
                    findings.append({"id": row["id"], "issue": "duplicata_" + field,
                                     "other_id": previous["id"], "mode": mode})
                    break
    return findings


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


def argument_cents(value):
    if not isinstance(value, (str, int)) or type(value) is bool:
        raise ValueError("Valor monetário precisa ser texto ou inteiro.")
    text = str(value).strip().removeprefix("R$").replace(" ", "").removesuffix("%")
    if "," in text or re.fullmatch(r"\d{1,3}(?:\.\d{3})+", text):
        text = text.replace(".", "").replace(",", ".")
    if not re.fullmatch(r"\d+(?:\.\d{1,2})?", text):
        raise ValueError("Contrato monetário sem correspondência exata em centavos.")
    return int(Decimal(text) * 100)


def proposal_record_matches(proposal, name, arguments, state, today):
    record, kind, action = proposal["record"], proposal["type"], proposal["action"]
    if not isinstance(record.get("id"), str) or not record["id"]:
        return False
    for key, value in record.items():
        if key.endswith("Cents") or key in {"monthlyRateBp", "day", "dueDay", "updatedAt"}:
            if type(value) is not int or not 0 <= value <= 9223372036854775807:
                return False
    if record.get("deleted", False) is not (action == "apagar"):
        return False
    expected, mutable = {}, set()
    previous = state.get(kind, {}).get(record["id"])
    if action in {"alterar", "apagar"}:
        if previous is None or previous.get("deleted"):
            return False
        if action == "alterar" or "before" in proposal:
            if proposal.get("before") != previous:
                return False
    elif previous is not None or proposal.get("before") is not None:
        return False
    if name in {"propor_lancamento", "criar_conta_fixa"}:
        expected = {"description": arguments["descricao"], "amountCents": argument_cents(arguments["valor"]),
                    "kind": arguments.get("tipo", "saida")}
        if kind == "transaction":
            expected["date"] = arguments.get("data") or today
        else:
            expected.update(day=int(arguments["dia"]), start=today[:7])
    elif name == "criar_meta":
        expected = {"name": arguments["nome"], "targetCents": argument_cents(arguments["valor"]),
                    "savedCents": argument_cents(arguments.get("ja_guardado") or "0"),
                    "deadline": arguments.get("prazo", ""), "startDate": today}
    elif name == "criar_divida":
        expected = {"name": arguments["nome"], "creditor": arguments.get("credor", ""),
                    "balanceCents": argument_cents(arguments["saldo"]),
                    "monthlyRateBp": argument_cents(arguments["juros_mes"]),
                    "minPaymentCents": argument_cents(arguments["parcela_minima"]),
                    "dueDay": int(arguments.get("vencimento") or 0)}
    elif name == "alterar_lancamento":
        for argument, field in {"descricao": "description", "data": "date", "valor": "amountCents"}.items():
            if arguments.get(argument) not in (None, ""):
                expected[field] = argument_cents(arguments[argument]) if argument == "valor" else arguments[argument]
        mutable = set(expected)
    elif name == "apagar_lancamento":
        expected["deleted"] = True
        mutable = {"deleted"}
    elif name in {"registrar_aporte", "registrar_pagamento"}:
        source_kind, argument = ("goal", "meta") if name == "registrar_aporte" else ("debt", "divida")
        source = next((item for item in state.get(source_kind, {}).values()
                       if not item.get("deleted") and plain(item.get("name", "")) == plain(arguments[argument])), None)
        if source is None:
            return False
        if kind == "transaction":
            expected = {"amountCents": argument_cents(arguments["valor"]), "date": arguments.get("data") or today,
                        "kind": "saida", "description": ("Aporte: " if source_kind == "goal" else "Pagamento: ") + source["name"]}
            if source_kind == "goal":
                expected["categoryId"] = source.get("categoryId", "")
            else:
                expected["categoryId"] = next(item["id"] for item in state.get("category", {}).values()
                                               if item.get("name") == "Pagamento de dívidas" and not item.get("deleted"))
        else:
            if record["id"] != source["id"]:
                return False
            field = "savedCents" if kind == "goal" else "balanceCents"
            mutable = {field}
            cents = record.get(field)
            if type(cents) is not int:
                return False
            currency = "R$ " + format(cents // 100, ",").replace(",", ".") + f",{cents % 100:02d}"
            if currency not in proposal["label"]:
                return False
    if kind in {"transaction", "recurring"}:
        category = state.get("category", {}).get(record.get("categoryId"))
        if category is None or category.get("deleted"):
            return False
        requested = arguments.get("categoria")
        matches = [item["id"] for item in state["category"].values()
                   if requested and not item.get("deleted") and plain(item.get("name", "")) == plain(requested)]
        if matches:
            expected["categoryId"] = matches[0]
            if name == "alterar_lancamento":
                mutable.add("categoryId")
    if any(record.get(field, "" if isinstance(value, str) else 0) != value for field, value in expected.items()):
        return False
    if previous is not None:
        for field in set(previous) | set(record):
            if field not in mutable | {"updatedAt"} and previous.get(field) != record.get(field):
                return False
    return True


def proposal_findings(row):
    onboarding_reminder = " Se a mensagem da pessoa tiver outro item com valor que ainda não virou cartão, chame a ferramenta para ele agora."
    contracts = {"propor_lancamento": [("criar", "transaction")],
                 "alterar_lancamento": [("alterar", "transaction")],
                 "apagar_lancamento": [("apagar", "transaction")],
                 "criar_meta": [("criar", "goal")], "criar_divida": [("criar", "debt")],
                 "criar_conta_fixa": [("criar", "recurring")],
                 "registrar_aporte": [("criar", "transaction"), ("alterar", "goal")],
                 "registrar_pagamento": [("alterar", "debt")]}
    proposals = row.get("proposals")
    if proposals is None:
        proposals = []
    if not isinstance(proposals, list) or any(
            not isinstance(item, dict) or not isinstance(item.get("record"), dict)
            or item.get("applied", False) is not False
            or not isinstance(item.get("label"), str) or not item["label"].strip()
            for item in proposals):
        return ["propostas_divergentes"]
    pending, consumed, last_user_consumed = {}, 0, 0
    state = {kind: {item["id"]: item for item in (row.get("ledger") or {}).get(collection, [])}
             for kind, collection in {"transaction": "transactions", "goal": "goals", "debt": "debts",
                                      "recurring": "recurring", "category": "categories"}.items()}
    for message in row["messages"]:
        if message["role"] == "user":
            last_user_consumed = consumed
        for call in message.get("tool_calls", []):
            pending[call["id"]] = call["function"]
        if message["role"] != "tool":
            continue
        function = pending.pop(message.get("tool_call_id"), None)
        if function is None or function["name"] != message.get("name"):
            return ["propostas_divergentes"]
        name, result = function["name"], message.get("content", "")
        result = result.removesuffix(onboarding_reminder)
        if name not in contracts or message.get("error") is True or executor_failure(result):
            continue
        start = consumed
        while consumed < len(proposals) and proposals[consumed]["label"] in result:
            consumed += 1
        group = proposals[start:consumed]
        kinds = [(item.get("action"), item.get("type")) for item in group]
        valid = kinds in [[kind] for kind in contracts[name]]
        if name == "registrar_pagamento":
            valid = valid or kinds == [("alterar", "debt"), ("criar", "transaction")]
        expected = "Pronto na tela para o usuário confirmar (" + "; ".join(item["label"] for item in group) + "). Ainda NÃO foi feito."
        if not valid or result != expected:
            return ["propostas_divergentes"]
        for proposal in group:
            try:
                matches = proposal_record_matches(proposal, name, function["arguments"], state, row.get("today", ""))
            except (KeyError, TypeError, ValueError, InvalidOperation, StopIteration):
                matches = False
            if not matches:
                return ["propostas_divergentes"]
            state[proposal["type"]][proposal["record"]["id"]] = proposal["record"]
        if name in {"alterar_lancamento", "apagar_lancamento"} and group[0]["record"].get("id") != function["arguments"].get("id"):
            return ["propostas_divergentes"]
    if consumed != len(proposals):
        return ["propostas_divergentes"]
    count = consumed - last_user_consumed
    if count:
        answer = plain(row["messages"][-1].get("content", ""))
        quantities = {"um": 1, "uma": 1, "o": 1, "1": 1, "dois": 2, "duas": 2, "2": 2,
                      "ambos os": 2, "tres": 3, "3": 3, "quatro": 4, "4": 4}
        claimed = [quantities[match] for match in re.findall(
            r"\b(ambos os|um|uma|o|1|dois|duas|2|tres|3|quatro|4) cart(?:ao|oes)\b", answer)]
        if (count > 1 and count not in claimed) or (count == 1 and any(value != count for value in claimed)):
            return ["quantidade_cartoes"]
    return []


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
    pending, seen, search_ids = {}, set(), set()
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
            search_ids.clear()
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
                if name in {"alterar_lancamento", "apagar_lancamento"} and (
                        not isinstance(arguments.get("id"), str) or arguments["id"] not in search_ids):
                    raise ValueError("ID sem origem em busca anterior do turno: " + identifier)
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
            elif message.get("name") == "buscar_lancamentos":
                search_ids.update(re.findall(r"(?m)^\s*-\s*\[([A-Za-z0-9_][A-Za-z0-9_.:-]*)\]", content))
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
        if proposal_findings(row):
            raise ValueError("Propostas ou quantidade de cartões divergentes: " + identifier)
        current = next(message["content"] for message in reversed(messages) if message["role"] == "user")
        context, separator, question = current.rpartition("</dados_financeiros>")
        if not separator:
            context, question = "", current
        normalized = " ".join(re.sub(r"[^\w\s]", " ", plain(question)).split())
        identity = (split, normalized, digest(context.encode()))
        if identity in questions:
            raise ValueError("Pergunta repetida, inclusive após normalização: " + identifier)
        questions.add(identity)
        splits[split].append(row)
    duplicates = duplicate_findings(rows)
    if duplicates:
        finding = duplicates[0]
        raise ValueError("Duplicata entre conjuntos (" + finding["issue"] + "): "
                         + finding["other_id"] + " / " + finding["id"])
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
