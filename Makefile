-include .env
export

.PHONY: db run test docker backup

db:        ## sobe só o PostgreSQL
	docker compose up -d db

run: db    ## roda o app localmente (http://127.0.0.1:8080)
	go run ./cmd/finanrich

test:
	go vet ./... && go test ./...

docker:    ## sobe banco + app em containers
	docker compose --profile app up -d --build

backup:    ## exporta o banco para backup-AAAA-MM-DD.sql
	docker compose exec -T db pg_dump -U finan finan > backup-$$(date +%F).sql
