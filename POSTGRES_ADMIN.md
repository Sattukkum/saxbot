# Администрирование PostgreSQL в контейнере

## 🔍 Подключение к PostgreSQL внутри контейнера

### Способ 1: Через psql (рекомендуется)

```bash
# Подключение к PostgreSQL контейнеру
docker exec -it saxbot-postgres psql -U saxbot -d saxbot
```

### Способ 2: Через bash с последующим psql

```bash
# Заход в контейнер
docker exec -it saxbot-postgres bash

# Внутри контейнера подключение к базе
psql -U saxbot -d saxbot
```

### Способ 3: Подключение с хоста (если порт открыт)

```bash
# Если у вас установлен psql локально и контейнер запущен с портом 5432
psql -h localhost -p 5432 -U saxbot -d saxbot
```

## 📊 Полезные SQL команды

### Основная информация о базе данных

```sql
-- Список всех таблиц
\dt

-- Описание структуры таблицы
\d users
\d quizzes

-- Размер таблиц
SELECT 
    schemaname,
    tablename,
    attname,
    n_distinct,
    correlation
FROM pg_stats
WHERE tablename IN ('users', 'quizzes');

-- Общая статистика
SELECT 
    'users' as table_name,
    COUNT(*) as total_records,
    COUNT(*) FILTER (WHERE is_admin = true) as admin_count,
    COUNT(*) FILTER (WHERE is_winner = true) as winner_count,
    COUNT(*) FILTER (WHERE status = 'active') as active_count
FROM users
UNION ALL
SELECT 
    'quizzes' as table_name,
    COUNT(*) as total_records,
    COUNT(*) FILTER (WHERE is_active = true) as active_count,
    0 as winner_count,
    0 as active_count_2
FROM quizzes;
```

### Работа с пользователями

```sql
-- Все пользователи
SELECT id, user_id, username, is_admin, warns, status, is_winner, created_at 
FROM users 
ORDER BY created_at DESC;

-- Только админы
SELECT user_id, username, warns, created_at 
FROM users 
WHERE is_admin = true;

-- Пользователи с предупреждениями
SELECT user_id, username, warns, status, created_at 
FROM users 
WHERE warns > 0 
ORDER BY warns DESC;

-- Текущие победители
SELECT user_id, username, admin_pref, created_at 
FROM users 
WHERE is_winner = true;

-- Поиск пользователя по ID
SELECT * FROM users WHERE user_id = 123456789;

-- Поиск пользователя по username
SELECT * FROM users WHERE username ILIKE '%username%';
```

### Работа с квизами

```sql
-- Все квизы
SELECT id, date, quote, song_name, quiz_time, is_active, created_at 
FROM quizzes 
ORDER BY date DESC;

-- Активные квизы
SELECT date, quote, song_name, quiz_time, created_at 
FROM quizzes 
WHERE is_active = true 
ORDER BY date DESC;

-- Квиз на сегодня
SELECT * FROM quizzes 
WHERE date = CURRENT_DATE 
ORDER BY created_at DESC 
LIMIT 1;

-- Статистика по месяцам
SELECT 
    DATE_TRUNC('month', date) as month,
    COUNT(*) as total_quizzes,
    COUNT(*) FILTER (WHERE is_active = false) as completed_quizzes
FROM quizzes 
GROUP BY DATE_TRUNC('month', date)
ORDER BY month DESC;
```

### Административные команды

```sql
-- Сброс статуса победителя у всех пользователей
UPDATE users SET is_winner = false WHERE is_winner = true;

-- Деактивация всех старых квизов
UPDATE quizzes SET is_active = false WHERE date < CURRENT_DATE;

-- Добавление предупреждения пользователю
UPDATE users SET warns = warns + 1 WHERE user_id = 123456789;

-- Сброс предупреждений пользователю
UPDATE users SET warns = 0 WHERE user_id = 123456789;

-- Бан пользователя
UPDATE users SET status = 'banned' WHERE user_id = 123456789;

-- Разбан пользователя
UPDATE users SET status = 'active' WHERE user_id = 123456789;
```

### Выход из psql

```sql
-- Выход из psql
\q
```

## 🔧 Полезные команды для отладки

### Проверка подключений

```sql
-- Активные подключения к базе
SELECT pid, usename, application_name, client_addr, state, query_start 
FROM pg_stat_activity 
WHERE datname = 'saxbot';

-- Размер базы данных
SELECT pg_size_pretty(pg_database_size('saxbot')) as database_size;

-- Размер таблиц
SELECT 
    tablename,
    pg_size_pretty(pg_total_relation_size(tablename::regclass)) as size
FROM pg_tables 
WHERE schemaname = 'public';
```

### Анализ производительности

```sql
-- Статистика по таблицам
SELECT 
    schemaname,
    tablename,
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes,
    n_live_tup as live_tuples
FROM pg_stat_user_tables;

-- Индексы и их использование
SELECT 
    indexrelname as index_name,
    idx_tup_read,
    idx_tup_fetch,
    idx_scan
FROM pg_stat_user_indexes;
```

## ⚠️ Важные замечания

1. **Пароль**: При подключении может потребоваться пароль. По умолчанию это `saxbot_password` (или значение из переменной `POSTGRES_PASSWORD`)

2. **Кодировка**: База данных использует UTF-8, поэтому русский текст отображается корректно

3. **Часовой пояс**: Все времена хранятся в UTC, но квизы работают с московским временем (UTC+3)

4. **Backup**: Всегда делайте backup перед выполнением UPDATE/DELETE операций

5. **Производительность**: Для небольших таблиц (~500 пользователей) дополнительные индексы не нужны
