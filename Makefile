.PHONY: up down logs

up:
	docker compose up --build

down:
	docker compose down -v

logs:
	docker compose logs -f bot
