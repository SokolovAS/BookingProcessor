# --- Build stage ---
FROM golang:1.22-alpine AS builder

# 1. Set module root
WORKDIR /app

# 2. Cache deps
COPY go.mod go.sum ./
RUN go mod download

# 3. Copy source
COPY . .

# 4. Build statically-linked binary from cmd/
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build -a -installsuffix cgo \
      -o app \
      ./cmd

# --- Run stage ---
FROM alpine:latest

# 5. (Optional) Add CA certs if your app does HTTPS
RUN apk --no-cache add ca-certificates

# 6. App dir
WORKDIR /root/

# 7. Copy binary in
COPY --from=builder /app/app .

# 8. Expose your port
EXPOSE 8080

# 9. Launch
ENTRYPOINT ["./app"]
