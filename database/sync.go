package database

import (
	"context"
	"fmt"
	"log"
	"time"

	redisClient "saxbot/redis"
)

// ========== SYNC FUNCTIONS ==========
// Функции для синхронизации данных между Redis и PostgreSQL

// SyncUserToPostgres синхронизирует данные пользователя из Redis в PostgreSQL (асинхронно)
func SyncUserToPostgres(userID int64) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in SyncUserToPostgres for user %d: %v", userID, r)
			}
		}()

		// Получаем данные пользователя из Redis
		redisUserData, isNewUser, err := redisClient.GetUserSafe(userID)
		if err != nil {
			log.Printf("Failed to get user %d from Redis for sync: %v", userID, err)
			return
		}

		if redisUserData == nil || isNewUser {
			log.Printf("User %d not found in Redis, skipping sync", userID)
			return
		}

		// Получаем или создаем пользователя в PostgreSQL
		pgUser, err := GetUser(userID)
		if err != nil {
			log.Printf("Failed to get/create user %d in PostgreSQL: %v", userID, err)
			return
		}

		// Обновляем данные в PostgreSQL
		pgUser.Username = redisUserData.Username
		pgUser.IsAdmin = redisUserData.IsAdmin
		pgUser.Warns = redisUserData.Warns
		pgUser.Status = redisUserData.Status
		pgUser.IsWinner = redisUserData.IsWinner
		pgUser.AdminPref = redisUserData.AdminPref

		err = SaveUser(pgUser)
		if err != nil {
			log.Printf("Failed to sync user %d to PostgreSQL: %v", userID, err)
			return
		}

		log.Printf("Successfully synced user %d to PostgreSQL", userID)
	}()
}

// SyncQuizToPostgres синхронизирует данные квиза из Redis в PostgreSQL (асинхронно)
func SyncQuizToPostgres(quote, songName string, quizTime time.Time) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in SyncQuizToPostgres: %v", r)
			}
		}()

		err := SaveQuizData(quote, songName, quizTime)
		if err != nil {
			log.Printf("Failed to sync quiz data to PostgreSQL: %v", err)
			return
		}

		log.Printf("Successfully synced quiz data to PostgreSQL for date %s", quizTime.Format("2006-01-02"))
	}()
}

// ========== WRAPPER FUNCTIONS ==========
// Обертки для функций Redis, которые сохраняют в кэш (Redis) и основное хранилище (PostgreSQL)

// SetUserWithSync сохраняет пользователя в Redis и синхронизирует с PostgreSQL
func SetUserWithSync(userID int64, userData *redisClient.UserData) error {
	// Сначала сохраняем в Redis (кэш с TTL)
	err := redisClient.SetUser(userID, userData)
	if err != nil {
		return err
	}

	// Асинхронно синхронизируем с PostgreSQL (основное хранилище)
	SyncUserToPostgres(userID)

	return nil
}

// SetUserPersistentWithSync сохраняет пользователя в Redis и синхронизирует с PostgreSQL
func SetUserPersistentWithSync(userID int64, userData *redisClient.UserData) error {
	// Сначала сохраняем в Redis (теперь только кэш с TTL)
	err := redisClient.SetUser(userID, userData)
	if err != nil {
		return err
	}

	// Асинхронно синхронизируем с PostgreSQL (основное хранилище)
	SyncUserToPostgres(userID)

	return nil
}

// UpdateUserWarnsWithSync обновляет предупреждения в Redis и синхронизирует с PostgreSQL
func UpdateUserWarnsWithSync(userID int64, delta int) error {
	// Сначала обновляем в Redis (кэш с TTL)
	err := redisClient.UpdateUserWarns(userID, delta)
	if err != nil {
		return err
	}

	// Асинхронно синхронизируем с PostgreSQL (основное хранилище)
	SyncUserToPostgres(userID)

	return nil
}

// SaveQuizDataWithSync сохраняет квиз в Redis и синхронизирует с PostgreSQL
func SaveQuizDataWithSync(quote, songName string, quizTime time.Time) error {
	// Сначала сохраняем в Redis (кэш с TTL)
	err := redisClient.SaveQuizData(quote, songName, quizTime)
	if err != nil {
		return err
	}

	// Асинхронно синхронизируем с PostgreSQL (основное хранилище)
	SyncQuizToPostgres(quote, songName, quizTime)

	return nil
}

