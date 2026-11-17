import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { selectIOSSimulator } from "./select-ios-simulator.mjs";

const first = "12345678-1234-1234-1234-123456789012";
const second = "87654321-4321-4321-4321-210987654321";
const device = (name, udid = first, state = "Shutdown") => ({ name, udid, state, isAvailable: true });

test("seleciona apenas iPhone disponível de runtime iOS", () => {
  assert.equal(selectIOSSimulator({ devices: {
    "com.apple.CoreSimulator.SimRuntime.tvOS-26": [device("iPhone falso")],
    "com.apple.CoreSimulator.SimRuntime.iOS-26": [
      device("iPad Pro"), { ...device("iPhone indisponível"), isAvailable: false },
      device("iPhone inválido", "valor; comando"), device("iPhone de teste", second),
    ],
  } }), second);
});

test("prefere um iPhone já aberto sem alterar a lista recebida", () => {
  const devices = [device("iPhone A"), device("iPhone B", second, "Booted")];
  assert.equal(selectIOSSimulator({ devices: { "runtime.iOS-26": devices } }), second);
  assert.equal(devices[0].udid, first);
});

test("sem simulador ou com resposta inválida reprova em vez de pular", () => {
  for (const value of [null, {}, { devices: null }, { devices: [] }, { devices: {} }]) {
    assert.throws(() => selectIOSSimulator(value), /simulador|resposta/i);
  }
});

test("a branch de validação não faz deploy nem desliga produção", () => {
  const config = JSON.parse(readFileSync(new URL("../vercel.json", import.meta.url), "utf8"));
  assert.deepEqual(config.git.deploymentEnabled, { "codex/validacao-ios-sem-xcode": false });
});
