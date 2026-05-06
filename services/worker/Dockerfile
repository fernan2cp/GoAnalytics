FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.work go.work.sum ./
COPY services/ingest/go.mod services/ingest/go.sum ./services/ingest/
COPY services/worker/go.mod services/worker/go.sum ./services/worker/
WORKDIR /src/services/worker
RUN go mod download

WORKDIR /src
COPY services/worker ./services/worker
WORKDIR /src/services/worker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/go-analytics-worker ./cmd/worker

FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /out/go-analytics-worker /app/go-analytics-worker

EXPOSE 8081
ENTRYPOINT ["/app/go-analytics-worker"]