// ResetAllUsersWinnerStatusWithSync сбрасывает статус победителя в Redis и синхронизирует с PostgreSQL
func ResetAllUsersWinnerStatusWithSync() error {
	// Сначала сбрасываем в Redis (кэш с TTL)
	err := redisClient.ResetAllUsersWinnerStatus()
	if err != nil {
		return err
	}

	// Асинхронно синхронизируем с PostgreSQL (основное хранилище)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in ResetAllUsersWinnerStatusWithSync: %v", r)
			}
		}()

		err := ResetAllUsersWinnerStatus()
		if err != nil {
			log.Printf("Failed to sync winner status reset to PostgreSQL: %v", err)
			return
		}

		log.Printf("Successfully synced winner status reset to PostgreSQL")
	}()

	return nil
}

// RefreshAllUsersAdminStatusWithSync обновляет админский статус в Redis и синхронизирует с PostgreSQL
func RefreshAllUsersAdminStatusWithSync() error {
	// Сначала обновляем в Redis (кэш с TTL)
	err := redisClient.RefreshAllUsersAdminStatus()
	if err != nil {
		return err
	}

	// Асинхронно синхронизируем с PostgreSQL (основное хранилище)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in RefreshAllUsersAdminStatusWithSync: %v", r)
			}
		}()

		err := RefreshAllUsersAdminStatus()
		if err != nil {
			log.Printf("Failed to sync admin status refresh to PostgreSQL: %v", err)
			return
		}

		log.Printf("Successfully synced admin status refresh to PostgreSQL")
	}()

	return nil
}

// ========== BATCH SYNC FUNCTIONS ==========
// Функции для массовой синхронизации данных

// SyncAllUsersFromRedis синхронизирует всех пользователей из Redis в PostgreSQL
func SyncAllUsersFromRedis() error {
	log.Printf("Starting full user sync from Redis to PostgreSQL...")

	// Получаем всех пользователей из Redis
	redisUsers, err := redisClient.GetAllUsers()
	if err != nil {
		return err
	}

	syncCount := 0
	for userID := range redisUsers {
		// Синхронизируем каждого пользователя асинхронно
		go func(uid int64) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Panic in batch sync for user %d: %v", uid, r)
				}
			}()

			SyncUserToPostgres(uid)
		}(userID)
		syncCount++

		// Небольшая задержка чтобы не перегружать систему
		if syncCount%10 == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	log.Printf("Initiated sync for %d users from Redis to PostgreSQL", syncCount)
	return nil
}

// VerifyDataConsistency проверяет консистентность данных между Redis и PostgreSQL
func VerifyDataConsistency() error {
	log.Printf("Starting data consistency check...")

	// Получаем статистику из обеих баз
	redisStats, err := redisClient.GetDatabaseInfo()
	if err != nil {
		return err
	}

	pgStats, err := GetDatabaseStats()
	if err != nil {
		return err
	}

	log.Printf("Redis stats: %+v", redisStats)
	log.Printf("PostgreSQL stats: %+v", pgStats)

	log.Printf("Data consistency check completed")
	return nil
}

// ========== HELPER FUNCTIONS ==========

// InitSyncService инициализирует сервис синхронизации
func InitSyncService() {
	log.Printf("Initializing database sync service...")

	// Запускаем периодическую синхронизацию данных
	go func() {
		ticker := time.NewTicker(30 * time.Minute) // Каждые 30 минут
		defer ticker.Stop()

		for range ticker.C {
			log.Printf("Running periodic data consistency check...")
			if err := VerifyDataConsistency(); err != nil {
				log.Printf("Error during consistency check: %v", err)
			}
		}
	}()

	log.Printf("Database sync service initialized")
}

// SyncUserWithRetry синхронизирует пользователя с повторными попытками
func SyncUserWithRetry(userID int64, maxRetries int) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in SyncUserWithRetry for user %d: %v", userID, r)
			}
		}()

		for attempt := 1; attempt <= maxRetries; attempt++ {
			// Получаем данные пользователя из Redis
			redisUserData, isNewUser, err := redisClient.GetUserSafe(userID)
			if err != nil {
				log.Printf("Attempt %d: Failed to get user %d from Redis: %v", attempt, userID, err)
				if attempt < maxRetries {
					time.Sleep(time.Duration(attempt) * time.Second) // Экспоненциальная задержка
					continue
				}
				return
			}

			if redisUserData == nil || isNewUser {
				log.Printf("User %d not found in Redis, stopping retry attempts", userID)
				return
			}

			// Получаем или создаем пользователя в PostgreSQL
			pgUser, err := GetUser(userID)
			if err != nil {
				log.Printf("Attempt %d: Failed to get/create user %d in PostgreSQL: %v", attempt, userID, err)
				if attempt < maxRetries {
					time.Sleep(time.Duration(attempt) * time.Second)
					continue
				}
				return
			}

			// Обновляем данные в PostgreSQL
			pgUser.Username = redisUserData.Username
			pgUser.IsAdmin = redisUserData.IsAdmin
			pgUser.Warns = redisUserData.Warns
			pgUser.Status = redisUserData.Status
			pgUser.IsWinner = redisUserData.IsWinner
			pgUser.AdminPref = redisUserData.AdminPref

			err = SaveUser(pgUser)
			if err != nil {
				log.Printf("Attempt %d: Failed to sync user %d to PostgreSQL: %v", attempt, userID, err)
				if attempt < maxRetries {
					time.Sleep(time.Duration(attempt) * time.Second)
					continue
				}
				return
			}

			log.Printf("Successfully synced user %d to PostgreSQL on attempt %d", userID, attempt)
			return
		}
	}()
}

