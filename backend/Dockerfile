# ── Stage 1: builder ─────────────────────────────────────────────────────────
FROM golang:alpine AS builder

WORKDIR /build

# Cache de dependências separado do código
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app ./cmd/server

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1001 appuser

WORKDIR /app

COPY --from=builder /app /app/server

USER appuser

EXPOSE 8080

CMD ["/app/server"]
