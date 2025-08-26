# Используем официальный образ Go для сборки
FROM golang:1.25-alpine AS builder

# Устанавливаем необходимые пакеты
RUN apk add --no-cache git ca-certificates tzdata

# Создаем рабочую директорию
WORKDIR /build

# Копируем go.mod и go.sum для кеширования зависимостей
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем весь исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o saxbot main.go

# Финальный образ
FROM alpine:latest

# Устанавливаем ca-certificates для HTTPS запросов
RUN apk --no-cache add ca-certificates tzdata

# Создаем пользователя для безопасности
RUN addgroup -S saxbot && adduser -S saxbot -G saxbot

# Создаем рабочую директорию
WORKDIR /app

# Создаем директорию для логов
RUN mkdir -p /app/logs && chown saxbot:saxbot /app/logs

# Копируем скомпилированное приложение из builder stage
COPY --from=builder /build/saxbot .

# Меняем владельца файлов
RUN chown saxbot:saxbot /app/saxbot

# Переключаемся на пользователя saxbot
USER saxbot

# Указываем порт (если потребуется для healthcheck)
EXPOSE 8080

# Healthcheck для проверки состояния приложения
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD pgrep saxbot || exit 1

# Запускаем приложение
CMD ["./saxbot"]
