FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api

# ---- Etapa 2: tests (parte del build, con el SDK de Go adentro) ----
FROM builder AS test
ENTRYPOINT ["sh", "scripts/check-coverage.sh"]

# ---- Etapa 3: runtime ----
FROM alpine:3.20
LABEL org.opencontainers.image.source https://github.com/Max20050/ingsoft3
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /api .
EXPOSE 8080
CMD ["./api"]
