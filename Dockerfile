# Set build stage
FROM golang:1.24-alpine AS build
# Install build dependencies for CGO
RUN apk add --no-cache gcc musl-dev
# Set working directory
WORKDIR /app
# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
# Download dependencies
RUN go mod download
# Copy the rest of the source
COPY . .
# Enable CGO for sqlite
ENV CGO_ENABLED=1
# Build
RUN go build -buildvcs=false -o kinozal_monitor cmd/*

# Set runtime stage
FROM alpine:latest
# Install CA certificates for HTTPS requests
RUN apk --no-cache add ca-certificates
# Create a non-root user
RUN addgroup -g 1001 -S appgroup && adduser -u 1001 -S appuser -G appgroup
# Copy binary from build stage
COPY --from=build /app/kinozal_monitor /kinozal_monitor
# Change ownership to the non-root user
RUN chown appuser:appgroup /kinozal_monitor
# Switch to non-root user
USER appuser
# Workdir
WORKDIR /
# Expose port
EXPOSE 8080
# Run binary
ENTRYPOINT ["/kinozal_monitor"]
