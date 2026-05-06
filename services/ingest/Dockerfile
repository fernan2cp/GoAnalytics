FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.work go.work.sum ./
COPY services/ingest/go.mod services/ingest/go.sum ./services/ingest/
COPY services/worker/go.mod services/worker/go.sum ./services/worker/
WORKDIR /src/services/ingest
RUN go mod download

WORKDIR /src
COPY services/ingest ./services/ingest
WORKDIR /src/services/ingest
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/go-analytics-ingest ./cmd/ingest

FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=builder /out/go-analytics-ingest /app/go-analytics-ingest

EXPOSE 8080
ENTRYPOINT ["/app/go-analytics-ingest"]
