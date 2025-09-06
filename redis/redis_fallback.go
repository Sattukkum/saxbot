package redis

import (
	"encoding/json"
	"fmt"
	"log"
	"saxbot/environment"
	"slices"
)

// GetUserWithFallback получает данные пользователя из Redis, с fallback на PostgreSQL
func GetUserWithFallback(userID int64) (*UserData, error) {
	// Сначала ищем в Redis (оперативная память с TTL)
	key := fmt.Sprintf("user:%d", userID)
	val, err := Client.Get(ctx, key).Result()
	if err == nil {
		// Данные найдены в Redis
		var userData UserData
		err = json.Unmarshal([]byte(val), &userData)
		if err != nil {
			log.Printf("Failed to unmarshal user data from Redis: %v", err)
			// Не возвращаем ошибку, попробуем PostgreSQL
		} else {
			// Проверяем и обновляем админский статус
			if UpdateUserAdminStatus(userID, &userData) {
				// Если статус изменился, сохраняем обновленные данные в Redis
				SetUser(userID, &userData)
			}
			return &userData, nil
		}
	}

	// Если не найдены в Redis или произошла ошибка, пробуем PostgreSQL
	log.Printf("User %d not found in Redis, trying PostgreSQL fallback", userID)

	// Этот импорт будет добавлен позже, чтобы избежать циклических зависимостей
	// Пока создадим заглушку
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

		return userData, nil
	}

	// Пользователь не найден ни в Redis, ни в PostgreSQL - создаем нового
	log.Printf("Creating new user %d (not found in any storage)", userID)
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

	// Сохраняем нового пользователя в Redis
	SetUser(userID, userData)

	return userData, nil
}
