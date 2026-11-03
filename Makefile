DB_URL=postgres://leads:leads@postgres:5432/leads?sslmode=disable

up:
	docker compose up -d --wait postgres

down:
	docker compose down

migrate-up:
	docker compose run --rm migrate -path=/migrations -database "$(DB_URL)" up

migrate-down:
	docker compose run --rm migrate -path=/migrations -database "$(DB_URL)" down -all
