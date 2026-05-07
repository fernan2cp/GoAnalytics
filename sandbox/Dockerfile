FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o sandbox-app .

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /app/sandbox-app .
COPY public/ ./public/

ENV PORT=3000

EXPOSE 3000

CMD ["./sandbox-app"]
