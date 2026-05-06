.PHONY: test tidy up up down migrate-up migrate-down migrate-force

test:
	cd services/ingest && go test ./...
	cd services/worker && go test ./...

tidy:
	cd services/ingest && go mod tidy
	cd services/worker && go mod tidy
	go work sync

up:
	docker compose up -d

down:
	docker compose down

migrate-up:
	docker compose --profile tools run --rm migrate -path=/migrations -database "postgres://analytics:analytics@postgres_analytics:5432/analytics?sslmode=disable" up

migrate-down:
	docker compose --profile tools run --rm migrate -path=/migrations -database "postgres://analytics:analytics@postgres_analytics:5432/analytics?sslmode=disable" down 1

migrate-force:
	docker compose --profile tools run --rm migrate -path=/migrations -database "postgres://analytics:analytics@postgres_analytics:5432/analytics?sslmode=disable" force $(VERSION)
