# SaxBot

Telegram бот для чата исполнителя Nick Sax https://t.me/kpsspunk

## Требования

### Для Docker
- Docker
- Docker Compose

### Для локальной разработки
- Go 1.25+
- Redis server
- PostgreSQL 16+

## Установка и запуск

### Запуск с Docker

1. **Клонируйте репозиторий:**
```bash
git clone <repository-url>
cd saxbot
```

2. **Настройте переменные окружения:**
```bash
cp env.example .env
# Отредактируйте .env файл, добавив ваш BOT_TOKEN и другие переменные
```

3. **Запустите с помощью Docker Compose:**
```bash
docker-compose up -d
```

4. **Просмотр логов:**
```bash
docker-compose logs -f saxbot
```

5. **Остановка:**
```bash
docker-compose down
```

### Использование Makefile

Для удобства управления проектом можно использовать Makefile:

```bash
# Показать все доступные команды
make help

# Настроить среду разработки
make dev-setup

# Запустить с Docker
make docker-up

# Просмотр логов
make docker-logs-bot

# Остановить
make docker-down

# Полный деплой
make deploy

# Управление базой данных Redis
make docker-info    # Показать содержимое БД
make docker-clear   # Очистить БД Redis
```

### Локальная разработка

#### 1. Установка зависимостей

```bash
go mod download
```

#### 2. Запуск Redis с конфигурацией проекта

Redis должен быть запущен с конфигурационным файлом для локальной разработки:

```bash
redis-server redis.local.conf
```

Данные будут сохраняться в папке `redis_data/`:
- `dump.rdb` - снимки базы данных (RDB)
- `appendonly.aof` - лог всех операций (AOF)

#### 3. Настройка переменных окружения

```bash
export BOT_TOKEN=your_bot_token_here
```

#### 4. Запуск бота

```bash
go run main.go
```

или собрать и запустить:

```bash
go build
./saxbot
```

## Структура проекта

- `main.go` - основная логика бота
- `redis/` - модуль для работы с Redis
  - `redis.go` - функции для работы с Redis
  - `data_struct.go` - структуры данных
- `database/` - модуль для работы с PostgreSQL
  - `data_struct.go` - структуры данных
  - `postgres.go` - функции для работы с PostgreSQL
  - `requests.go` - используемые запросы в базу данных PostgreSQL
  - `sync.go` - функции для синхронизации баз данных Redis и PostgreSQL
- `environment/` - функции для инициализации переменных окружения
- `messages/` - функции для отправки сообщений ботом
- `text_cases/` - текстовые шаблоны для сообщений
- `admins/` - функции для администрирования чата
- `activities/` - интерактивные активности для чата
- `redis.conf` - конфигурация Redis для Docker
- `docker-compose.yml` - конфигурация Docker Compose
- `Dockerfile` - образ для Go приложения
- `.dockerignore` - исключения для Docker build
- `env.example` - пример переменных окружения
- `Makefile` - команды для управления проектом

## Docker конфигурация

### Сервисы

1. **saxbot-postgres** - PostgreSQL база данных (основное хранилище)
2. **saxbot-redis** - Redis сервер (кэш с TTL 30 минут)
3. **saxbot-app** - Go приложение бота

### Volumes

- `postgres_data` - персистентное хранилище PostgreSQL
- `redis_data` - временное хранилище Redis (кэш)
- `./logs` - директория для логов приложения (если потребуется)

### Network

Все сервисы работают в изолированной сети `saxbot-network`

### Health checks

- PostgreSQL: проверка через `pg_isready`
- Redis: проверка через `redis-cli ping`
- SaxBot: проверка процесса через `pgrep`

## Архитектура данных

Проект использует гибридную архитектуру хранения:

### Redis (Кэш)
- **Назначение**: Быстрый доступ к часто используемым данным
- **TTL**: 30 минут для пользовательских данных
- **Fallback**: При отсутствии данных автоматически загружает из PostgreSQL

### PostgreSQL (Основное хранилище)
- **Назначение**: Надежное персистентное хранение всех данных
- **Структура**: GORM модели с автоматической миграцией
- **Синхронизация**: Асинхронное сохранение из Redis

### Процесс работы
1. **Чтение**: Redis → PostgreSQL (fallback) → создание нового
2. **Запись**: Redis (кэш) + PostgreSQL (асинхронно)
3. **Миграция**: Команда "Миграция" для переноса данных из старого Redis

## Структура пользовательских данных

Каждый пользователь имеет следующие поля:
- `Username` - имя пользователя
- `IsAdmin` - флаг администратора
- `Warns` - количество предупреждений
- `Status` - статус пользователя (active, banned, etc.)
- `IsWinner` - флаг победителя квиза
- `AdminPref` - админский префикс (если админ)
