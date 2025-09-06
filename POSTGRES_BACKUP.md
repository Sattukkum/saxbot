# Бэкап и перенос PostgreSQL базы данных

## 📦 Создание дампа базы данных

### Способ 1: Полный дамп базы данных

```bash
# Создание полного дампа (структура + данные)
docker exec saxbot-postgres pg_dump -U saxbot -d saxbot > backup_$(date +%Y%m%d_%H%M%S).sql

# Создание сжатого дампа
docker exec saxbot-postgres pg_dump -U saxbot -d saxbot | gzip > backup_$(date +%Y%m%d_%H%M%S).sql.gz
```

### Способ 2: Дамп только данных (без структуры)

```bash
# Только данные
docker exec saxbot-postgres pg_dump -U saxbot -d saxbot --data-only > data_backup_$(date +%Y%m%d_%H%M%S).sql

# Только структура
docker exec saxbot-postgres pg_dump -U saxbot -d saxbot --schema-only > schema_backup_$(date +%Y%m%d_%H%M%S).sql
```

### Способ 3: Дамп конкретных таблиц

```bash
# Только пользователи
docker exec saxbot-postgres pg_dump -U saxbot -d saxbot -t users > users_backup_$(date +%Y%m%d_%H%M%S).sql

# Только квизы
docker exec saxbot-postgres pg_dump -U saxbot -d saxbot -t quizzes > quizzes_backup_$(date +%Y%m%d_%H%M%S).sql

# Несколько таблиц
docker exec saxbot-postgres pg_dump -U saxbot -d saxbot -t users -t quizzes > main_tables_backup_$(date +%Y%m%d_%H%M%S).sql
```

### Способ 4: Бинарный дамп (быстрее для больших баз)

```bash
# Создание бинарного дампа
docker exec saxbot-postgres pg_dump -U saxbot -d saxbot -Fc > backup_$(date +%Y%m%d_%H%M%S).dump

# Просмотр содержимого бинарного дампа
docker exec saxbot-postgres pg_restore --list backup_20241201_143022.dump
```

## 🚀 Перенос базы данных на сервер

### Подготовка на исходном сервере

1. **Остановите бота для консистентности данных:**
```bash
docker-compose stop saxbot
```

2. **Создайте дамп:**
```bash
# Полный дамп с сжатием
docker exec saxbot-postgres pg_dump -U saxbot -d saxbot | gzip > saxbot_full_backup_$(date +%Y%m%d_%H%M%S).sql.gz

# Проверьте размер файла
ls -lh saxbot_full_backup_*.sql.gz
```

3. **Создайте также дамп переменных окружения:**
```bash
# Сохраните .env файл
cp .env env_backup_$(date +%Y%m%d_%H%M%S)
```

### Передача файлов на целевой сервер

```bash
# Через SCP
scp saxbot_full_backup_*.sql.gz user@target-server:/path/to/saxbot/

# Через rsync (более надежно)
rsync -avz --progress saxbot_full_backup_*.sql.gz user@target-server:/path/to/saxbot/

# Через Docker volume (если используете общее хранилище)
docker cp saxbot_full_backup_*.sql.gz target-container:/backup/
```

### Восстановление на целевом сервере

1. **Подготовьте окружение:**
```bash
# Перейдите в папку проекта на целевом сервере
cd /path/to/saxbot

# Убедитесь, что у вас есть актуальный код
git pull origin main

# Настройте переменные окружения
cp env_backup_* .env
# Отредактируйте .env при необходимости
```

2. **Запустите только PostgreSQL:**
```bash
# Запустите только PostgreSQL для восстановления
docker-compose up -d postgres

# Дождитесь готовности
docker-compose logs postgres
```

3. **Восстановите данные:**

#### Из обычного SQL дампа:
```bash
# Распакуйте дамп
gunzip saxbot_full_backup_*.sql.gz

# Восстановите данные
cat saxbot_full_backup_*.sql | docker exec -i saxbot-postgres psql -U saxbot -d saxbot

# Или через копирование в контейнер
docker cp saxbot_full_backup_*.sql saxbot-postgres:/tmp/
docker exec saxbot-postgres psql -U saxbot -d saxbot -f /tmp/saxbot_full_backup_*.sql
```

#### Из бинарного дампа:
```bash
# Восстановление бинарного дампа
docker cp backup_*.dump saxbot-postgres:/tmp/
docker exec saxbot-postgres pg_restore -U saxbot -d saxbot /tmp/backup_*.dump
```

4. **Проверьте восстановление:**
```bash
# Подключитесь к базе и проверьте данные
docker exec -it saxbot-postgres psql -U saxbot -d saxbot

# Внутри psql:
\dt                                    # Список таблиц
SELECT COUNT(*) FROM users;            # Количество пользователей
SELECT COUNT(*) FROM quizzes;          # Количество квизов
SELECT * FROM users LIMIT 5;          # Первые 5 пользователей
\q                                     # Выход
```

5. **Запустите полную систему:**
```bash
# Запустите все сервисы
docker-compose up -d

# Проверьте статус
docker-compose ps

# Следите за логами
docker-compose logs -f saxbot
```

