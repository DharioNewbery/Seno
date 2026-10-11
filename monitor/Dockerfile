# Dockerfile do Monitor Seno (multi-stage; ARQUITETURA §8.1).
# Build stage: golang; stage final: distroless/static (binário estático,
# migrações embutidas via go:embed).
# Contexto de build: RAIZ do monorepo (contrato compartilhado com a API vai
# junto; compose.prod usa build.context: .. — ver deploy/compose.yaml).
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY contract ./contract
COPY monitor/go.mod monitor/go.sum ./monitor/
COPY monitor/cmd ./monitor/cmd
COPY monitor/db ./monitor/db
COPY monitor/internal ./monitor/internal
RUN CGO_ENABLED=0 GOOS=linux go build -C monitor -trimpath \
	-ldflags="-s -w" -o /bin/seno-monitor ./cmd/monitor

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /bin/seno-monitor /seno-monitor
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/seno-monitor"]
