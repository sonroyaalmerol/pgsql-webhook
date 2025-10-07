FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o pgsql-webhook .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/pgsql-webhook .

RUN addgroup -g 1000 pgsql && \
    adduser -D -u 1000 -G pgsql pgsql && \
    chown pgsql:pgsql /app/pgsql-webhook

USER pgsql

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ps aux | grep '[p]gsql-webhook' || exit 1

ENTRYPOINT ["./pgsql-webhook"]
