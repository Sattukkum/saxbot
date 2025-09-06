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
func SetWithExpiration(key string, value any, expiration time.Duration) error {
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

// GetUser получает данные пользователя из Redis с fallback на PostgreSQL
func GetUser(userID int64) (*UserData, error) {
	// Используем функцию с fallback логикой
	return GetUserWithFallback(userID)
}

// GetUserSafe получает данные пользователя без создания нового
// Возвращает (userData, isNewUser, error)
// isNewUser = true только если пользователь действительно не найден
// isNewUser = false если произошла ошибка или пользователь существует
func GetUserSafe(userID int64) (*UserData, bool, error) {
	// Сначала ищем в Redis
	key := fmt.Sprintf("user:%d", userID)
	val, err := Client.Get(ctx, key).Result()
	if err == nil {
		// Данные найдены в Redis
		var userData UserData
		err = json.Unmarshal([]byte(val), &userData)
		if err != nil {
			return nil, false, fmt.Errorf("failed to unmarshal user data from Redis: %v", err)
		}

		// Проверяем и обновляем админский статус
		if UpdateUserAdminStatus(userID, &userData) {
			// Если статус изменился, сохраняем обновленные данные
			SetUser(userID, &userData)
		}

		return &userData, false, nil
	}

	if err != redis.Nil {
		// Ошибка Redis - не создаем нового пользователя
		log.Printf("Redis error when getting user %d: %v", userID, err)
		return nil, false, err
	}

	// Если не найдены в Redis, пробуем PostgreSQL
	pgUser, err := getFromPostgreSQL(userID)
	if err != nil {
		log.Printf("Failed to get user %d from PostgreSQL: %v", userID, err)
	} else if pgUser != nil {
		// Конвертируем из PostgreSQL структуры в Redis структуру
		userData := &UserData{
			Username:  pgUser.Username,
			IsAdmin:   pgUser.IsAdmin,
			Warns:     pgUser.Warns,
			Status:    pgUser.Status,
			IsWinner:  pgUser.IsWinner,
			AdminPref: pgUser.AdminPref,
		}

		// Сохраняем в Redis для быстрого доступа в будущем
		SetUser(userID, userData)

		return userData, false, nil
	}

	// Пользователь действительно не найден ни в Redis, ни в PostgreSQL
	log.Printf("User %d truly not found in any storage - can create new", userID)
	return nil, true, nil
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

	// Сохраняем только в память (с TTL)
	return SetUser(userID, userData)
}

// UpdateUserAdminStatus обновляет админский статус пользователя на основе переменной окружения ADMINS
func UpdateUserAdminStatus(userID int64, userData *UserData) bool {
	admins := environment.GetAdmins()
	if len(admins) == 0 {
		return false
	}

	newAdminStatus := slices.Contains(admins, userID)
	if userData.IsAdmin != newAdminStatus {
		log.Printf("Updating admin status for user %d: %t -> %t", userID, userData.IsAdmin, newAdminStatus)
		userData.IsAdmin = newAdminStatus
		return true // Статус изменился
	}
	return false // Статус не изменился
}

// RefreshAllUsersAdminStatus обновляет админский статус для всех пользователей в Redis
func RefreshAllUsersAdminStatus() error {
	log.Printf("Starting admin status refresh for users in Redis...")

	// Получаем все ключи пользователей в Redis
	keys, err := Client.Keys(ctx, "user:*").Result()
	if err != nil {
		return fmt.Errorf("failed to get user keys: %v", err)
	}

	updatedCount := 0
	for _, key := range keys {
		// Извлекаем userID из ключа (формат: user:123456)
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
			// Сохраняем обновленные данные в Redis
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

// FlushAll очищает всю базу данных Redis
func FlushAll() error {
	return Client.FlushAll(ctx).Err()
}

// GetAllKeys получает все ключи из базы данных
func GetAllKeys() ([]string, error) {
	return Client.Keys(ctx, "*").Result()
}

// GetAllUsers получает всех пользователей из базы данных
func GetAllUsers() (map[int64]*UserData, error) {
	keys, err := Client.Keys(ctx, "user:*").Result()
	if err != nil {
		return nil, err
	}

	users := make(map[int64]*UserData)
	for _, key := range keys {
		// Извлекаем userID из ключа "user:123"
		userIDStr := strings.TrimPrefix(key, "user:")
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			continue // Пропускаем некорректные ключи
		}

		userData, err := GetUser(userID)
		if err != nil {
			continue // Пропускаем пользователей с ошибками
		}

		users[userID] = userData
	}

	return users, nil
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

	// Добавляем количество ключей разных типов
	userKeys, _ := Client.Keys(ctx, "user:*").Result()
	quizKeys, _ := Client.Keys(ctx, "quiz_*").Result()
	stats["user_keys_count"] = len(userKeys)
	stats["quiz_keys_count"] = len(quizKeys)

	return stats, nil
}

// SaveQuizTime сохраняет время квиза на сегодня в Redis
func SaveQuizTime(quizTime time.Time) error {
	// Используем московское время (UTC+3)
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	now := time.Now().In(moscowTZ)
	today := now.Format("2006-01-02")
	key := fmt.Sprintf("quiz_time:%s", today)

	// Сохраняем время в формате RFC3339 для точности
	timeStr := quizTime.Format(time.RFC3339)

	// Устанавливаем TTL до конца дня + 1 час (чтобы не потерять данные на границе дней)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day()+1, 1, 0, 0, 0, moscowTZ)
	ttl := endOfDay.Sub(now)

	return Client.Set(ctx, key, timeStr, ttl).Err()
}

// LoadQuizTime загружает время квиза на сегодня из Redis
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

	// Парсим время из RFC3339 формата
	quizTime, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse quiz time: %v", err)
	}

	return quizTime, nil
}

