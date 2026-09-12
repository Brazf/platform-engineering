# ── Stage 1: builder ─────────────────────────────────────────────────────────
FROM golang:alpine AS builder

WORKDIR /build

# Cache de dependências separado do código
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /k3s-bootstrap ./cmd/k3s-bootstrap

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM hashicorp/terraform:1.9 AS terraform

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1001 appuser && \
    mkdir -p /var/lib/proxlab/terraform && \
    chown appuser:appuser /var/lib/proxlab/terraform

WORKDIR /app

COPY --from=terraform /bin/terraform /usr/local/bin/terraform
COPY --from=builder /app /app/server
COPY --from=builder /k3s-bootstrap /app/k3s-bootstrap

USER appuser

ENV TERRAFORM_BIN=/usr/local/bin/terraform \
    TERRAFORM_WORKDIR=/var/lib/proxlab/terraform

EXPOSE 8080

CMD ["/app/server"]
