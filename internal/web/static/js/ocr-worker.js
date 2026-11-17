const BASE = "/static/vendor/ocr/";
let engine;

self.onmessage = async ({ data }) => {
  let api;
  try {
    if (!(data.image instanceof ArrayBuffer) || data.image.byteLength > 12 * 1024 * 1024) throw new Error("Imagem fora do limite de leitura.");
    if (!engine) {
      self.postMessage({ progress: "Preparando a leitura local…" });
      importScripts(BASE + "tesseract-core-lstm.js");
      engine = await self.TesseractCore({ locateFile: (name) => BASE + name, print() {}, printErr() {} });
      const response = await fetch(BASE + "por.traineddata");
      if (!response.ok) throw new Error("Não consegui carregar os recursos de leitura. Conecte-se uma vez e tente novamente.");
      engine.FS.writeFile("/por.traineddata", new Uint8Array(await response.arrayBuffer()));
    }
    self.postMessage({ progress: "Reconhecendo o texto neste aparelho…" });
    api = new engine.TessBaseAPI();
    if (api.Init(null, "por", 1) !== 0) throw new Error("Não consegui iniciar a leitura em português.");
    engine.FS.writeFile("/input", new Uint8Array(data.image));
    if (api.SetImageFile(1, 0) === 1) throw new Error("Não consegui abrir esta imagem.");
    api.SetVariable("user_defined_dpi", "200");
    if (api.Recognize(null) !== 0) throw new Error("Não consegui reconhecer o documento.");
    const text = api.GetUTF8Text();
    const confidence = api.MeanTextConf();
    if (!text.trim() || confidence < 35) throw new Error("A imagem está pouco legível. Recorte o documento ou envie uma foto mais nítida.");
    if (text.length > 1024 * 1024) throw new Error("Texto maior que o limite de leitura.");
    self.postMessage({ text, confidence });
  } catch (error) {
    self.postMessage({ error: error.message || "Não consegui ler esta imagem. Tente um extrato OFX, CSV ou uma foto mais nítida." });
  } finally {
    if (api) { api.End(); engine.destroy(api); }
    if (engine) { try { engine.FS.unlink("/input"); } catch {} }
  }
};