// GetSyncStatus возвращает статус синхронизации
func GetSyncStatus() (map[string]interface{}, error) {
	status := make(map[string]interface{})

	// Получаем статистику из Redis
	redisStats, err := redisClient.GetDatabaseInfo()
	if err != nil {
		status["redis_error"] = err.Error()
	} else {
		status["redis_stats"] = redisStats
	}

	// Получаем статистику из PostgreSQL
	pgStats, err := GetDatabaseStats()
	if err != nil {
		status["postgres_error"] = err.Error()
	} else {
		status["postgres_stats"] = pgStats
	}

	status["timestamp"] = time.Now().Format(time.RFC3339)

	return status, nil
}

// CleanupSyncService очищает ресурсы сервиса синхронизации
func CleanupSyncService() {
	log.Printf("Cleaning up database sync service...")
	// Здесь можно добавить логику очистки, если потребуется
	log.Printf("Database sync service cleanup completed")
}

// GetContext возвращает контекст Redis для использования в миграции
func GetContext() context.Context {
	return context.Background()
}

// GetAllUsersWithFallback получает всех пользователей с fallback на PostgreSQL
func GetAllUsersWithFallback() (map[int64]*redisClient.UserData, error) {
	// Сначала пробуем Redis
	redisUsers, err := redisClient.GetAllUsers()
	if err == nil && len(redisUsers) > 0 {
		return redisUsers, nil
	}

	log.Printf("Failed to get users from Redis or Redis is empty: %v, trying PostgreSQL fallback", err)

	// Fallback на PostgreSQL
	pgUsers, err := GetAllUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to get users from PostgreSQL: %v", err)
	}

	// Конвертируем в формат Redis
	result := make(map[int64]*redisClient.UserData)
	for _, pgUser := range pgUsers {
		result[pgUser.UserID] = &redisClient.UserData{
			Username:  pgUser.Username,
			IsAdmin:   pgUser.IsAdmin,
			Warns:     pgUser.Warns,
			Status:    pgUser.Status,
			IsWinner:  pgUser.IsWinner,
			AdminPref: pgUser.AdminPref,
		}
	}

	log.Printf("Loaded %d users from PostgreSQL fallback", len(result))
	return result, nil
}

// ========== QUIZ STATUS SYNC FUNCTIONS ==========

// GetQuizAlreadyWasWithFallback проверяет статус квиза с fallback на PostgreSQL
func GetQuizAlreadyWasWithFallback() (bool, error) {
	// Сначала проверяем в Redis
	wasQuiz, err := redisClient.GetQuizAlreadyWas()
	if err == nil {
		return wasQuiz, nil
	}

	log.Printf("Failed to get quiz status from Redis: %v, trying PostgreSQL fallback", err)

	// Fallback на PostgreSQL
	return GetQuizAlreadyWas()
}

// SetQuizAlreadyWasWithSync устанавливает флаг завершения квиза в Redis и синхронизирует с PostgreSQL
func SetQuizAlreadyWasWithSync() error {
	// Сначала устанавливаем в Redis
	err := redisClient.SetQuizAlreadyWas()
	if err != nil {
		log.Printf("Failed to set quiz completed flag in Redis: %v", err)
	}

	// Асинхронно синхронизируем с PostgreSQL
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in SetQuizAlreadyWasWithSync: %v", r)
			}
		}()

		err := SetQuizAlreadyWas()
		if err != nil {
			log.Printf("Failed to sync quiz completion to PostgreSQL: %v", err)
			return
		}

		log.Printf("Successfully synced quiz completion to PostgreSQL")
	}()

	return err
}
