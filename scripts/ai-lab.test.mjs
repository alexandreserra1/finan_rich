import assert from "node:assert/strict";
import { test } from "node:test";
import { spawnSync } from "node:child_process";
import { classify, root } from "./engineering.mjs";

test("laboratório aciona o gate e testa validadores sem inferência", () => {
  assert.deepEqual(classify(["tools/ai-lab/evaluate.py"]), ["tooling"]);
  const result = spawnSync("python3", ["-m", "unittest", "discover", "-s", "tools/ai-lab", "-v"], {
    cwd: root, encoding: "utf8", timeout: 10000,
    env: { ...process.env, PYTHONDONTWRITEBYTECODE: "1" },
  });
  assert.equal(result.status, 0, result.stderr || result.error?.message || result.stdout);
});
