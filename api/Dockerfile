# Dockerfile da API Seno (multi-stage; ARQUITETURA §8.1).
# Build stage: golang; stage final: distroless/static (binário estático,
# migrações embutidas via go:embed).
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/seno-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /bin/seno-api /seno-api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/seno-api"]
