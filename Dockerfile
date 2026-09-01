# ================================
# Stage 1: Build Stage
# ================================
FROM golang:1.23-alpine AS builder
# Izinkan Go mengunduh toolchain yang sesuai dengan go.mod secara otomatis
ENV GOTOOLCHAIN=auto
# Install certs & git
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /app
# Dependency Caching
COPY go.mod go.sum ./
RUN go mod download
# Copy source code
COPY . .
# Build biner Go tanpa CGO
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main ./cmd/api/main.go

# ================================
# Stage 2: Final Runtime Stage
# ================================
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata poppler-utils
WORKDIR /app
# Copy biner dari stage builder
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]