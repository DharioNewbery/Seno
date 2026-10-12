# Dockerfile da API Seno (multi-stage; ARQUITETURA §8.1).
# Build stage: golang; stage final: distroless/static (binário estático,
# migrações embutidas via go:embed).
# Contexto de build: RAIZ do monorepo (contract/ compartilhado com a API;
# os compose usam build.context: ..).
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY contract ./contract
COPY api/go.mod api/go.sum ./api/
COPY api ./api
RUN CGO_ENABLED=0 GOOS=linux go build -C api -trimpath \
	-ldflags="-s -w" -o /bin/seno-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /bin/seno-api /seno-api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/seno-api"]
