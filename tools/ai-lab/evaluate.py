import hashlib
import fcntl
import json
import os
from pathlib import Path
import platform
import re
import signal
import subprocess
import sys
import time
import tempfile
from datetime import datetime, timezone
from urllib.request import Request, urlopen


ROOT = Path(__file__).resolve().parents[2]
OUTPUT = ROOT / ".artifacts" / "ai-lab"
API = "http://127.0.0.1:11434/api/"
MODELS = ("qwen2.5:7b-instruct", "qwen3.5:4b")
CASES = r"economizar|mínimo|uber|R\$ 500|R\$ 400|mudou|20 dias"


def summarize(log, returncode):
    events = [json.loads(line) for line in log.splitlines() if line.strip()]
    text = "".join(item.get("Output", "") for item in events)
    score = re.search(r"NOTA: (\d+) de (\d+) critérios", text)
    earned, total = map(int, score.groups()) if score else (0, 0)
    completed = any(item.get("Test") == "TestEval" and item.get("Action") == "pass"
                    for item in events)
    failed_cases = len(re.findall(r"\bFALHA\b", text))
    return {"passed": returncode == 0 and completed and total > 0
            and earned == total and failed_cases == 0,
            "earned": earned, "total": total, "failed_cases": failed_cases,
            "go_exit_code": returncode, "test_completed": completed}


def local_environment(environment, model):
    clean = {key: value for key, value in environment.items()
             if not key.startswith("FINAN_EVAL_")}
    clean.update(FINAN_EVAL_PRESET="ollama", FINAN_EVAL_MODEL=model,
                 FINAN_EVAL_KEY="laboratorio-local-sem-segredo",
                 FINAN_EVAL_LOCAL="1", FINAN_EVAL_ONLY=CASES)
    return clean


def request(endpoint, payload=None, timeout=10):
    body = json.dumps(payload).encode() if payload is not None else None
    req = Request(API + endpoint, data=body,
                  headers={"Content-Type": "application/json"})
    with urlopen(req, timeout=timeout) as response:
        return json.load(response)


def metadata(model):
    installed = request("tags")["models"]
    entry = next(item for item in installed if item["name"] == model)
    shown = request("show", {"model": model})
    if "apache" not in shown.get("license", "").lower():
        raise ValueError("Licença Apache não encontrada: " + model)
    return {"name": model, "digest": entry["digest"], "size_bytes": entry["size"],
            "details": shown.get("details"), "parameters": shown.get("parameters"),
            "capabilities": shown.get("capabilities"),
            "template_sha256": hashlib.sha256(shown.get("template", "").encode()).hexdigest(),
            "license_sha256": hashlib.sha256(shown["license"].encode()).hexdigest()}


def smoke(model):
    payload = {"model": model, "stream": True, "think": False, "keep_alive": "2m",
               "options": {"num_ctx": 8192, "num_predict": 160, "temperature": 0.7,
                           "top_p": 0.8, "top_k": 20, "seed": 42},
               "messages": [{"role": "user", "content":
                             "Responda em português, em até duas frases: qual a diferença "
                             "entre orçamento e saldo? Não recomende investimentos."}]}
    req = Request(API + "chat", data=json.dumps(payload).encode(),
                  headers={"Content-Type": "application/json"})
    started = time.monotonic()
    first, answer, final = None, "", {}
    with urlopen(req, timeout=90) as response:
        for index in range(1024):
            item = read_event(response)
            if item is None:
                break
            if time.monotonic() - started > 90:
                raise TimeoutError("Smoke test excedeu 90 segundos.")
            if item.get("error"):
                raise RuntimeError(item["error"])
            content = item.get("message", {}).get("content", "")
            if content and first is None:
                first = time.monotonic() - started
            answer += content
            if item.get("done"):
                final = item
                break
    if first is None or not final:
        raise ValueError("Streaming vazio ou incompleto.")
    loaded = request("ps").get("models", [])
    return {"first_text_seconds": first, "wall_seconds": time.monotonic() - started,
            "answer": answer, "done_reason": final.get("done_reason"),
            "output_tokens": final.get("eval_count"),
            "generation_nanoseconds": final.get("eval_duration"),
            "resident_size_bytes_observed": next((item.get("size") for item in loaded
                                                  if item["name"] == model), None),
            "peak_memory_measured": False}


def read_event(response):
    line = response.readline(65537)
    if len(line) > 65536:
        raise ValueError("Linha de streaming excedeu 64 KiB.")
    return json.loads(line) if line else None


def bounded_smoke(model):
    result = subprocess.run([sys.executable, __file__, "--smoke", model], cwd=ROOT,
                            capture_output=True, text=True, timeout=95, check=True)
    return json.loads(result.stdout)


