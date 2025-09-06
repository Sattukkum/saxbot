-- Инициализация базы данных saxbot
-- Этот файл выполняется автоматически при первом запуске PostgreSQL контейнера

-- Создаем базу данных saxbot (если она еще не создана через переменные окружения)
-- CREATE DATABASE saxbot;

-- Подключаемся к базе saxbot
\c saxbot;

-- Создаем расширения, которые могут пригодиться
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Комментарий о структуре
COMMENT ON DATABASE saxbot IS 'Saxbot Telegram Bot Database - stores user data, quizzes, and bot configuration';

-- Создаем пользователя для приложения (если нужен дополнительный)
-- DO $$ 
-- BEGIN
--     IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'saxbot_app') THEN
--         CREATE ROLE saxbot_app WITH LOGIN PASSWORD 'app_password';
--         GRANT CONNECT ON DATABASE saxbot TO saxbot_app;
--         GRANT USAGE ON SCHEMA public TO saxbot_app;
--         GRANT CREATE ON SCHEMA public TO saxbot_app;
--     END IF;
-- END
-- $$;

-- Настройки производительности для небольшой базы данных
ALTER SYSTEM SET shared_buffers = '256MB';
ALTER SYSTEM SET effective_cache_size = '1GB';
ALTER SYSTEM SET maintenance_work_mem = '64MB';
ALTER SYSTEM SET checkpoint_completion_target = 0.9;
ALTER SYSTEM SET wal_buffers = '16MB';
ALTER SYSTEM SET default_statistics_target = 100;

-- Применяем настройки
SELECT pg_reload_conf();

-- Логируем успешную инициализацию
DO $$
BEGIN
    RAISE NOTICE 'Saxbot PostgreSQL database initialized successfully';
    RAISE NOTICE 'Database: saxbot';
    RAISE NOTICE 'User: saxbot';
    RAISE NOTICE 'Ready for GORM auto-migration';
END
$$;
