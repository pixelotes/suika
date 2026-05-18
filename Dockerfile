FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o suika ./cmd/suika

FROM alpine:3.21

RUN apk add --no-cache tzdata

WORKDIR /app
COPY --from=builder /build/suika .
COPY --from=builder /build/web ./web

EXPOSE 8080

ENTRYPOINT ["./suika", "-config", "/app/config/config.yml"]
