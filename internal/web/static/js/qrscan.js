// Leitor de QR code com a câmera, dentro da própria tela (sem plugin nativo):
// a câmera vem do navegador (getUserMedia) e a leitura é feita pelo jsQR
// (open source, Apache-2.0), carregado só quando preciso. Funciona no app do
// celular e no navegador em endereço seguro (https ou localhost).

let jsQR = null;

function loadJsQR() {
  jsQR ??= new Promise((resolve, reject) => {
    if (window.jsQR) return resolve(window.jsQR);
    const s = Object.assign(document.createElement("script"), { src: "/static/vendor/jsqr/jsQR.min.js" });
    s.onload = () => resolve(window.jsQR);
    s.onerror = () => reject(new Error("não consegui carregar o leitor de QR code"));
    document.head.append(s);
  });
  return jsQR;
}

export const cameraAvailable = () => !!(window.isSecureContext && navigator.mediaDevices?.getUserMedia);

// scanQR abre a câmera e devolve o texto do primeiro QR code lido, ou null
// se a pessoa cancelar.
export async function scanQR(instructions) {
  const decode = await loadJsQR();
  let stream;
  try {
    stream = await navigator.mediaDevices.getUserMedia({ video: { facingMode: "environment" }, audio: false });
  } catch (err) {
    throw new Error(err?.name === "NotAllowedError"
      ? "o app não tem permissão para usar a câmera; libere nas configurações do celular ou digite o código"
      : "não consegui abrir a câmera; digite o endereço e o código");
  }

  const overlay = document.createElement("div");
  overlay.className = "qr-overlay";
  overlay.innerHTML = `<video playsinline muted></video><div class="qr-frame" aria-hidden="true"></div>
    <p class="qr-text"></p><button type="button" class="secondary">Cancelar</button>`;
  overlay.querySelector(".qr-text").textContent = instructions;
  document.body.append(overlay);
  const video = overlay.querySelector("video");
  video.srcObject = stream;
  await video.play().catch(() => {});

  const canvas = document.createElement("canvas");
  const g = canvas.getContext("2d", { willReadFrequently: true });
  return new Promise((resolve) => {
    let done = false;
    const finish = (value) => {
      if (done) return;
      done = true;
      stream.getTracks().forEach((t) => t.stop());
      overlay.remove();
      resolve(value);
    };
    overlay.querySelector("button").addEventListener("click", () => finish(null));
    const tick = () => {
      if (done) return;
      if (video.readyState >= 2 && video.videoWidth) {
        const scale = Math.min(1, 640 / Math.max(video.videoWidth, video.videoHeight));
        canvas.width = Math.round(video.videoWidth * scale);
        canvas.height = Math.round(video.videoHeight * scale);
        g.drawImage(video, 0, 0, canvas.width, canvas.height);
        const img = g.getImageData(0, 0, canvas.width, canvas.height);
        const code = decode(img.data, img.width, img.height, { inversionAttempts: "dontInvert" });
        if (code?.data) return finish(code.data);
      }
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });
}
