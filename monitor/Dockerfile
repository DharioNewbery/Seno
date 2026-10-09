# Dockerfile do Monitor Seno (multi-stage; mesmo padrão da API, ARQUITETURA §8.1).
# Build stage: golang; stage final: distroless/static (binário estático,
# migrações embutidas via go:embed).
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/seno-monitor ./cmd/monitor

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /bin/seno-monitor /seno-monitor
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/seno-monitor"]
# shortcut: build não inclui contract/ (replace direto pelo go.work em dev não
# alcança o build isolado da imagem). Quando contract/ ganhar código (Fase 2),
# este COPY precisa de contexto na raiz do monorepo — ver PLANO_MONITOR.
