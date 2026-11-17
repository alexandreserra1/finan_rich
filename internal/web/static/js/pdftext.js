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
export async function pdfText(bytes, askPassword, { signal, recognize, onProgress = () => {}, onMixedPage = () => {} } = {}) {
  signal?.throwIfAborted();
  const { getDocument, OPS } = await pdfjs();
  signal?.throwIfAborted();
  const task = getDocument({ data: bytes, isEvalSupported: false, useSystemFonts: false });
  task.onPassword = (update, reason) => {
    const password = askPassword(reason === 2 ? "Senha incorreta. Tente de novo:" : "Este PDF tem senha (muitos bancos usam os primeiros dígitos do CPF). Digite a senha:");
    if (password == null) task.destroy();
    else update(password);
  };
  const abort = () => { void task.destroy(); };
  signal?.addEventListener("abort", abort, { once: true });
  try {
    const doc = await task.promise;
    if (doc.numPages > 30) throw new Error("Este PDF tem mais de 30 páginas. Divida-o em arquivos menores.");
    const lines = [];
    let length = 0;
    for (let number = 1; number <= doc.numPages; number++) {
      signal?.throwIfAborted();
      onProgress(`Lendo a página ${number} de ${doc.numPages}…`);
      const page = await doc.getPage(number);
      try {
        const content = await page.getTextContent();
        let text = toLines(content.items).join("\n");
        const operations = recognize ? await page.getOperatorList() : null;
        const images = operations && imageCoverage(operations, OPS);
        const natural = page.getViewport({ scale: 1 });
        if (images?.present && text.trim()) onMixedPage();
        if ((!text.trim() || images?.area > natural.width * natural.height * 0.15) && recognize) {
          const viewport = page.getViewport({ scale: Math.min(2, 1800 / Math.max(natural.width, natural.height)) });
          if (!Number.isFinite(viewport.width * viewport.height) || viewport.width * viewport.height > 3_240_000) throw new Error("Página maior que o limite de leitura.");
          const canvas = document.createElement("canvas");
          canvas.width = Math.ceil(viewport.width);
          canvas.height = Math.ceil(viewport.height);
          try {
            await page.render({ canvasContext: canvas.getContext("2d"), viewport }).promise;
            const image = await new Promise((resolve) => canvas.toBlob(resolve, "image/png"));
            if (!image) throw new Error("Não consegui preparar a página digitalizada.");
            text = await recognize(image);
          } finally { canvas.width = canvas.height = 0; }
        }
        length += text.length;
        if (length > 1024 * 1024) throw new Error("O PDF contém texto demais. Divida-o em arquivos menores.");
        lines.push(text);
      } finally { page.cleanup(); }
    }
    return lines.join("\n");
  } finally {
    signal?.removeEventListener("abort", abort);
    await task.destroy();
  }
}

function imageCoverage(operations, ops) {
  const stack = [];
  let matrix = [1, 0, 0, 1];
  let area = 0;
  let present = false;
  for (let index = 0; index < operations.fnArray.length; index++) {
    const operation = operations.fnArray[index];
    if (operation === ops.save) stack.push([...matrix]);
    else if (operation === ops.restore) matrix = stack.pop() || [1, 0, 0, 1];
    else if (operation === ops.transform) {
      const transform = operations.argsArray[index];
      const [first, second, third, fourth] = matrix;
      matrix = [first * transform[0] + third * transform[1], second * transform[0] + fourth * transform[1], first * transform[2] + third * transform[3], second * transform[2] + fourth * transform[3]];
    } else if ([ops.paintImageXObject, ops.paintInlineImageXObject, ops.paintImageMaskXObject].includes(operation)) {
      present = true;
      area += Math.abs(matrix[0] * matrix[3] - matrix[1] * matrix[2]);
    }
  }
  return { area, present };
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
