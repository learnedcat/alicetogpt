# ---------- build stage ----------
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Зависимости
COPY go.mod go.sum ./
RUN go mod download

# Исходники
COPY . .

# Сборка
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o alice-gpt .

# ---------- runtime stage ----------
FROM alpine:3.20

WORKDIR /app

# TLS сертификаты (нужны для OpenAI)
RUN apk add --no-cache ca-certificates

# Бинарник
COPY --from=builder /app/alice-gpt /app/alice-gpt

EXPOSE 8080

ENTRYPOINT ["/app/alice-gpt"]
