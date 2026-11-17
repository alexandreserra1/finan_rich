export function createOCR({ signal, onProgress = () => {} } = {}) {
  const worker = new Worker("/static/js/ocr-worker.js");
  let pending;
  let closed = false;
  const close = () => {
    closed = true;
    worker.terminate();
    signal?.removeEventListener("abort", abort);
    if (pending) { clearTimeout(pending.timer); pending.reject(new Error("Leitura cancelada.")); pending = null; }
  };
  const abort = () => close();
  signal?.addEventListener("abort", abort, { once: true });
  worker.onmessage = ({ data }) => {
    if (data.progress) { onProgress(data.progress); return; }
    if (!pending) return;
    const current = pending;
    pending = null;
    clearTimeout(current.timer);
    if (data.error) current.reject(new Error(data.error));
    else current.resolve(data);
  };
  worker.onerror = () => { if (pending) pending.reject(new Error("A leitura local não está disponível. Confira a conexão e tente novamente.")); close(); };
  return {
    close,
    async recognize(blob) {
      signal?.throwIfAborted();
      if (closed || pending) throw new Error("A leitura não está disponível.");
      const image = await blob.arrayBuffer();
      signal?.throwIfAborted();
      if (closed) throw new Error("Leitura cancelada.");
      return new Promise((resolve, reject) => {
        const timer = setTimeout(() => { reject(new Error("A leitura demorou demais. Recorte a imagem ou envie um arquivo menor.")); close(); }, 45000);
        pending = { resolve, reject, timer };
        worker.postMessage({ image }, [image]);
      });
    },
  };
}
