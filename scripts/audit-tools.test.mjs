import assert from "node:assert/strict";
import { test } from "node:test";
import { blocking } from "./audit-tools.mjs";

const advisory = (id, severity = "high") => ({ source: 1, title: "aviso", url: `https://github.com/advisories/${id}`, severity });
const report = (...vias) => ({ vulnerabilities: { pacote: { severity: "high", via: vias } } });
const allowed = [{ id: "GHSA-liberado", until: "2026-12-17" }];

test("aviso liberado dentro do prazo não barra, mas aparece", () => {
  assert.deepEqual(blocking(report(advisory("GHSA-liberado")), "2026-11-17", allowed), { block: [], excused: ["pacote: GHSA-liberado"] });
});

test("liberação vencida volta a barrar", () => {
  assert.deepEqual(blocking(report(advisory("GHSA-liberado")), "2026-12-18", allowed).block, ["pacote: GHSA-liberado"]);
});

test("aviso alto ou crítico sem liberação barra", () => {
  assert.deepEqual(blocking(report(advisory("GHSA-novo", "critical")), "2026-11-17", allowed).block, ["pacote: GHSA-novo"]);
});

test("aviso moderado e dependência só por outro pacote não barram", () => {
  assert.deepEqual(blocking(report(advisory("GHSA-leve", "moderate"), "picomatch"), "2026-11-17", allowed).block, []);
});
