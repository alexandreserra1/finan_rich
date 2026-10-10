.PHONY: run test lint build release-snapshot docker

run:              ## roda o app (abre o navegador)
	go run ./cmd/finanrich

test:             ## testes + verificações
	test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	go vet ./...
	go test ./...

build:            ## gera executáveis para Windows, macOS e Linux em dist/
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
