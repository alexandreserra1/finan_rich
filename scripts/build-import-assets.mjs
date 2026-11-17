import { copyFileSync, mkdirSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { createHash } from "node:crypto";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const source = join(root, "tools/import-assets/node_modules");
const destination = join(root, "internal/web/static/vendor/ocr");
mkdirSync(destination, { recursive: true });
for (const obsolete of ["por.traineddata.gz"]) rmSync(join(destination, obsolete), { force: true });
const files = {
  "tesseract-core-lstm.js": "tesseract.js-core/tesseract-core-lstm.js",
  "tesseract-core-lstm.wasm": "tesseract.js-core/tesseract-core-lstm.wasm",
  "LICENSE": "tesseract.js-core/LICENSE",
};
const manifest = {};
for (const [name, path] of Object.entries(files)) {
  copyFileSync(join(source, path), join(destination, name));
  if (name === "LICENSE") writeFileSync(join(destination, name), readFileSync(join(destination, name), "utf8").trimEnd() + "\n");
  const bytes = readFileSync(join(destination, name));
  manifest[name] = { bytes: bytes.length, sha256: createHash("sha256").update(bytes).digest("hex") };
}
const model = await fetch("https://raw.githubusercontent.com/tesseract-ocr/tessdata_fast/4.1.0/por.traineddata", { signal: AbortSignal.timeout(30000) });
if (!model.ok) throw new Error("Não consegui baixar o modelo de português.");
const bytes = Buffer.from(await model.arrayBuffer());
const digest = createHash("sha256").update(bytes).digest("hex");
if (digest !== "c4932b937207a9514b7514d518b931a99938c02a28a5a5a553f8599ed58b7deb") throw new Error("O modelo de português não corresponde ao hash aprovado.");
writeFileSync(join(destination, "por.traineddata"), bytes);
manifest["por.traineddata"] = { bytes: bytes.length, sha256: digest };
writeFileSync(join(destination, "resources.json"), JSON.stringify(manifest, null, 2) + "\n");
console.log("Recursos locais de OCR copiados com hashes; nenhum documento foi processado.");
