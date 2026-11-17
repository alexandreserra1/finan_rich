import { isPDF, pdfText } from "./pdftext.js";
import { createOCR } from "./ocr.js";

export const MAX_FILE_BYTES = 8 * 1024 * 1024;

export function imageType(bytes) {
  if (bytes[0] === 0x89 && String.fromCharCode(...bytes.slice(1, 4)) === "PNG") return "image/png";
  if (bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff) return "image/jpeg";
  if (String.fromCharCode(...bytes.slice(0, 4)) === "RIFF" && String.fromCharCode(...bytes.slice(8, 12)) === "WEBP") return "image/webp";
  return "";
}

export function imageDimensions(bytes, type) {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  let width = 0;
  let height = 0;
  if (type === "image/png" && bytes.length >= 24) {
    width = view.getUint32(16); height = view.getUint32(20);
  } else if (type === "image/jpeg") {
    let offset = 2;
    while (offset + 9 < bytes.length) {
      if (bytes[offset] !== 0xff) break;
      const marker = bytes[offset + 1];
      const length = view.getUint16(offset + 2);
      if (length < 2 || offset + length + 2 > bytes.length) break;
      if ([0xc0, 0xc1, 0xc2].includes(marker)) { height = view.getUint16(offset + 5); width = view.getUint16(offset + 7); break; }
      offset += length + 2;
    }
  } else if (type === "image/webp" && bytes.length >= 30) {
    const chunk = String.fromCharCode(...bytes.slice(12, 16));
    if (chunk === "VP8X") {
      if (bytes[20] & 2) throw new Error("Envie uma imagem estática, não uma animação.");
      width = 1 + (view.getUint32(24, true) & 0xffffff); height = 1 + (bytes[27] | (bytes[28] << 8) | (bytes[29] << 16));
    } else if (chunk === "VP8 ") {
      width = view.getUint16(26, true) & 0x3fff; height = view.getUint16(28, true) & 0x3fff;
    } else if (chunk === "VP8L") {
      const bits = view.getUint32(21, true); width = 1 + (bits & 0x3fff); height = 1 + ((bits >>> 14) & 0x3fff);
    }
  }
  if (!width || !height) throw new Error("Imagem danificada ou formato não suportado. Use PNG, JPEG ou WebP.");
  if (width * height > 20_000_000 || width > 10000 || height > 10000) throw new Error("Imagem muito grande. Recorte o documento antes de enviar.");
  return { width, height };
}

export async function imageForOCR(blob, signal) {
  signal?.throwIfAborted();
  const url = await new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result);
    reader.onerror = () => reject(new Error("Não consegui abrir esta imagem."));
    reader.readAsDataURL(blob);
  });
  const image = new Image();
  try {
    image.src = url;
    await image.decode();
    signal?.throwIfAborted();
    if (!image.naturalWidth || image.naturalWidth * image.naturalHeight > 20_000_000) throw new Error("Imagem muito grande. Recorte o documento antes de enviar.");
    const scale = Math.min(1, 1800 / Math.max(image.naturalWidth, image.naturalHeight));
    const canvas = document.createElement("canvas");
    canvas.width = Math.round(image.naturalWidth * scale);
    canvas.height = Math.round(image.naturalHeight * scale);
    const context = canvas.getContext("2d");
    context.fillStyle = "white";
    context.fillRect(0, 0, canvas.width, canvas.height);
    context.drawImage(image, 0, 0, canvas.width, canvas.height);
    const result = await new Promise((resolve) => canvas.toBlob(resolve, "image/png"));
    canvas.width = canvas.height = 0;
    if (!result) throw new Error("Não consegui preparar esta imagem.");
    return result;
  } finally { image.src = ""; }
}

export async function readDocument(file, { signal, onProgress = () => {}, askPassword = () => null } = {}) {
  if (!file.size || file.size > MAX_FILE_BYTES) throw new Error("Escolha um documento de até 8 MB.");
  const bytes = new Uint8Array(await file.arrayBuffer());
  signal?.throwIfAborted();
  let reader;
  let usedOCR = false;
  let mixed = false;
  let ocrPages = 0;
  const recognize = async (blob) => {
    if (++ocrPages > 6) throw new Error("Este documento precisa de leitura por imagem em mais de 6 páginas. Divida-o em arquivos menores.");
    usedOCR = true;
    reader ||= createOCR({ signal, onProgress });
    return (await reader.recognize(blob)).text;
  };
  const timeout = setTimeout(() => reader?.close(), 120000);
  try {
    if (isPDF(bytes)) {
      onProgress("Lendo as páginas do PDF neste aparelho…");
      const text = await pdfText(bytes, askPassword, { signal, recognize, onProgress, onMixedPage: () => { mixed = true; } });
      return { text, source: usedOCR ? "PDF com reconhecimento de imagem" : "PDF com texto", requireReview: usedOCR || mixed, mixed };
    }
    const type = imageType(bytes);
    if (type) {
      imageDimensions(bytes, type);
      onProgress("Preparando a imagem neste aparelho…");
      return { text: await recognize(await imageForOCR(new Blob([bytes], { type }), signal)), source: "Imagem com reconhecimento de texto", requireReview: true };
    }
    let binary = "";
    for (let offset = 0; offset < bytes.length; offset += 0x8000) binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
    return { base64: btoa(binary), source: "Arquivo financeiro", requireReview: false };
  } finally { clearTimeout(timeout); reader?.close(); }
}
