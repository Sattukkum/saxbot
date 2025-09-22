package redis

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRepository struct {
	Client *redis.Client
	ctx    context.Context
}

func NewRedisRepository(client *redis.Client, ctx context.Context) *RedisRepository {
	return &RedisRepository{Client: client, ctx: ctx}
}

// Инициализация подключения к Redis
func InitRedis(addr, password string, db int) (*redis.Client, error) {
	Client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	ctx := context.Background()
	_, err := Client.Ping(ctx).Result()
	if err != nil {
		return Client, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	log.Println("Successfully connected to Redis")
	return Client, nil
}

// Закрыть подключение к Redis
func CloseRedis(Client *redis.Client) error {
	if Client != nil {
		return Client.Close()
	}
	return nil
}

// Устаонвить значение с TTL
func (r *RedisRepository) SetWithExpiration(key string, value any, expiration time.Duration) error {
	return r.Client.Set(r.ctx, key, value, expiration).Err()
}

// Получить значение по ключу
func (r *RedisRepository) Get(key string) (string, error) {
	val, err := r.Client.Get(r.ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key %s does not exist", key)
	}
	return val, err
}

// Удалить ключ
func (r *RedisRepository) Delete(key string) error {
	return r.Client.Del(r.ctx, key).Err()
}

// Проверить существование ключа
func (r *RedisRepository) Exists(key string) (bool, error) {
	val, err := r.Client.Exists(r.ctx, key).Result()
	return val > 0, err
}

// Увеличить числовое значение на 1
func (r *RedisRepository) Increment(key string) (int64, error) {
	return r.Client.Incr(r.ctx, key).Result()
}

// Увеличить числовое значение на указанное число
func (r *RedisRepository) IncrementBy(key string, value int64) (int64, error) {
	return r.Client.IncrBy(r.ctx, key, value).Result()
}

// Установить время жизни для существующего ключа
func (r *RedisRepository) SetExpiration(key string, expiration time.Duration) error {
	return r.Client.Expire(r.ctx, key, expiration).Err()
}

// Получить оставшееся время жизни ключа
func (r *RedisRepository) GetTTL(key string) (time.Duration, error) {
	return r.Client.TTL(r.ctx, key).Result()
}

// Очистить всю базу данных Redis
func (r *RedisRepository) FlushAll() error {
	return r.Client.FlushAll(r.ctx).Err()
}

// Получить все ключи из Redis
func (r *RedisRepository) GetAllKeys() ([]string, error) {
	return r.Client.Keys(r.ctx, "*").Result()
}

// Получить информацию о базе данных Redis
func (r *RedisRepository) GetDatabaseInfo() (map[string]string, error) {
	info, err := r.Client.Info(r.ctx).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	keys, _ := r.GetAllKeys()
	result["total_keys"] = fmt.Sprintf("%d", len(keys))
	result["redis_info"] = info

	return result, nil
}

// Удалить истекшие ключи
func (r *RedisRepository) CleanupExpiredKeys() error {
	keys, err := r.Client.Keys(r.ctx, "user:*").Result()
	if err != nil {
		return err
	}

	expiredCount := 0
	for _, key := range keys {
		ttl, err := r.Client.TTL(r.ctx, key).Result()
		if err != nil {
			continue
		}
		if ttl < 0 {
			r.Client.Del(r.ctx, key)
			expiredCount++
		}
	}

	log.Printf("Cleanup completed: removed %d expired keys", expiredCount)
	return nil
}

// Получить статистику использования памяти Redis
func (r *RedisRepository) GetMemoryStats() (map[string]any, error) {
	info, err := r.Client.Info(r.ctx, "memory").Result()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]any)
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				stats[parts[0]] = parts[1]
			}
		}
	}

	userKeys, _ := r.Client.Keys(r.ctx, "user:*").Result()
	quizKeys, _ := r.Client.Keys(r.ctx, "quiz_*").Result()
	stats["user_keys_count"] = len(userKeys)
	stats["quiz_keys_count"] = len(quizKeys)

	return stats, nil
}
