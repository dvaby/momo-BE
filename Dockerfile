# ==========================================
# Stage 1: Build Binary
# ==========================================
FROM golang:alpine AS builder

WORKDIR /app

# Aktifkan auto toolchain jika go.mod membutuhkan versi Go lebih tinggi
ENV GOTOOLCHAIN=auto

# Install git jika ada dependency yang butuh VCS
RUN apk add --no-cache git

# Copy dependency manifest & download modules
COPY go.mod go.sum ./
RUN go mod download

# Copy seluruh source code
COPY . .

# Build binary Golang yang terkompresi & statis
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main ./cmd/api

# ==========================================
# Stage 2: Production Runtime
# ==========================================
FROM alpine:3.19

WORKDIR /app

# WARN: poppler-utils WAJIB ada untuk fitur extract-pdf (pdftotext)
RUN apk add --no-cache poppler-utils ca-certificates tzdata

# Set timezone ke Asia/Jakarta
ENV TZ=Asia/Jakarta

# Copy binary dari stage builder
COPY --from=builder /app/main /app/main

EXPOSE 8080

CMD ["/app/main"]