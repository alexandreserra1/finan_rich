// Extrai o texto de um PDF neste aparelho com o pdf.js (o leitor de PDF do
// Firefox, que vai dentro do app). Devolve uma linha de texto por linha da
// página, na ordem de leitura, para o Go reconhecer as compras da fatura.
// O pdf.js (~1,8 MB) só é carregado quando a pessoa escolhe um PDF.

const BASE = "/static/vendor/pdfjs/";
let lib = null;

// Safari (até a 26) não percorre ReadableStream com "for await", que o pdf.js 6
// usa ao ler o texto: sem isto, a fatura em PDF dava "undefined is not a function".
if (!ReadableStream.prototype[Symbol.asyncIterator]) {
  ReadableStream.prototype[Symbol.asyncIterator] = async function* () {
    const reader = this.getReader();
    try {
      for (;;) {
        const { done, value } = await reader.read();
        if (done) return;
        yield value;
      }
    } finally {
      reader.releaseLock();
    }
  };
}

async function pdfjs() {
  if (!lib) {
    lib = await import(BASE + "pdf.min.js");
    lib.GlobalWorkerOptions.workerSrc = BASE + "pdf.worker.min.js";
  }
  return lib;
}

export const isPDF = (bytes) => bytes.length > 4 && bytes[0] === 0x25 && bytes[1] === 0x50 && bytes[2] === 0x44 && bytes[3] === 0x46; // %PDF

// pdfText lê o PDF. askPassword é chamada se ele tiver senha (muitos bancos
// usam parte do CPF); devolve a senha ou null para desistir.
export async function pdfText(bytes, askPassword) {
  const { getDocument } = await pdfjs();
  const task = getDocument({ data: bytes, isEvalSupported: false, useSystemFonts: false });
  task.onPassword = (update, reason) => {
    const password = askPassword(reason === 2 ? "Senha incorreta. Tente de novo:" : "Este PDF tem senha (muitos bancos usam os primeiros dígitos do CPF). Digite a senha:");
    if (password == null) task.destroy();
    else update(password);
  };
  const doc = await task.promise;
  const lines = [];
  for (let n = 1; n <= doc.numPages; n++) {
    const page = await doc.getPage(n);
    const content = await page.getTextContent();
    lines.push(...toLines(content.items));
    page.cleanup();
  }
  await task.destroy();
  return lines.join("\n");
}

// toLines junta os pedaços de texto que estão na mesma altura da página,
// da esquerda para a direita, com espaço onde há um vão entre eles.
function toLines(items) {
  const rows = [];
  for (const it of items) {
    if (!it.str || !it.str.trim()) continue;
    const [, , , , x, y] = it.transform;
    const h = Math.abs(it.height) || 8;
    let row = rows.find((r) => Math.abs(r.y - y) <= h * 0.45);
    if (!row) rows.push((row = { y, h, parts: [] }));
    row.parts.push({ x, w: it.width, str: it.str });
  }
  rows.sort((a, b) => b.y - a.y); // de cima para baixo
  return rows.map((r) => {
    r.parts.sort((a, b) => a.x - b.x);
    let line = "";
    let end = null;
    for (const p of r.parts) {
      const gap = end == null ? 0 : p.x - end;
      if (line && gap > r.h * 0.15 && !line.endsWith(" ") && !p.str.startsWith(" ")) line += gap > r.h * 2 ? "   " : " ";
      line += p.str;
      end = p.x + p.w;
    }
    return line.replace(/\s+$/, "");
  });
}
