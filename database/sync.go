package database

import (
	"fmt"
	"log"
	"runtime"
	"slices"
	"sync"
	"time"

	"saxbot/environment"
	redisClient "saxbot/redis"
)

// Функции для синхронизации данных между Redis и PostgreSQL

// Синхронизация данных пользователя из Redis в PostgreSQL
func SyncUserToPostgres(userID int64) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in SyncUserToPostgres for user %d: %v", userID, r)
			}
		}()

		redisUserData, err := redisClient.GetUser(userID)
		if err != nil {
			log.Printf("Failed to get user %d from Redis for sync: %v", userID, err)
			return
		}

		if redisUserData == nil {
			log.Printf("User %d not found in Redis, skipping sync", userID)
			return
		}

		pgUser, err := GetUser(userID)
		if err != nil {
			log.Printf("Failed to get/create user %d in PostgreSQL: %v", userID, err)
			return
		}

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

// Синхронизация данных квиза из Redis в PostgreSQL
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

// Функции для синхронизации Redis и Postgres

// Сохранить пользователя в Redis и Postgres
func SetUserSync(userID int64, userData *redisClient.UserData) error {
	err := redisClient.SetUser(userID, userData)
	if err != nil {
		return err
	}

	SyncUserToPostgres(userID)

	return nil
}

// Обновить счетчик предупреждений в Redis и Postgres
func UpdateUserWarnsSync(userID int64, delta int) error {
	err := redisClient.UpdateUserWarns(userID, delta)
	if err != nil {
		return err
	}

	SyncUserToPostgres(userID)

	return nil
}

// Сохранить данные квиза в Redis и Postgres
func SaveQuizDataSync(quote, songName string, quizTime time.Time) error {
	err := redisClient.SaveQuizData(quote, songName, quizTime)
	if err != nil {
		return err
	}

	SyncQuizToPostgres(quote, songName, quizTime)

	return nil
}

// Сбросить статус победителя в Redis и Postgres
func ResetAllUsersWinnerStatusSync() error {
	err := redisClient.ResetAllUsersWinnerStatus()
	if err != nil {
		return err
	}

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

// Обновить данные об админах в Redis и Postgres
func RefreshAllAdminStatusSync() error {
	err := redisClient.RefreshAllUsersAdminStatus()
	if err != nil {
		return err
	}

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

// Функции для массовой синхронизации данных

// Синхронизация всех пользователей из Redis в PostgreSQL
func SyncAllUsersFromRedis() error {
	log.Printf("Starting full user sync from Redis to PostgreSQL...")

	redisUsers, err := redisClient.GetAllUsers()
	if err != nil {
		return err
	}

	var maxWorkers = runtime.GOMAXPROCS(0) * 2
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for userID := range redisUsers {
		wg.Add(1)

		sem <- struct{}{}
		go func(uid int64) {
			defer wg.Done()
			defer func() { <-sem }()

			defer func() {
				if r := recover(); r != nil {
					log.Printf("Panic in batch sync for user %d: %v", uid, r)
				}
			}()

			SyncUserToPostgres(uid)
		}(userID)
	}

	wg.Wait()
	log.Printf("Finished sync for %d users from Redis to PostgreSQL", len(redisUsers))
	return nil
}

func GetUserSync(userID int64) (userData *redisClient.UserData, err error) {
	userData, err = redisClient.GetUser(userID)
	if err == nil {
		return userData, nil
	}

	log.Printf("User %d not found in Redis, trying PostgreSQL fallback", userID)
	pgUser, err := GetUser(userID)
	if err != nil {
		log.Printf("Failed to get user %d from PostgreSQL: %v", userID, err)
	} else if pgUser != nil {
		userData = &redisClient.UserData{
			Username:  pgUser.Username,
			IsAdmin:   pgUser.IsAdmin,
			Warns:     pgUser.Warns,
			Status:    pgUser.Status,
			IsWinner:  pgUser.IsWinner,
			AdminPref: pgUser.AdminPref,
		}

		redisClient.SetUser(userID, userData)
		return userData, nil
	}

	log.Printf("Creating new user %d (not found in any storage)", userID)
	admins := environment.GetAdmins()
	if len(admins) == 0 {
		log.Printf("ADMINS environment variable is empty")
	}

	if slices.Contains(admins, userID) {
		log.Printf("userID: %d is admin", userID)
		userData = &redisClient.UserData{Username: "", IsAdmin: true, Warns: 0, Status: "active", IsWinner: false}
	} else {
		log.Printf("userID: %d is not admin", userID)
		userData = &redisClient.UserData{Username: "", IsAdmin: false, Warns: 0, Status: "active", IsWinner: false}
	}

	redisClient.SetUser(userID, userData)

	return userData, nil
}

// Получить всех пользователей из Redis и Postgres
func GetAllUsersSync() (map[int64]*redisClient.UserData, error) {
	redisUsers, err := redisClient.GetAllUsers()
	if err == nil && len(redisUsers) > 0 {
		return redisUsers, nil
	}

	log.Printf("Failed to get users from Redis or Redis is empty: %v, trying PostgreSQL fallback", err)

	pgUsers, err := GetAllUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to get users from PostgreSQL: %v", err)
	}

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

// Получить статус квиза из Redis и Postgres
func GetQuizAlreadyWasSync() (bool, error) {
	wasQuiz, err := redisClient.GetQuizAlreadyWas()
	if err == nil {
		return wasQuiz, nil
	}

	log.Printf("Failed to get quiz status from Redis: %v, trying PostgreSQL fallback", err)

	return GetQuizAlreadyWas()
}

// Установить флаг завершения квиза в Redis и Postgres
func SetQuizAlreadyWasSync() error {
	err := redisClient.SetQuizAlreadyWas()
	if err != nil {
		log.Printf("Failed to set quiz completed flag in Redis: %v", err)
	}

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
