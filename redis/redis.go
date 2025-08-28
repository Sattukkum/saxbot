package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	Client *redis.Client
	ctx    = context.Background()
)

// InitRedis инициализирует подключение к Redis
func InitRedis(addr, password string, db int) error {
	Client = redis.NewClient(&redis.Options{
		Addr:     addr,     // localhost:6379
		Password: password, // пароль, если есть
		DB:       db,       // номер базы данных
	})

	// Проверяем подключение
	_, err := Client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	log.Println("Successfully connected to Redis")
	return nil
}

// CloseRedis закрывает подключение к Redis
func CloseRedis() error {
	if Client != nil {
		return Client.Close()
	}
	return nil
}

// SetWithExpiration устанавливает значение с временем жизни
func SetWithExpiration(key string, value interface{}, expiration time.Duration) error {
	return Client.Set(ctx, key, value, expiration).Err()
}

// Get получает значение по ключу
func Get(key string) (string, error) {
	val, err := Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key %s does not exist", key)
	}
	return val, err
}

// Delete удаляет ключ
func Delete(key string) error {
	return Client.Del(ctx, key).Err()
}

// Exists проверяет существование ключа
func Exists(key string) (bool, error) {
	val, err := Client.Exists(ctx, key).Result()
	return val > 0, err
}

// Increment увеличивает числовое значение на 1
func Increment(key string) (int64, error) {
	return Client.Incr(ctx, key).Result()
}

// IncrementBy увеличивает числовое значение на указанное число
func IncrementBy(key string, value int64) (int64, error) {
	return Client.IncrBy(ctx, key, value).Result()
}

// SetExpiration устанавливает время жизни для существующего ключа
func SetExpiration(key string, expiration time.Duration) error {
	return Client.Expire(ctx, key, expiration).Err()
}

// GetTTL получает оставшееся время жизни ключа
func GetTTL(key string) (time.Duration, error) {
	return Client.TTL(ctx, key).Result()
}

// GetUser получает данные пользователя из Redis (сначала из памяти, затем с диска)
func GetUser(userID int64) (*UserData, error) {
	// Сначала ищем в оперативной памяти (с TTL)
	key := fmt.Sprintf("user:%d", userID)
	val, err := Client.Get(ctx, key).Result()
	if err == nil {
		// Данные найдены в памяти
		var userData UserData
		err = json.Unmarshal([]byte(val), &userData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal user data from memory: %v", err)
		}

		// Проверяем и обновляем админский статус
		if UpdateUserAdminStatus(userID, &userData) {
			// Если статус изменился, сохраняем обновленные данные
			SetUser(userID, &userData)
			SetUserPersistent(userID, &userData)
		}

		return &userData, nil
	}

	if err != redis.Nil {
		return nil, err
	}

	// Если не найдены в памяти, ищем на диске (персистентные данные)
	persistentKey := fmt.Sprintf("user_persistent:%d", userID)
	val, err = Client.Get(ctx, persistentKey).Result()
	if err == nil {
		// Данные найдены на диске, загружаем в память с TTL
		var userData UserData
		err = json.Unmarshal([]byte(val), &userData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal user data from disk: %v", err)
		}

		// Проверяем и обновляем админский статус
		statusChanged := UpdateUserAdminStatus(userID, &userData)

		// Сохраняем в память с TTL для быстрого доступа
		SetUser(userID, &userData)

		// Если статус изменился, также обновляем персистентные данные
		if statusChanged {
			SetUserPersistent(userID, &userData)
		}

		return &userData, nil
	}

	if err != redis.Nil {
		return nil, err
	}

	// Пользователь не найден ни в памяти, ни на диске - создаем нового
	admins := os.Getenv("ADMINS")
	if admins == "" {
		log.Printf("ADMINS environment variable is empty")
	}

	adminInts := make([]int, 0)
	for _, s := range strings.Split(admins, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if adminInt, err := strconv.Atoi(s); err == nil {
			adminInts = append(adminInts, adminInt)
		} else {
			log.Printf("Failed to parse admin ID '%s': %v", s, err)
		}
	}

	var userData *UserData
	if slices.Contains(adminInts, int(userID)) {
		log.Printf("userID: %d is admin", userID)
		userData = &UserData{Username: "", IsAdmin: true, Reputation: 0, Warns: 0, Status: "active"}
	} else {
		log.Printf("userID: %d is not admin", userID)
		userData = &UserData{Username: "", IsAdmin: false, Reputation: 0, Warns: 0, Status: "active"}
	}

	// Сохраняем нового пользователя и в память, и на диск
	SetUser(userID, userData)
	SetUserPersistent(userID, userData)

	return userData, nil
}

// SetUser сохраняет данные пользователя в Redis с TTL 30 минут
func SetUser(userID int64, userData *UserData) error {
	key := fmt.Sprintf("user:%d", userID)
	data, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %v", err)
	}
	// Устанавливаем TTL 30 минут для хранения в оперативной памяти
	return Client.Set(ctx, key, data, 30*time.Minute).Err()
}

// SetUserPersistent сохраняет данные пользователя в Redis без TTL (для сохранения на диск)
func SetUserPersistent(userID int64, userData *UserData) error {
	key := fmt.Sprintf("user_persistent:%d", userID)
	data, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %v", err)
	}
	// Сохраняем без TTL для персистентности на диске
	return Client.Set(ctx, key, data, 0).Err()
}

