# Imagem do intermediário grátis (cmd/api) para o Cloud Run.
# Construída a partir da raiz do repositório: veja scripts/deploy-proxy.sh.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /finanproxy ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /finanproxy /finanproxy
USER nonroot
ENTRYPOINT ["/finanproxy"]
