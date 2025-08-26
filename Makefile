.PHONY: help build run docker-build docker-up docker-down docker-logs clean

# Переменные
BINARY_NAME=saxbot
DOCKER_COMPOSE_FILE=docker-compose.yml

help: ## Показать справку
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Собрать Go приложение локально
	go build -o $(BINARY_NAME) main.go

run: ## Запустить приложение локально (требует Redis)
	@echo "Убедитесь, что Redis запущен: redis-server redis.local.conf"
	go run main.go

redis-local: ## Запустить Redis для локальной разработки
	redis-server redis.local.conf

test: ## Запустить тесты
	go test -v ./...

clean: ## Очистить скомпилированные файлы
	rm -f $(BINARY_NAME)
	go clean

# Docker команды
docker-build: ## Собрать Docker образы
	docker-compose -f $(DOCKER_COMPOSE_FILE) build

docker-up: ## Запустить все сервисы в Docker
	docker-compose -f $(DOCKER_COMPOSE_FILE) up -d

docker-down: ## Остановить все сервисы Docker
	docker-compose -f $(DOCKER_COMPOSE_FILE) down

docker-logs: ## Показать логи всех сервисов
	docker-compose -f $(DOCKER_COMPOSE_FILE) logs -f

docker-logs-bot: ## Показать логи только бота
	docker-compose -f $(DOCKER_COMPOSE_FILE) logs -f saxbot

docker-logs-redis: ## Показать логи только Redis
	docker-compose -f $(DOCKER_COMPOSE_FILE) logs -f redis

docker-restart: ## Перезапустить все сервисы
	docker-compose -f $(DOCKER_COMPOSE_FILE) restart

docker-restart-bot: ## Перезапустить только бота
	docker-compose -f $(DOCKER_COMPOSE_FILE) restart saxbot

docker-info: ## Показать информацию о Redis БД
	SAXBOT_ARGS="./saxbot --info" docker-compose -f $(DOCKER_COMPOSE_FILE) run --rm saxbot

docker-clear: ## Запустить с очисткой Redis БД
	SAXBOT_ARGS="./saxbot --clear-redis" docker-compose -f $(DOCKER_COMPOSE_FILE) run --rm saxbot

docker-shell-bot: ## Войти в контейнер бота
	docker-compose -f $(DOCKER_COMPOSE_FILE) exec saxbot sh

docker-shell-redis: ## Войти в Redis CLI
	docker-compose -f $(DOCKER_COMPOSE_FILE) exec redis redis-cli

docker-clean: ## Очистить Docker ресурсы
	docker-compose -f $(DOCKER_COMPOSE_FILE) down -v
	docker system prune -f

# Разработка
dev-setup: ## Настроить среду разработки
	@if [ ! -f .env ]; then cp env.example .env; echo "Создан .env файл. Не забудьте добавить BOT_TOKEN!"; fi
	go mod download

fmt: ## Форматировать код
	go fmt ./...

lint: ## Проверить код линтером (требует golangci-lint)
	golangci-lint run

# Производство
deploy: docker-clean docker-build docker-up ## Полный деплой (очистка + сборка + запуск)
