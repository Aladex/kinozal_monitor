# Set build stage
FROM golang:1.19.5-buster AS build
# Copy and build
COPY . /app
WORKDIR /app
# Download dependencies and build
RUN go mod tidy && go build -buildvcs=false -o kinozal_monitor cmd/*
RUN chmod +x /app/kinozal_monitor  # Make sure the binary is executable

# Set runtime stage
FROM golang:1.19.5-buster AS runtime
# Copy binary from build stage
COPY --from=build /app/kinozal_monitor /kinozal_monitor
# Make sure the binary is executable
RUN chmod +x /kinozal_monitor
# Expose port
EXPOSE 8080
# Run binary
ENTRYPOINT ["/kinozal_monitor"]
