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
	log.Printf("Creating new user %d (not found in persistent storage)", userID)
	admins := environment.GetAdmins()
	if len(admins) == 0 {
		log.Printf("ADMINS environment variable is empty")
	}

	var userData *UserData
	if slices.Contains(admins, userID) {
		log.Printf("userID: %d is admin", userID)
		userData = &UserData{Username: "", IsAdmin: true, Warns: 0, Status: "active", IsWinner: false}
	} else {
		log.Printf("userID: %d is not admin", userID)
		userData = &UserData{Username: "", IsAdmin: false, Warns: 0, Status: "active", IsWinner: false}
	}

	// Сохраняем нового пользователя и в память, и на диск
	SetUser(userID, userData)
	SetUserPersistent(userID, userData)

	return userData, nil
}

// GetUserSafe получает данные пользователя без создания нового при ошибках Redis
// Возвращает (userData, isNewUser, error)
// isNewUser = true только если пользователь действительно не найден (redis.Nil)
// isNewUser = false если произошла ошибка Redis или пользователь существует
func GetUserSafe(userID int64) (*UserData, bool, error) {
	// Сначала ищем в оперативной памяти (с TTL)
	key := fmt.Sprintf("user:%d", userID)
	val, err := Client.Get(ctx, key).Result()
	if err == nil {
		// Данные найдены в памяти
		var userData UserData
		err = json.Unmarshal([]byte(val), &userData)
		if err != nil {
			return nil, false, fmt.Errorf("failed to unmarshal user data from memory: %v", err)
		}

		// Проверяем и обновляем админский статус
		if UpdateUserAdminStatus(userID, &userData) {
			// Если статус изменился, сохраняем обновленные данные
			SetUser(userID, &userData)
			SetUserPersistent(userID, &userData)
		}

		return &userData, false, nil
	}

	if err != redis.Nil {
		// Ошибка Redis - не создаем нового пользователя
		log.Printf("Redis error when getting user %d from memory: %v", userID, err)
		return nil, false, err
	}

	// Если не найдены в памяти, ищем на диске (персистентные данные)
	persistentKey := fmt.Sprintf("user_persistent:%d", userID)
	val, err = Client.Get(ctx, persistentKey).Result()
	if err == nil {
		// Данные найдены на диске, загружаем в память с TTL
		var userData UserData
		err = json.Unmarshal([]byte(val), &userData)
		if err != nil {
			return nil, false, fmt.Errorf("failed to unmarshal user data from disk: %v", err)
		}

		// Проверяем и обновляем админский статус
		statusChanged := UpdateUserAdminStatus(userID, &userData)

		// Сохраняем в память с TTL для быстрого доступа
		SetUser(userID, &userData)

		// Если статус изменился, также обновляем персистентные данные
		if statusChanged {
			SetUserPersistent(userID, &userData)
		}

		return &userData, false, nil
	}

	if err != redis.Nil {
		// Ошибка Redis - не создаем нового пользователя
		log.Printf("Redis error when getting user %d from persistent storage: %v", userID, err)
		return nil, false, err
	}

	// Пользователь действительно не найден ни в памяти, ни на диске
	log.Printf("User %d truly not found in any storage - can create new", userID)
	return nil, true, nil
}

// SetUser сохраняет данные пользователя в Redis с TTL 30 минут
func SetUser(userID int64, userData *UserData) error {
	// Проверяем целостность данных перед сохранением (но не блокируем для памяти)
	if err := validateUserDataIntegrity(userID, userData); err != nil {
		log.Printf("Warning: SetUser data integrity check failed for user %d: %v", userID, err)
		// Для памяти не блокируем, так как данные временные
	}

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
	// Проверяем целостность данных перед сохранением
	if err := validateUserDataIntegrity(userID, userData); err != nil {
		log.Printf("Blocking save for user %d: %v", userID, err)
		return err // Блокируем сохранение при подозрительных данных
	}

	key := fmt.Sprintf("user_persistent:%d", userID)
	data, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %v", err)
	}
	// Сохраняем без TTL для персистентности на диске
	return Client.Set(ctx, key, data, 0).Err()
}

// validateUserDataIntegrity проверяет подозрительные случаи обнуления данных
func validateUserDataIntegrity(userID int64, userData *UserData) error {
	// Проверяем существующие данные в персистентном хранилище
	existingKey := fmt.Sprintf("user_persistent:%d", userID)
	val, err := Client.Get(ctx, existingKey).Result()

	if err == nil {
		// Пользователь уже существует в персистентном хранилище
		var existingData UserData
		if json.Unmarshal([]byte(val), &existingData) == nil {
			// КРИТИЧЕСКАЯ ЗАЩИТА: запрещаем сохранение данных с меньшим количеством предупреждений
			if userData.Warns < existingData.Warns {
				return fmt.Errorf("prevented data corruption: cannot reduce warns from %d to %d (warns can only increase)", existingData.Warns, userData.Warns)
			}

			// Логируем сброс IsWinner (это нормально для квиза)
			if existingData.IsWinner && !userData.IsWinner {
				log.Printf("Info: Resetting IsWinner for user %d (normal for quiz reset)", userID)
			}
		}
	} else if err != redis.Nil {
		// Если есть ошибка доступа к Redis (не "ключ не найден"), логируем это
		log.Printf("Warning: Could not check existing data for user %d: %v", userID, err)
	}

	return nil
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
	memoryKeys, _ := Client.Keys(ctx, "user:*").Result()
	persistentKeys, _ := Client.Keys(ctx, "user_persistent:*").Result()
	stats["memory_keys_count"] = len(memoryKeys)
	stats["persistent_keys_count"] = len(persistentKeys)

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
		return "", "", time.Time{}, fmt.Errorf("quiz data for today not found")
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

// ResetAllUsersWinnerStatus сбрасывает состояние IsWinner в false у всех пользователей
func ResetAllUsersWinnerStatus() error {
	log.Printf("Starting winner status reset for all users...")

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

		// Проверяем, нужно ли обновлять IsWinner
		if userData.IsWinner {
			userData.IsWinner = false

			// Сохраняем обновленные данные в персистентное хранилище
			err = SetUserPersistent(userID, &userData)
			if err != nil {
				log.Printf("Failed to save updated persistent user data for %d: %v", userID, err)
				continue
			}

			// Также обновляем в памяти, если пользователь там есть
			memKey := fmt.Sprintf("user:%d", userID)
			if Client.Exists(ctx, memKey).Val() > 0 {
				SetUser(userID, &userData)
			}

			updatedCount++
			log.Printf("Reset winner status for user %d", userID)
		}
	}

	log.Printf("Winner status reset completed. Updated %d users out of %d total.", updatedCount, len(keys))
	return nil
}
