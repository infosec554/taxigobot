# Build Stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the application
# -ldflags="-s -w" reduces binary size by stripping debug info
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o main ./cmd/main.go

# Final Run Stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for external API calls (Telegram, Gmail, etc.)
RUN apk --no-cache add ca-certificates tzdata

# Set timezone (optional, useful for logs)
ENV TZ=Asia/Tashkent

COPY --from=builder /app/main .
COPY --from=builder /app/migrations ./migrations

# Expose the application port
EXPOSE 8080

CMD ["./main"]
