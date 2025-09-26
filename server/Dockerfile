# Build stage
FROM golang:1.24.4-alpine AS builder
RUN apk add --no-cache git build-base
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /out/chat_server ./cmd

FROM alpine:3.19.0
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /out/chat_server ./chat_server
COPY --from=builder /src/internal/config/config.json ./internal/config/config.json

EXPOSE 8000

ENTRYPOINT ["/app/chat_server"]
