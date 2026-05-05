.PHONY: test tidy up down

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
