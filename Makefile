# Tarefas do dia a dia. O finance.wasm (regras em Go para o navegador) é gerado
# e não fica no git: "make wasm" (ou go generate ./internal/web) cria o arquivo.
.PHONY: wasm run test test-wasm e2e build release-snapshot docker mobile apk

WASM := internal/web/static/finance.wasm
WASM_PKGS := ./internal/domain/ ./internal/finance/ ./internal/money/ ./internal/report/ ./internal/wasmapi/

wasm:             ## compila as regras para o navegador
	go run ./tools/wasmbuild

$(WASM):
	go run ./tools/wasmbuild

run: $(WASM)      ## roda o app (abre o navegador)
	go run ./cmd/finanrich

test: wasm        ## gofmt, vet e testes do Go (servidor e regras em WebAssembly)
	test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	go vet ./...
	go test ./...
	$(MAKE) test-wasm

test-wasm:        ## roda as regras compiladas para WebAssembly (precisa do Node)
	env -i PATH="$$PATH" HOME="$$HOME" GOPATH="$$(go env GOPATH)" GOCACHE="$$(go env GOCACHE)" GOOS=js GOARCH=wasm \
		go test -exec="$$(go env GOROOT)/lib/wasm/go_js_wasm_exec" $(WASM_PKGS)

e2e: wasm         ## testes no navegador: offline, sincronização e app do celular (precisa do Node)
	go build -o dist/finanrich ./cmd/finanrich
	go run ./tools/mobilebuild mobile/www
	cd e2e && npm install --no-audit --no-fund && npx playwright install chromium && FINANRICH_BIN=../dist/finanrich APP_WWW=../mobile/www npm test

mobile: wasm      ## prepara o app do celular (mobile/www + projeto Android)
	go run ./tools/mobilebuild mobile/www
	cd mobile && npm install --no-audit --no-fund && npx cap sync

apk: mobile       ## compila o APK de teste (precisa do Android SDK e Java 21)
	cd mobile/android && ./gradlew assembleDebug
	@echo "APK: mobile/android/app/build/outputs/apk/debug/app-debug.apk"

build: wasm       ## gera executáveis para Windows, macOS e Linux em dist/
	@mkdir -p dist
	@for target in windows/amd64 darwin/arm64 darwin/amd64 linux/amd64 linux/arm64; do \
		os=$${target%/*}; arch=$${target#*/}; ext=$$( [ $$os = windows ] && echo .exe ); \
		echo "dist/finanrich_$${os}_$${arch}$$ext"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/finanrich_$${os}_$${arch}$$ext ./cmd/finanrich; \
	done

release-snapshot: ## simula a release localmente (precisa do goreleaser)
	goreleaser release --snapshot --clean

docker:           ## sobe o app em Docker
	docker compose up -d --build
