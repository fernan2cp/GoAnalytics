ifeq ($(OS),Windows_NT)
GO ?= C:/Program Files/Go/bin/go.exe
else
GO ?= go
endif

.PHONY: test tidy up down logs logs-follow migrate-up migrate-down migrate-force build-worker-image build-subject-jobs-image build-query-api-image

ANALYTICS_IMAGE_PREFIX ?= go-analytics

test:
	cd services/ingest && "$(GO)" test ./...
	cd services/worker && "$(GO)" test ./...

tidy:
	cd services/ingest && "$(GO)" mod tidy
	cd services/worker && "$(GO)" mod tidy
	"$(GO)" work sync

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs --tail=100

logs-follow:
	docker compose logs -f

migrate-up:
	docker compose --profile tools run --rm migrate -path=/migrations -database "postgres://analytics:analytics@postgres_analytics:5432/analytics?sslmode=disable" up

migrate-down:
	docker compose --profile tools run --rm migrate -path=/migrations -database "postgres://analytics:analytics@postgres_analytics:5432/analytics?sslmode=disable" down 1

migrate-force:
	docker compose --profile tools run --rm migrate -path=/migrations -database "postgres://analytics:analytics@postgres_analytics:5432/analytics?sslmode=disable" force $(VERSION)

build-worker-image:
	docker build --build-arg WORKER_COMMAND=worker -f services/worker/Dockerfile -t $(ANALYTICS_IMAGE_PREFIX)-worker:local .

# Estos targets quedan preparados para activarse cuando sus comandos se creen
# en TASK-CA-0303 y TASK-CA-0304. Cada imagen conserva un proceso independiente.
build-subject-jobs-image:
	docker build --build-arg WORKER_COMMAND=subject-jobs -f services/worker/Dockerfile -t $(ANALYTICS_IMAGE_PREFIX)-subject-jobs:local .

build-query-api-image:
	docker build --build-arg WORKER_COMMAND=query-api -f services/worker/Dockerfile -t $(ANALYTICS_IMAGE_PREFIX)-query-api:local .