## 🔄 Автоматический бэкап

### Создание скрипта автоматического бэкапа

Создайте файл `backup.sh`:

```bash
#!/bin/bash

# Настройки
BACKUP_DIR="/path/to/backups"
RETENTION_DAYS=30
DATE=$(date +%Y%m%d_%H%M%S)

# Создаем директорию для бэкапов
mkdir -p $BACKUP_DIR

# Создаем дамп
echo "Creating backup..."
docker exec saxbot-postgres pg_dump -U saxbot -d saxbot | gzip > $BACKUP_DIR/saxbot_backup_$DATE.sql.gz

# Проверяем размер
BACKUP_SIZE=$(du -h $BACKUP_DIR/saxbot_backup_$DATE.sql.gz | cut -f1)
echo "Backup created: saxbot_backup_$DATE.sql.gz ($BACKUP_SIZE)"

# Удаляем старые бэкапы
echo "Cleaning old backups..."
find $BACKUP_DIR -name "saxbot_backup_*.sql.gz" -mtime +$RETENTION_DAYS -delete

echo "Backup completed successfully!"
```

### Настройка cron для автоматических бэкапов

```bash
# Редактируем crontab
crontab -e

# Добавляем строку для ежедневного бэкапа в 3:00
0 3 * * * /path/to/saxbot/backup.sh >> /var/log/saxbot_backup.log 2>&1

# Или еженедельный бэкап по воскресеньям в 2:00
0 2 * * 0 /path/to/saxbot/backup.sh >> /var/log/saxbot_backup.log 2>&1
```

## 🔧 Продвинутые операции

### Создание дампа с исключениями

```bash
# Исключить определенные таблицы
docker exec saxbot-postgres pg_dump -U saxbot -d saxbot --exclude-table=user_activities > backup_without_activities.sql

# Только определенные таблицы
docker exec saxbot-postgres pg_dump -U saxbot -d saxbot -t users -t quizzes > main_tables_only.sql
```

### Восстановление с очисткой

```bash
# Очистить базу перед восстановлением
docker exec saxbot-postgres psql -U saxbot -d saxbot -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

# Восстановить данные
cat backup.sql | docker exec -i saxbot-postgres psql -U saxbot -d saxbot
```

### Миграция между версиями

```bash
# Если нужно обновить версию PostgreSQL
# 1. Создайте дамп из старой версии
docker exec old-postgres pg_dump -U saxbot -d saxbot > migration_backup.sql

# 2. Остановите старый контейнер
docker-compose down

# 3. Обновите docker-compose.yml с новой версией PostgreSQL
# 4. Запустите новый контейнер
docker-compose up -d postgres

# 5. Восстановите данные
cat migration_backup.sql | docker exec -i saxbot-postgres psql -U saxbot -d saxbot
```

## 📋 Проверочный чек-лист для переноса

### Перед переносом:
- [ ] Остановлен бот на исходном сервере
- [ ] Создан полный дамп базы данных
- [ ] Сохранены переменные окружения (.env)
- [ ] Проверен размер дампа (должен быть > 0)
- [ ] Дамп успешно сжат (если используется gzip)

### На целевом сервере:
- [ ] Установлен Docker и Docker Compose
- [ ] Скопированы файлы проекта
- [ ] Настроены переменные окружения
- [ ] Запущен PostgreSQL контейнер
- [ ] Проверено подключение к базе
- [ ] Восстановлен дамп
- [ ] Проверено количество записей
- [ ] Запущена полная система
- [ ] Проверена работа бота

### После переноса:
- [ ] Протестирована команда "Миграция" (должна показать что данные уже есть)
- [ ] Проверена работа квизов
- [ ] Проверена работа админских команд
- [ ] Настроен автоматический бэкап
- [ ] Удален дамп с исходного сервера (после подтверждения работы)

## 🆘 Восстановление при проблемах

### Если что-то пошло не так:

```bash
# 1. Остановите все сервисы
docker-compose down

# 2. Удалите volume PostgreSQL (ВНИМАНИЕ: удалит все данные!)
docker volume rm saxbot_postgres_data

# 3. Перезапустите PostgreSQL
docker-compose up -d postgres

# 4. Восстановите из бэкапа
cat backup.sql | docker exec -i saxbot-postgres psql -U saxbot -d saxbot

# 5. Запустите все сервисы
docker-compose up -d
```

### Проверка целостности данных:

```sql
-- Проверка на дублирование пользователей
SELECT user_id, COUNT(*) 
FROM users 
GROUP BY user_id 
HAVING COUNT(*) > 1;

-- Проверка на дублирование квизов
SELECT date, COUNT(*) 
FROM quizzes 
GROUP BY date 
HAVING COUNT(*) > 1;

-- Проверка консистентности данных
SELECT 
    (SELECT COUNT(*) FROM users) as total_users,
    (SELECT COUNT(*) FROM users WHERE is_admin = true) as admin_users,
    (SELECT COUNT(*) FROM users WHERE is_winner = true) as winner_users,
    (SELECT COUNT(*) FROM quizzes) as total_quizzes,
    (SELECT COUNT(*) FROM quizzes WHERE is_active = true) as active_quizzes;
```
