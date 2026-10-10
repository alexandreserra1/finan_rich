FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /finanrich ./cmd/finanrich

FROM gcr.io/distroless/static-debian12
COPY --from=build /finanrich /finanrich
ENV ADDR=0.0.0.0:8080
EXPOSE 8080
ENTRYPOINT ["/finanrich"]
