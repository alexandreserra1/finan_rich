# Reconhecimento de texto local

Tesseract.js-core 6.1.2 (Apache-2.0), motor LSTM portátil, sem SIMD e sem engine legada.
O wrapper próprio usa um único Web Worker, encerrável inclusive durante a inicialização.
Não carrega JavaScript de CDN, não usa IA remota e não envia documentos para a internet.

Modelo português: tesseract-ocr/tessdata_fast, versão 4.1.0 (Apache-2.0).
Origem: https://github.com/tesseract-ocr/tessdata_fast/tree/4.1.0
SHA-256: c4932b937207a9514b7514d518b931a99938c02a28a5a5a553f8599ed58b7deb

O motor sem SIMD evita duplicar binários e preserva o orçamento de APK. O modelo rápido
é menor; a revisão continua obrigatória porque OCR pode errar valores e datas.
No app os recursos são empacotados. No site são baixados do próprio Rich no primeiro uso
e guardados pelo service worker; não entram no precache inicial da página.

Reconstrução: `npm ci --prefix tools/import-assets --ignore-scripts` e
`node scripts/build-import-assets.mjs`. O script verifica o hash do modelo antes de copiar.
Hashes e tamanhos dos recursos ficam em resources.json. LICENSE cobre Apache-2.0.
