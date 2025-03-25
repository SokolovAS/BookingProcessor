# Dockerfile.hey
FROM golang:1.18-alpine AS builder
RUN apk add --no-cache git
RUN go install github.com/rakyll/hey@latest

FROM alpine:latest
COPY --from=builder /go/bin/hey /usr/local/bin/hey
ENTRYPOINT ["/usr/local/bin/hey"]
