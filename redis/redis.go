package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"saxbot/environment"
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

// Инициализация подключения к Redis
func InitRedis(addr, password string, db int) error {
	Client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	_, err := Client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	log.Println("Successfully connected to Redis")
	return nil
}

// Закрыть подключение к Redis
func CloseRedis() error {
	if Client != nil {
		return Client.Close()
	}
	return nil
}

// Устаонвить значение с TTL
func SetWithExpiration(key string, value any, expiration time.Duration) error {
	return Client.Set(ctx, key, value, expiration).Err()
}

// Получить значение по ключу
func Get(key string) (string, error) {
	val, err := Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key %s does not exist", key)
	}
	return val, err
}

// Удалить ключ
func Delete(key string) error {
	return Client.Del(ctx, key).Err()
}

// Проверить существование ключа
func Exists(key string) (bool, error) {
	val, err := Client.Exists(ctx, key).Result()
	return val > 0, err
}

// Увеличить числовое значение на 1
func Increment(key string) (int64, error) {
	return Client.Incr(ctx, key).Result()
}

// Увеличить числовое значение на указанное число
func IncrementBy(key string, value int64) (int64, error) {
	return Client.IncrBy(ctx, key, value).Result()
}

// Установить время жизни для существующего ключа
func SetExpiration(key string, expiration time.Duration) error {
	return Client.Expire(ctx, key, expiration).Err()
}

// Получить оставшееся время жизни ключа
func GetTTL(key string) (time.Duration, error) {
	return Client.TTL(ctx, key).Result()
}

// Получить данные пользователя
func GetUser(userID int64) (*UserData, error) {
	key := fmt.Sprintf("user:%d", userID)
	val, err := Client.Get(ctx, key).Result()
	if err != nil {
		return &UserData{}, err
	}
	var userData UserData
	err = json.Unmarshal([]byte(val), &userData)
	if err != nil {
		log.Printf("Failed to unmarshal user data from Redis: %v", err)
		return &UserData{}, err
	} else {
		if UpdateUserAdminStatus(userID, &userData) {
			SetUser(userID, &userData)
		}
		return &userData, nil
	}
}

// Сохранить данные пользователя
func SetUser(userID int64, userData *UserData) error {
	key := fmt.Sprintf("user:%d", userID)
	data, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %v", err)
	}
	return Client.Set(ctx, key, data, 30*time.Minute).Err()
}

// Обновить количество предупреждений пользователя
func UpdateUserWarns(userID int64, delta int) error {
	userData, err := GetUser(userID)
	if err != nil {
		return err
	}
	userData.Warns += delta
	if userData.Warns < 0 {
		userData.Warns = 0
	}

	return SetUser(userID, userData)
}

// Обновить админский статус пользователя на основе переменной окружения ADMINS. При изменении статуса - true
func UpdateUserAdminStatus(userID int64, userData *UserData) bool {
	admins := environment.GetAdmins()
	if len(admins) == 0 {
		return false
	}

	newAdminStatus := slices.Contains(admins, userID)
	if userData.IsAdmin != newAdminStatus {
		log.Printf("Updating admin status for user %d: %t -> %t", userID, userData.IsAdmin, newAdminStatus)
		userData.IsAdmin = newAdminStatus
		return true
	}
	return false
}

// Обновить админский статус для всех пользователей в Redis
func RefreshAllUsersAdminStatus() error {
	log.Printf("Starting admin status refresh for users in Redis...")

	keys, err := Client.Keys(ctx, "user:*").Result()
	if err != nil {
		return fmt.Errorf("failed to get user keys: %v", err)
	}

	updatedCount := 0
	for _, key := range keys {
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			continue
		}

		userID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			log.Printf("Failed to parse userID from key %s: %v", key, err)
			continue
		}

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

		if UpdateUserAdminStatus(userID, &userData) {
			err = SetUser(userID, &userData)
			if err != nil {
				log.Printf("Failed to save updated user data for %d: %v", userID, err)
				continue
			}

			updatedCount++
		}
	}

	log.Printf("Admin status refresh completed. Updated %d users out of %d total.", updatedCount, len(keys))
	return nil
}

// Очистить всю базу данных Redis
func FlushAll() error {
	return Client.FlushAll(ctx).Err()
}

// Получить все ключи из Redis
func GetAllKeys() ([]string, error) {
	return Client.Keys(ctx, "*").Result()
}

// Получить всех пользователей из Redis
func GetAllUsers() (map[int64]*UserData, error) {
	keys, err := Client.Keys(ctx, "user:*").Result()
	if err != nil {
		return nil, err
	}

	users := make(map[int64]*UserData)
	for _, key := range keys {
		userIDStr := strings.TrimPrefix(key, "user:")
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			continue
		}

		userData, err := GetUser(userID)
		if err != nil {
			continue
		}

		users[userID] = userData
	}

	return users, nil
}

// Получить информацию о базе данных Redis
func GetDatabaseInfo() (map[string]string, error) {
	info, err := Client.Info(ctx).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	keys, _ := GetAllKeys()
	result["total_keys"] = fmt.Sprintf("%d", len(keys))
	result["redis_info"] = info

	return result, nil
}

