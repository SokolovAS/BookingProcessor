# Use the official Go image for building
FROM golang:1.18-alpine AS builder
WORKDIR /app

# Copy your debug Go program
COPY main_debug.go .

# Build the debug binary
RUN go build -o debug-app main_debug.go

# Use a lightweight base image for running the binary
FROM alpine:latest
COPY --from=builder /app/debug-app /usr/local/bin/debug-app
EXPOSE 6060
CMD ["debug-app"]
