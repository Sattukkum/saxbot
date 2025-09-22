package redis

import (
	"encoding/json"
	"fmt"
	"log"
	"saxbot/domain"
	"saxbot/environment"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Получить данные пользователя
func (r *RedisRepository) GetUser(userID int64) (domain.User, error) {
	key := fmt.Sprintf("user:%d", userID)
	val, err := r.Client.Get(r.ctx, key).Result()
	if err != nil {
		return domain.User{}, err
	}
	var userDataR UserRedis
	err = json.Unmarshal([]byte(val), &userDataR)
	if err != nil {
		log.Printf("Failed to unmarshal user data from Redis: %v", err)
		return domain.User{}, err
	} else {
		user := userFromRedisToDomain(userID, userDataR)
		r.UpdateUserAdminStatus(&user)
		err = r.SaveUser(&user)
		if err != nil {
			log.Printf("Failed to update admin status in Redis for user %d: %v", userID, err)
		}
		log.Printf("Got user %d from Redis\nParams:\nUsername:%s\nWarns:%d", user.UserID, user.Username, user.Warns)
		return user, nil
	}
}

// Сохранить данные пользователя
func (r *RedisRepository) SaveUser(user *domain.User) error {
	key := fmt.Sprintf("user:%d", user.UserID)
	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %v", err)
	}
	return r.Client.Set(r.ctx, key, data, 30*time.Minute).Err()
}

// Получить всех пользователей из Redis
func (r *RedisRepository) GetAllUsers() ([]domain.User, error) {
	keys, err := r.Client.Keys(r.ctx, "user:*").Result()
	if err != nil {
		return nil, err
	}

	var users []domain.User
	for _, key := range keys {
		userIDStr := strings.TrimPrefix(key, "user:")
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			continue
		}

		userData, err := r.GetUser(userID)
		if err != nil {
			continue
		}

		users = append(users, userData)
	}

	return users, nil
}

// Обновить количество предупреждений пользователя
func (r *RedisRepository) UpdateUserWarns(userID int64, delta int) error {
	userData, err := r.GetUser(userID)
	if err != nil {
		return err
	}
	userData.Warns += delta
	if userData.Warns < 0 {
		userData.Warns = 0
	}

	return r.SaveUser(&userData)
}

// Обновить админский статус пользователя на основе переменной окружения ADMINS. При изменении статуса - true
func (r *RedisRepository) UpdateUserAdminStatus(user *domain.User) bool {
	admins := environment.GetAdmins()
	if len(admins) == 0 {
		return false
	}

	newAdminStatus := slices.Contains(admins, user.UserID)
	if user.IsAdmin != newAdminStatus {
		log.Printf("Updating admin status for user %d: %t -> %t", user.UserID, user.IsAdmin, newAdminStatus)
		user.IsAdmin = newAdminStatus
		return true
	}
	return false
}

// Обновить админский статус для всех пользователей в Redis
func (r *RedisRepository) RefreshAllUsersAdminStatus() error {
	log.Printf("Starting admin status refresh for users in Redis...")

	keys, err := r.Client.Keys(r.ctx, "user:*").Result()
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

		val, err := r.Client.Get(r.ctx, key).Result()
		if err != nil {
			log.Printf("Failed to get user data for %d: %v", userID, err)
			continue
		}

		var userDataR UserRedis
		err = json.Unmarshal([]byte(val), &userDataR)
		if err != nil {
			log.Printf("Failed to unmarshal user data for %d: %v", userID, err)
			continue
		}

		user := userFromRedisToDomain(userID, userDataR)

		r.UpdateUserAdminStatus(&user)
		err = r.SaveUser(&user)
		if err != nil {
			log.Printf("Failed to save updated user data for %d: %v", userID, err)
			continue
		}
		updatedCount++
	}

	log.Printf("Admin status refresh completed. Updated %d users out of %d total.", updatedCount, len(keys))
	return nil
}

// Сбросить состояние IsWinner в false у всех пользователей в Redis
func (r *RedisRepository) ResetAllUsersWinnerStatus() error {
	log.Printf("Starting winner status reset for users in Redis...")

	// Получаем все ключи пользователей в Redis
	keys, err := r.Client.Keys(r.ctx, "user:*").Result()
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

		val, err := r.Client.Get(r.ctx, key).Result()
		if err != nil {
			log.Printf("Failed to get user data for %d: %v", userID, err)
			continue
		}

		var userDataR UserRedis
		err = json.Unmarshal([]byte(val), &userDataR)
		if err != nil {
			log.Printf("Failed to unmarshal user data for %d: %v", userID, err)
			continue
		}

		if userDataR.IsWinner {
			userDataR.IsWinner = false
			user := userFromRedisToDomain(userID, userDataR)
			err = r.SaveUser(&user)
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

func (r *RedisRepository) SetUserWinnerStatus(userID int64, isWinner bool) error {
	user, err := r.GetUser(userID)
	if err != nil {
		log.Printf("Failed to get user %d", userID)
		return err
	}

	user.IsWinner = isWinner
	err = r.SaveUser(&user)
	if err != nil {
		log.Printf("Failed to update winner status for user %d", userID)
		return err
	}

	log.Printf("Winner status successfully updated for user %d", userID)
	return nil
}
