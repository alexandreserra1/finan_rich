import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

export function selectIOSSimulator(snapshot) {
  if (!snapshot?.devices || typeof snapshot.devices !== "object" || Array.isArray(snapshot.devices)) throw new Error("A resposta do simulador iOS é inválida.");
  const candidates = [];
  for (const [runtime, devices] of Object.entries(snapshot.devices)) {
    if (!runtime.includes(".iOS-") || !Array.isArray(devices)) continue;
    for (const device of devices) {
      if (device?.isAvailable === true && typeof device.name === "string" && device.name.startsWith("iPhone ") && typeof device.udid === "string" && /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i.test(device.udid)) candidates.push(device);
    }
  }
  candidates.sort((first, second) => Number(second.state === "Booted") - Number(first.state === "Booted") || first.name.localeCompare(second.name));
  if (!candidates.length) throw new Error("Nenhum simulador iPhone disponível no runner. Confira os runtimes do Xcode.");
  return candidates[0].udid;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    console.log(selectIOSSimulator(JSON.parse(readFileSync(0, "utf8"))));
  } catch (error) {
    console.error(error instanceof SyntaxError ? "A resposta do simulador não é JSON válido." : error.message);
    process.exitCode = 1;
  }
}
