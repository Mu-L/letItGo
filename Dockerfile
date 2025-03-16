FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build applications
RUN go build -o ./bin/api ./api
RUN go build -o ./bin/producer ./producer
RUN go build -o ./bin/consumer ./consumer

# Final stage
FROM alpine:latest

WORKDIR /app

# Install CA certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Copy binaries from builder stage
COPY --from=builder /app/bin/ /app/

# Copy .env file
COPY --from=builder /app/.env /app/

ENV API_PORT=8080

# Default command will be overridden in docker-compose.yml
CMD ["./api"]
