## 1. App e site
- [x] 1.1 Sem nada guardado, vale a IA grátis (`cloud-ai.js`); desligar guarda `{ off: true }`.
- [x] 1.2 Consultor desligado oferece "Ligar a IA grátis" num toque; nome "IA grátis da Rich".
## 2. Computador
- [x] 2.1 Sem a IA local pronta, o `/api/chat` oferece o grátis (`free`) e a tela mostra o botão.
## 3. Documentação
- [x] 3.1 Política de privacidade, IA-GRATIS.md e MOBILE.md.
## 4. Validação
- [x] 4.1 e2e do app em Chromium e WebKit; teste Go do `/api/chat`.
- [ ] 4.2 Conferir no celular de verdade.
  - Emulador (Android 14, APK oficial v1.11.0, 22/11/2026): o Consultor abriu com a IA grátis, sem chave,
    e respondeu pela rede (primeiro texto em 3 s, resposta inteira em 12 s). Falta celular de verdade.
