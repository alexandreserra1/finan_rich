# Tarefas: SQLite no celular e na web

Detalhes, critérios de aceite e riscos em [plan.md](plan.md).

## Fase 0: provas
- [x] Tarefa 1: SQLite WebAssembly com OPFS funciona no Chromium e no WebKit (medir tamanho e tempo)
- [ ] Tarefa 2: plugin nativo `@capacitor-community/sqlite` compila no Android e no iOS (limite do APK subiu para 18 MB; falta o CI confirmar)
- [ ] Ponto de controle A: números conferidos com o Alexandre

## Fase 1: web
- [x] Tarefa 3: `db.js` com o motor SQLite da web, desligado por padrão, e2e verdes com ele ligado
- [x] Tarefa 4: migração do IndexedDB (com backup intacto) e SQLite ligado por padrão
- [x] Tarefa 5: uma aba de cada vez ("Usar aqui")
- [ ] Ponto de controle B: `make test`, `make e2e` (Chromium e WebKit), CI, conferência à mão

## Fase 2: celular
- [x] Tarefa 6: motor SQLite nativo no app e migração do IndexedDB do WebView (o código; liga quando o plugin entrar no app)
- [ ] Tarefa 7: teste em Android e iPhone de verdade (Alexandre)
- [ ] Ponto de controle C: confirmação antes de publicar

## Fase 3: docs e versão
- [ ] Tarefa 8: ADR-002, ARCHITECTURE, MOBILE, README, novidades
- [ ] Tarefa 9 (duas versões depois): apagar o backup do IndexedDB