// SetQuizAlreadyWas устанавливает флаг, что квиз сегодня уже был
func SetQuizAlreadyWas() error {
	// Используем московское время (UTC+3)
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	now := time.Now().In(moscowTZ)
	today := now.Format("2006-01-02")
	key := fmt.Sprintf("quiz_was:%s", today)

	// Устанавливаем TTL до конца дня + 1 час (чтобы не потерять данные на границе дней)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day()+1, 1, 0, 0, 0, moscowTZ)
	ttl := endOfDay.Sub(now)

	return Client.Set(ctx, key, "true", ttl).Err()
}

// GetQuizAlreadyWas проверяет, был ли квиз сегодня уже
func GetQuizAlreadyWas() (bool, error) {
	// Используем московское время (UTC+3)
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ).Format("2006-01-02")
	key := fmt.Sprintf("quiz_was:%s", today)

	val, err := Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil // Квиза сегодня еще не было
	}
	if err != nil {
		return false, err
	}

	return val == "true", nil
}

// ClearQuizAlreadyWas очищает флаг, что квиз сегодня уже был
func ClearQuizAlreadyWas() error {
	// Используем московское время (UTC+3)
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ).Format("2006-01-02")
	key := fmt.Sprintf("quiz_was:%s", today)

	return Client.Del(ctx, key).Err()
}

// QuizData структура для сохранения полных данных квиза
type QuizData struct {
	Quote    string    `json:"quote"`
	SongName string    `json:"song_name"`
	QuizTime time.Time `json:"quiz_time"`
}

// SaveQuizData сохраняет полные данные квиза на сегодня в Redis
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

	// Устанавливаем TTL до конца дня + 1 час
	endOfDay := time.Date(now.Year(), now.Month(), now.Day()+1, 1, 0, 0, 0, moscowTZ)
	ttl := endOfDay.Sub(now)

	return Client.Set(ctx, key, data, ttl).Err()
}

// LoadQuizData загружает полные данные квиза на сегодня из Redis
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

// ResetAllUsersWinnerStatus сбрасывает состояние IsWinner в false у всех пользователей в Redis
func ResetAllUsersWinnerStatus() error {
	log.Printf("Starting winner status reset for users in Redis...")

	// Получаем все ключи пользователей в Redis
	keys, err := Client.Keys(ctx, "user:*").Result()
	if err != nil {
		return fmt.Errorf("failed to get user keys: %v", err)
	}

	updatedCount := 0
	for _, key := range keys {
		// Извлекаем userID из ключа (формат: user:123456)
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

		// Проверяем, нужно ли обновлять IsWinner
		if userData.IsWinner {
			userData.IsWinner = false

			// Сохраняем обновленные данные в Redis
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
