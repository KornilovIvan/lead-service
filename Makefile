DB_URL=postgres://leads:leads@postgres:5432/leads?sslmode=disable
TEST_DB_URL=postgres://leads:leads@postgres:5432/leads_test?sslmode=disable

up:
	docker compose up -d --wait postgres

down:
	docker compose down

migrate-up:
	docker compose run --rm migrate -path=/migrations -database "$(DB_URL)" up

migrate-down:
	docker compose run --rm migrate -path=/migrations -database "$(DB_URL)" down -all

test-db:
	docker compose exec postgres psql -U leads -d postgres -tc "SELECT 1 FROM pg_database WHERE datname = 'leads_test'" | grep -q 1 || docker compose exec postgres psql -U leads -d postgres -c "CREATE DATABASE leads_test"
	docker compose run --rm migrate -path=/migrations -database "$(TEST_DB_URL)" up