def unload(model):
    request("generate", {"model": model, "keep_alive": 0}, timeout=30)
    deadline = time.monotonic() + 5
    while True:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise RuntimeError("Modelo permaneceu residente após descarregar: " + model)
        loaded = request("ps", timeout=min(1, remaining)).get("models", [])
        if not any(item["name"] == model for item in loaded):
            return
        time.sleep(0.05)


def source_hashes():
    files = list((ROOT / "internal").rglob("*.go"))
    files += list(Path(__file__).parent.glob("*.py"))
    files += list((Path(__file__).parent / "corpusgen").glob("*.go"))
    files += list((Path(__file__).parent / "reservedeval").glob("*.go"))
    files += [ROOT / "go.mod", ROOT / "go.sum", Path(__file__).parent / "models.json",
              Path(__file__).parent / "runtime-model.json", Path(__file__).parent / "requirements-mlx.lock"]
    return {str(path.relative_to(ROOT)): hashlib.sha256(path.read_bytes()).hexdigest()
            for path in sorted(files)}


def evaluate(model, directory):
    started = time.monotonic()
    child = subprocess.Popen(["go", "test", "./internal/advisor", "-run", "^TestEval$",
                              "-count=1", "-json", "-timeout=5m"], cwd=ROOT,
                             env=local_environment(os.environ, model),
                             stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                             text=True, start_new_session=True)
    try:
        stdout, stderr = child.communicate(timeout=330)
    except BaseException:
        os.killpg(child.pid, signal.SIGKILL)
        child.communicate()
        raise
    target = directory / (model.replace(":", "-") + ".jsonl")
    target.write_text(stdout, encoding="utf-8")
    target.chmod(0o600)
    result = summarize(stdout, child.returncode)
    result.update(wall_seconds=time.monotonic() - started,
                  log=str(target.relative_to(ROOT)), stderr=stderr[-2000:])
    return result


def save(report, directory):
    target = directory / "report.json"
    temporary = directory / "report.tmp"
    temporary.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    temporary.chmod(0o600)
    temporary.replace(target)


def run_model(model, expected, directory):
    result = {"model": model}
    try:
        unload(model)
        result["metadata"] = metadata(model)
        if result["metadata"]["digest"] != expected:
            raise ValueError("Digest diferente do manifesto revisado: " + model)
        result["smoke"] = bounded_smoke(model)
        result["evaluation"] = evaluate(model, directory)
        if metadata(model)["digest"] != expected:
            raise ValueError("Modelo mudou durante a avaliação.")
    except Exception as error:
        result["error"] = str(error)
    finally:
        try:
            unload(model)
        except Exception as error:
            result["cleanup_error"] = str(error)
    return result


def main():
    OUTPUT.mkdir(parents=True, exist_ok=True)
    OUTPUT.chmod(0o700)
    lock = (OUTPUT / "laboratory.lock").open("a")
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        sys.exit("Outro laboratório já está usando estes modelos; aguarde sua conclusão.")
    directory = Path(tempfile.mkdtemp(prefix="run-", dir=OUTPUT))
    manifest = json.loads((Path(__file__).parent / "models.json").read_text())
    report = {"started_at": datetime.now(timezone.utc).isoformat(), "status": "running",
              "platform": platform.platform(), "runtime": request("version"),
              "source_hashes": source_hashes(), "case_filter": CASES, "results": [],
              "limits": {"go_test_minutes_per_model": 5, "smoke_seconds": 90},
              "scope": "Dados fictícios; critérios automáticos existentes, sem aprovação financeira humana.",
              "manifest": manifest, "run_directory": str(directory.relative_to(ROOT))}
    save(report, directory)
    print("Evidências: " + str(directory.relative_to(ROOT)), flush=True)
    for model in MODELS:
        result = run_model(model, manifest[model], directory)
        report["results"].append(result)
        save(report, directory)
        print(json.dumps(result, ensure_ascii=False), flush=True)
        if result.get("cleanup_error"):
            break
    unchanged = report["source_hashes"] == source_hashes()
    report["sources_unchanged"] = unchanged
    passed = unchanged and all(not result.get("error") and not result.get("cleanup_error") and
                              result.get("evaluation", {}).get("passed") for result in report["results"])
    report["status"] = "passed_automatic_checks" if passed else "needs_review"
    report["finished_at"] = datetime.now(timezone.utc).isoformat()
    save(report, directory)
    return 0 if passed else 1


if __name__ == "__main__":
    if len(sys.argv) == 3 and sys.argv[1] == "--smoke" and sys.argv[2] in MODELS:
        print(json.dumps(smoke(sys.argv[2]), ensure_ascii=False))
    elif len(sys.argv) == 1:
        sys.exit(main())
    else:
        sys.exit("Use evaluate.py sem argumentos para comparar os modelos locais.")
