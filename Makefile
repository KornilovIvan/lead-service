DB_URL=postgres://leads:leads@postgres:5432/leads?sslmode=disable
TEST_DB_URL=postgres://leads:leads@postgres:5432/leads_test?sslmode=disable
REDIS_URL=redis://localhost:6379/1

up:
	docker compose up -d --wait postgres redis

down:
	docker compose down

migrate-up:
	docker compose run --rm migrate -path=/migrations -database "$(DB_URL)" up

migrate-down:
	docker compose run --rm migrate -path=/migrations -database "$(DB_URL)" down -all

test-db:
	docker compose exec postgres psql -U leads -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = 'leads_test'" | grep -q 1 || docker compose exec postgres psql -U leads -d postgres -c "CREATE DATABASE leads_test"
	docker compose run --rm migrate -path=/migrations -database "$(TEST_DB_URL)" up

test:
	TEST_DATABASE_URL='postgres://leads:leads@localhost:5432/leads_test?sslmode=disable' REDIS_URL='$(REDIS_URL)' go test ./...

lint:
	docker run --rm -v "$(CURDIR)":/app -w /app golangci/golangci-lint:v1.55.2 golangci-lint run
