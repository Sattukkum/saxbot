# Docker Setup для SaxBot

## Что было настроено

### 1. 📦 Docker Compose конфигурация (`docker-compose.yml`)

- **Сервис Redis**: 
  - Образ: `redis:7-alpine`
  - Порт: 6379
  - Персистентное хранилище данных
  - Health check
  - Кастомная конфигурация из `redis.conf`

- **Сервис SaxBot**:
  - Собирается из локального Dockerfile
  - Переменные окружения для подключения к Redis
  - Зависит от Redis (запускается после него)
  - Изолированная сеть

### 2. 🐳 Dockerfile для Go приложения

- **Multi-stage build**:
  - Builder stage: компиляция Go приложения
  - Final stage: минимальный Alpine образ
- **Безопасность**: отдельный пользователь `saxbot`
- **Health check**: проверка процесса
- **Оптимизация**: статическая компиляция без CGO

### 3. ⚙️ Конфигурационные файлы

- **`redis.conf`**: конфигурация Redis для Docker (bind 0.0.0.0, dir /data)
- **`redis.local.conf`**: конфигурация Redis для локальной разработки (bind 127.0.0.1, dir ./redis_data)
- **`.dockerignore`**: исключения для Docker build
- **`env.example`**: пример переменных окружения

### 4. 🛠️ Makefile для автоматизации

Команды для управления проектом:
- `make help` - показать справку
- `make docker-up` - запустить в Docker
- `make docker-logs-bot` - логи бота
- `make redis-local` - запустить Redis локально
- `make deploy` - полный деплой

### 5. 📝 Обновленная документация

- Инструкции по Docker в README.md
- Описание структуры проекта
- Примеры использования

## Изменения в коде

### main.go
```go
// Добавлена поддержка переменных окружения для Redis
redisHost := os.Getenv("REDIS_HOST")
if redisHost == "" {
    redisHost = "localhost"
}
redisPort := os.Getenv("REDIS_PORT")
if redisPort == "" {
    redisPort = "6379"
}
redisAddr := redisHost + ":" + redisPort
```

## Как использовать

### Быстрый старт с Docker:
```bash
# 1. Скопировать переменные окружения
cp env.example .env
# Отредактировать .env, добавив BOT_TOKEN

# 2. Запустить
docker-compose up -d

# 3. Просмотр логов
docker-compose logs -f saxbot
```

### Локальная разработка:
```bash
# 1. Настроить окружение
make dev-setup

# 2. Запустить Redis
make redis-local

# 3. В другом терминале запустить бота
export BOT_TOKEN=your_token
make run
```

## Volumes и персистентность

- **`redis_data`** - Docker volume для данных Redis
- **`./logs`** - локальная папка для логов (если потребуется)
- **`./redis_data/`** - локальная папка для данных Redis при разработке

## Сеть

Все сервисы изолированы в сети `saxbot-network`. Бот подключается к Redis по имени сервиса `redis:6379`.

## Health Checks

- **Redis**: `redis-cli ping`
- **SaxBot**: проверка процесса через `pgrep saxbot`

Это обеспечивает правильный порядок запуска и мониторинг состояния сервисов.
