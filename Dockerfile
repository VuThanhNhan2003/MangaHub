# -------- Build stage --------
FROM golang:1.24.1-alpine AS builder

WORKDIR /app

# Cài git gcc musl sqlite protobuf (go mod cần)
RUN apk add --no-cache git gcc musl-dev sqlite-dev protobuf protobuf-dev

# Copy go mod trước để cache dependency
COPY go.mod go.sum ./
RUN go mod download

# Copy toàn bộ source
COPY . .

# Build binary unified server
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -o mangahub-server cmd/server/main.go

# -------- Runtime stage --------
FROM alpine:latest

WORKDIR /app

# Copy binary từ build stage
COPY --from=builder /app/mangahub-server .

# Tạo thư mục data
RUN mkdir -p /app/data

# Expose ports
EXPOSE 8080
EXPOSE 9090
EXPOSE 9091/udp
EXPOSE 9092

# Default env
ENV PORT=8080 \
    DB_PATH=/app/data/mangahub.db \
    TCP_PORT=9090 \
    UDP_PORT=9091 \
    GRPC_PORT=9092

# Run server
CMD ["./mangahub-server"]
