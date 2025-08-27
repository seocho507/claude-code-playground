# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy shared library first
COPY shared/ ./shared/

# Copy service source code
COPY services/auth-service/ ./services/auth-service/
WORKDIR /app/services/auth-service

# Clean and rebuild dependencies (generate fresh go.sum)
RUN rm -f go.sum
RUN go clean -modcache
RUN go mod tidy -v
RUN go mod download -x

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/services/auth-service/main .

# Copy config files from builder stage
COPY --from=builder /app/services/auth-service/config ./config

# Expose port
EXPOSE 8001

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8001/health || exit 1

# Run the binary
CMD ["./main"]