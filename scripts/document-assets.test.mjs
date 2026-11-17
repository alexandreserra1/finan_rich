import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");

test("recursos de OCR correspondem aos hashes aprovados", () => {
  const base = join(root, "internal/web/static/vendor/ocr");
  const resources = JSON.parse(readFileSync(join(base, "resources.json"), "utf8"));
  for (const [name, expected] of Object.entries(resources)) {
    const bytes = readFileSync(join(base, name));
    assert.equal(bytes.length, expected.bytes, name);
    assert.equal(createHash("sha256").update(bytes).digest("hex"), expected.sha256, name);
  }
  assert.equal(resources["por.traineddata"].sha256, "c4932b937207a9514b7514d518b931a99938c02a28a5a5a553f8599ed58b7deb");
});

test("ferramentas de recursos usam apenas licenças permitidas", () => {
  const lock = JSON.parse(readFileSync(join(root, "tools/import-assets/package-lock.json"), "utf8"));
  for (const [name, entry] of Object.entries(lock.packages)) if (name) assert.equal(entry.license, "Apache-2.0", name);
});
