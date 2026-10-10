FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /finanrich ./cmd/finanrich \
    && mkdir /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /finanrich /finanrich
COPY --from=build --chown=nonroot:nonroot /data /data
ENV FINAN_ADDR=0.0.0.0:8080 \
    FINAN_DATA_DIR=/data \
    FINAN_NO_BROWSER=1
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/finanrich"]