// UpdateUserReputation обновляет репутацию пользователя
func UpdateUserReputation(userID int64, delta int) error {
	userData, err := GetUser(userID)
	if err != nil {
		return err
	}
	userData.Reputation += delta

	// Сохраняем и в память (с TTL), и на диск (персистентно)
	err = SetUser(userID, userData)
	if err != nil {
		return err
	}
	return SetUserPersistent(userID, userData)
}

// UpdateUserWarns обновляет количество предупреждений пользователя
func UpdateUserWarns(userID int64, delta int) error {
	userData, err := GetUser(userID)
	if err != nil {
		return err
	}
	userData.Warns += delta
	if userData.Warns < 0 {
		userData.Warns = 0
	}

	// Сохраняем и в память (с TTL), и на диск (персистентно)
	err = SetUser(userID, userData)
	if err != nil {
		return err
	}
	return SetUserPersistent(userID, userData)
}

// UpdateUserAdminStatus обновляет админский статус пользователя на основе переменной окружения ADMINS
func UpdateUserAdminStatus(userID int64, userData *UserData) bool {
	admins := os.Getenv("ADMINS")
	if admins == "" {
		return false
	}

	adminInts := make([]int, 0)
	for _, s := range strings.Split(admins, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if adminInt, err := strconv.Atoi(s); err == nil {
			adminInts = append(adminInts, adminInt)
		}
	}

	newAdminStatus := slices.Contains(adminInts, int(userID))
	if userData.IsAdmin != newAdminStatus {
		log.Printf("Updating admin status for user %d: %t -> %t", userID, userData.IsAdmin, newAdminStatus)
		userData.IsAdmin = newAdminStatus
		return true // Статус изменился
	}
	return false // Статус не изменился
}

// RefreshAllUsersAdminStatus обновляет админский статус для всех существующих пользователей
func RefreshAllUsersAdminStatus() error {
	log.Printf("Starting admin status refresh for all users...")

	// Получаем все ключи пользователей (персистентные)
	keys, err := Client.Keys(ctx, "user_persistent:*").Result()
	if err != nil {
		return fmt.Errorf("failed to get user keys: %v", err)
	}

	updatedCount := 0
	for _, key := range keys {
		// Извлекаем userID из ключа (формат: user_persistent:123456)
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			continue
		}

		userID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			log.Printf("Failed to parse userID from key %s: %v", key, err)
			continue
		}

		// Получаем данные пользователя
		val, err := Client.Get(ctx, key).Result()
		if err != nil {
			log.Printf("Failed to get user data for %d: %v", userID, err)
			continue
		}

		var userData UserData
		err = json.Unmarshal([]byte(val), &userData)
		if err != nil {
			log.Printf("Failed to unmarshal user data for %d: %v", userID, err)
			continue
		}

		// Обновляем админский статус
		if UpdateUserAdminStatus(userID, &userData) {
			// Сохраняем обновленные данные
			err = SetUserPersistent(userID, &userData)
			if err != nil {
				log.Printf("Failed to save updated user data for %d: %v", userID, err)
				continue
			}

			// Также обновляем в памяти, если пользователь там есть
			memKey := fmt.Sprintf("user:%d", userID)
			if Client.Exists(ctx, memKey).Val() > 0 {
				SetUser(userID, &userData)
			}

			updatedCount++
		}
	}

	log.Printf("Admin status refresh completed. Updated %d users out of %d total.", updatedCount, len(keys))
	return nil
}

// FlushAll очищает всю базу данных Redis
func FlushAll() error {
	return Client.FlushAll(ctx).Err()
}

// GetAllKeys получает все ключи из базы данных
func GetAllKeys() ([]string, error) {
	return Client.Keys(ctx, "*").Result()
}

// GetDatabaseInfo получает информацию о базе данных
func GetDatabaseInfo() (map[string]string, error) {
	info, err := Client.Info(ctx).Result()
	if err != nil {
		return nil, err
	}

	// Парсим основную информацию
	result := make(map[string]string)
	keys, _ := GetAllKeys()
	result["total_keys"] = fmt.Sprintf("%d", len(keys))
	result["redis_info"] = info

	return result, nil
}

// CleanupExpiredKeys принудительно удаляет истекшие ключи для освобождения памяти
func CleanupExpiredKeys() error {
	// Получаем все ключи с TTL
	keys, err := Client.Keys(ctx, "user:*").Result()
	if err != nil {
		return err
	}

	expiredCount := 0
	for _, key := range keys {
		ttl, err := Client.TTL(ctx, key).Result()
		if err != nil {
			continue
		}
		// Если TTL истек (отрицательное значение), удаляем ключ
		if ttl < 0 {
			Client.Del(ctx, key)
			expiredCount++
		}
	}

	log.Printf("Cleanup completed: removed %d expired keys", expiredCount)
	return nil
}

// GetMemoryStats получает статистику использования памяти
func GetMemoryStats() (map[string]interface{}, error) {
	info, err := Client.Info(ctx, "memory").Result()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]interface{})
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				stats[parts[0]] = parts[1]
			}
		}
	}

	// Добавляем количество ключей разных типов
	memoryKeys, _ := Client.Keys(ctx, "user:*").Result()
	persistentKeys, _ := Client.Keys(ctx, "user_persistent:*").Result()
	stats["memory_keys_count"] = len(memoryKeys)
	stats["persistent_keys_count"] = len(persistentKeys)

	return stats, nil
}
