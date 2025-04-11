# Build stage
FROM golang:1.21 as builder
WORKDIR /cmd
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Force a static build for linux/amd64
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o app ./cmd

# Run stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /cmd/app .
EXPOSE 8080
CMD ["./app"]
