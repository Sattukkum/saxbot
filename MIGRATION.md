# Миграция данных из Redis в PostgreSQL

Этот документ описывает процесс миграции существующих данных из персистентного хранилища Redis в PostgreSQL.

## 📋 Обзор

С обновлением архитектуры проекта данные теперь хранятся в:
- **Redis** - как кэш с TTL 30 минут
- **PostgreSQL** - как основное персистентное хранилище

## 🚀 Процесс миграции

### 1. Подготовка

1. **Остановите текущий бот:**
```bash
docker-compose down
```

2. **Обновите проект:**
```bash
git pull origin main
```

3. **Создайте резервную копию Redis (опционально):**
```bash
# Если Redis работает локально
redis-cli --rdb backup-$(date +%Y%m%d-%H%M%S).rdb

# Если Redis в Docker
docker-compose up redis -d
docker exec saxbot-redis redis-cli --rdb /data/backup-$(date +%Y%m%d-%H%M%S).rdb
```

### 2. Запуск с PostgreSQL

1. **Запустите новую архитектуру:**
```bash
docker-compose up -d
```

2. **Проверьте статус сервисов:**
```bash
docker-compose ps
```

Должны быть запущены:
- `saxbot-postgres` (healthy)
- `saxbot-redis` (healthy)  
- `saxbot-app` (running)

### 3. Выполнение миграции

1. **В Telegram чате отправьте команду:**
```
Миграция
```

2. **Следите за логами:**
```bash
docker-compose logs -f saxbot
```

### 4. Мониторинг миграции

Во время миграции в логах вы увидите:

```
🚀 Starting migration from Redis persistent storage to PostgreSQL...
✅ PostgreSQL connection verified
📊 Starting user data migration...
📈 Found 150 persistent users in Redis
🔄 Processing user 1/150 (0.7%)
...
✅ Created new user 123456 in PostgreSQL
🔄 Updated user 789012 (fields: [username warns])
⏭️  User 345678 already up-to-date in PostgreSQL
...
📊 Starting quiz data migration...
📈 Found 45 quizzes in Redis
✅ Migrated quiz for date 2024-01-15
...
🎉 Migration completed!
📊 USER MIGRATION STATISTICS:
   Total users found: 150
   Successfully migrated: 142
   Skipped (up-to-date): 5
   Errors: 3
📊 QUIZ MIGRATION STATISTICS:
   Total quizzes found: 45
   Successfully migrated: 45
   Errors: 0
✅ Migration completed successfully without errors!
```

## ⚠️ Возможные проблемы

### Ошибка подключения к PostgreSQL
```
❌ PostgreSQL не подключен. Миграция невозможна.
```

**Решение:**
1. Проверьте статус PostgreSQL: `docker-compose ps postgres`
2. Проверьте логи: `docker-compose logs postgres`
3. Перезапустите сервисы: `docker-compose restart`

### Ошибки миграции отдельных записей
```
⚠️  Failed to create user 123456 in PostgreSQL: duplicate key value
```

**Решение:**
- Это нормально, если пользователь уже существует
- Проверьте финальную статистику - важно, чтобы общее количество ошибок было минимальным

### Таймаут миграции
Если миграция занимает очень много времени:

1. **Проверьте ресурсы системы:**
```bash
docker stats
```

2. **Увеличьте ресурсы для PostgreSQL** в `docker-compose.yml`:
```yaml
postgres:
  deploy:
    resources:
      limits:
        memory: 1G
        cpus: '1.0'
```

## 🔍 Проверка результатов

### 1. Проверка данных в PostgreSQL

```bash
# Подключение к PostgreSQL
docker exec -it saxbot-postgres psql -U saxbot -d saxbot

# Проверка таблиц
\dt

# Количество пользователей
SELECT COUNT(*) FROM users;

# Количество квизов  
SELECT COUNT(*) FROM quizzes;

# Выход
\q
```

### 2. Проверка работы fallback

1. **Очистите Redis кэш:**
```bash
docker exec saxbot-redis redis-cli FLUSHALL
```

2. **Проверьте работу бота** - данные должны автоматически загружаться из PostgreSQL

## 📊 Мониторинг после миграции

### Проверка синхронизации
Каждые 30 минут запускается автоматическая проверка консистентности данных:

```bash
docker-compose logs saxbot | grep "consistency check"
```

### Статистика баз данных
В логах периодически появляется статистика:

```
Redis stats: map[total_keys:25 user_keys_count:20 quiz_keys_count:5]
PostgreSQL stats: map[total_users:150 active_users:142 total_quizzes:45]
```

## 🔄 Откат (если необходимо)

Если что-то пошло не так, можно вернуться к старой версии:

1. **Остановите сервисы:**
```bash
docker-compose down
```

2. **Переключитесь на предыдущую версию:**
```bash
git checkout <previous-commit>
```

3. **Запустите старую версию:**
```bash
docker-compose up -d
```

## ✅ Завершение

После успешной миграции:

1. **Данные автоматически синхронизируются** между Redis и PostgreSQL
2. **Redis служит кэшем** для быстрого доступа
3. **PostgreSQL является источником истины** для всех данных
4. **Fallback работает автоматически** при отсутствии данных в Redis

Миграция завершена! 🎉