// Удалить истекшие ключи
func CleanupExpiredKeys() error {
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
		if ttl < 0 {
			Client.Del(ctx, key)
			expiredCount++
		}
	}

	log.Printf("Cleanup completed: removed %d expired keys", expiredCount)
	return nil
}

// Получить статистику использования памяти Redis
func GetMemoryStats() (map[string]any, error) {
	info, err := Client.Info(ctx, "memory").Result()
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

	userKeys, _ := Client.Keys(ctx, "user:*").Result()
	quizKeys, _ := Client.Keys(ctx, "quiz_*").Result()
	stats["user_keys_count"] = len(userKeys)
	stats["quiz_keys_count"] = len(quizKeys)

	return stats, nil
}

// Сохранить сегодняшнее время квиза в Redis
func SaveQuizTime(quizTime time.Time) error {
	// Используем московское время (UTC+3)
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	now := time.Now().In(moscowTZ)
	today := now.Format("2006-01-02")
	key := fmt.Sprintf("quiz_time:%s", today)

	timeStr := quizTime.Format(time.RFC3339)

	endOfDay := time.Date(now.Year(), now.Month(), now.Day()+1, 1, 0, 0, 0, moscowTZ)
	ttl := endOfDay.Sub(now)

	return Client.Set(ctx, key, timeStr, ttl).Err()
}

// Получить сегодняшнее время квиза из Redis
func LoadQuizTime() (time.Time, error) {
	// Используем московское время (UTC+3)
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ).Format("2006-01-02")
	key := fmt.Sprintf("quiz_time:%s", today)

	val, err := Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return time.Time{}, fmt.Errorf("quiz time for today not found")
	}
	if err != nil {
		return time.Time{}, err
	}

	quizTime, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse quiz time: %v", err)
	}

	return quizTime, nil
}

// Установить флаг, что квиз сегодня уже был
func SetQuizAlreadyWas() error {
	// Используем московское время (UTC+3)
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	now := time.Now().In(moscowTZ)
	today := now.Format("2006-01-02")
	key := fmt.Sprintf("quiz_was:%s", today)

	endOfDay := time.Date(now.Year(), now.Month(), now.Day()+1, 1, 0, 0, 0, moscowTZ)
	ttl := endOfDay.Sub(now)

	return Client.Set(ctx, key, "true", ttl).Err()
}

// Проверить, был ли квиз сегодня проведен. Если был - true
func GetQuizAlreadyWas() (bool, error) {
	// Используем московское время (UTC+3)
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ).Format("2006-01-02")
	key := fmt.Sprintf("quiz_was:%s", today)

	val, err := Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return val == "true", nil
}

// Очистить флаг, что квиз сегодня уже был
func ClearQuizAlreadyWas() error {
	// Используем московское время (UTC+3)
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ).Format("2006-01-02")
	key := fmt.Sprintf("quiz_was:%s", today)

	return Client.Del(ctx, key).Err()
}

// Сохранить полные данные сегодняшнего квиза в Redis
func SaveQuizData(quote, songName string, quizTime time.Time) error {
	// Используем московское время (UTC+3)
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	now := time.Now().In(moscowTZ)
	today := now.Format("2006-01-02")
	key := fmt.Sprintf("quiz_data:%s", today)

	quizData := QuizData{
		Quote:    quote,
		SongName: songName,
		QuizTime: quizTime,
	}

	data, err := json.Marshal(quizData)
	if err != nil {
		return fmt.Errorf("failed to marshal quiz data: %v", err)
	}

	endOfDay := time.Date(now.Year(), now.Month(), now.Day()+1, 1, 0, 0, 0, moscowTZ)
	ttl := endOfDay.Sub(now)

	return Client.Set(ctx, key, data, ttl).Err()
}

// Загрузить полные данные сегодняшнего квиза из Redis
func LoadQuizData() (string, string, time.Time, error) {
	// Используем московское время (UTC+3)
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ).Format("2006-01-02")
	key := fmt.Sprintf("quiz_data:%s", today)

	val, err := Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", "", time.Time{}, fmt.Errorf("quiz data for today not found in Redis")
	}
	if err != nil {
		return "", "", time.Time{}, err
	}

	var quizData QuizData
	err = json.Unmarshal([]byte(val), &quizData)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to unmarshal quiz data: %v", err)
	}

	return quizData.Quote, quizData.SongName, quizData.QuizTime, nil
}

// Сбросить состояние IsWinner в false у всех пользователей в Redis
func ResetAllUsersWinnerStatus() error {
	log.Printf("Starting winner status reset for users in Redis...")

	// Получаем все ключи пользователей в Redis
	keys, err := Client.Keys(ctx, "user:*").Result()
	if err != nil {
		return fmt.Errorf("failed to get user keys: %v", err)
	}

	updatedCount := 0
	for _, key := range keys {
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			continue
		}

		userID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			log.Printf("Failed to parse userID from key %s: %v", key, err)
			continue
		}

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

		if userData.IsWinner {
			userData.IsWinner = false

			err = SetUser(userID, &userData)
			if err != nil {
				log.Printf("Failed to save updated user data for %d: %v", userID, err)
				continue
			}

			updatedCount++
			log.Printf("Reset winner status for user %d", userID)
		}
	}

	log.Printf("Winner status reset completed. Updated %d users out of %d total.", updatedCount, len(keys))
	return nil
}
