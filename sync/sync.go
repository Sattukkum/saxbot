package sync

import (
	"context"
	"log"

	"saxbot/database"
	"saxbot/domain"
	redisClient "saxbot/redis"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Функции для синхронизации данных между Redis и PostgreSQL

type UserInterface interface {
	GetUser(userID int64) (domain.User, error)
	SaveUser(user *domain.User) error
	GetAllUsers() ([]domain.User, error)
	UpdateUserWarns(userID int64, delta int) error
	RefreshAllUsersAdminStatus() error
	SetUserWinnerStatus(userID int64, isWinner bool) error
	ResetAllUsersWinnerStatus() error
}

type SyncService struct {
	redis    UserInterface
	postgres UserInterface
}

func NewSyncService(client *redis.Client, ctx context.Context, db *gorm.DB) (SyncService, *redisClient.RedisRepository, *database.PostgresRepository) {
	redis := redisClient.NewRedisRepository(client, ctx)
	postgres := database.NewPostgresRepository(db)
	return SyncService{
		redis:    redis,
		postgres: postgres,
	}, redis, postgres
}

// Получить пользователя из Redis или Postgres (или создать нового)
func (s *SyncService) GetUser(userID int64) (domain.User, error) {
	user, err := s.redis.GetUser(userID)
	if err != nil {
		log.Printf("Couldn't get user %d from redis, %v", userID, err)
	} else {
		return user, nil
	}
	user, err = s.postgres.GetUser(userID)
	if err != nil {
		log.Printf("Couldn't get user %d from postgres, %v", userID, err)
		return domain.User{}, err
	}
	err = s.redis.SaveUser(&user)
	if err != nil {
		log.Printf("Couldn't save new user %d to redis, %v", userID, err)
	}
	return user, nil
}

// Сохранить пользователя в Redis и Postgres
func (s *SyncService) SaveUser(user *domain.User) error {
	err := s.postgres.SaveUser(user)
	if err != nil {
		log.Printf("Couldn't save user to postgres, %v", err)
	}
	err = s.redis.SaveUser(user)
	if err != nil {
		log.Printf("Couldn't save user to redis, %v", err)
	}

	return err
}

// Получить всех пользователей из Postgres (если не получилось - из Redis)
func (s *SyncService) GetAllUsers() ([]domain.User, error) {
	users, err := s.postgres.GetAllUsers()
	if err != nil {
		log.Printf("Couldn't get all users from postgres, %v", err)
	} else {
		return users, nil
	}

	users, err = s.redis.GetAllUsers()
	if err != nil {
		log.Printf("Couldn't get all users from redis, %v", err)
		return []domain.User{}, err
	}
	return users, nil
}

// Обновить счетчик предупреждений в Redis и Postgres
func (s *SyncService) UpdateUserWarns(userID int64, delta int) error {
	err := s.redis.UpdateUserWarns(userID, delta)
	if err != nil {
		log.Printf("Couldn't update user %d warns in redis, %v", userID, err)
	}

	err = s.postgres.UpdateUserWarns(userID, delta)
	if err != nil {
		log.Printf("Couldn't update user %d warns in postgres, %v", userID, err)
	}

	return err
}

// Обновить админиские статусы в соответствие с переменной окружения ADMINS
func (s *SyncService) RefreshAllUsersAdminStatus() error {
	err := s.redis.RefreshAllUsersAdminStatus()
	if err != nil {
		log.Printf("Couldn't update users admin statuses in Redis, %v", err)
	}
	err = s.postgres.RefreshAllUsersAdminStatus()
	if err != nil {
		log.Printf("Couldn't update users admin statuses in Postgres, %v", err)
	}
	return err
}

// Установить статус для победителя квиза
func (s *SyncService) SetUserWinnerStatus(userID int64, isWinner bool) error {
	err := s.redis.SetUserWinnerStatus(userID, isWinner)
	if err != nil {
		log.Printf("Couldn't update winner status for user %d in Redis, %v", userID, err)
	}
	err = s.postgres.SetUserWinnerStatus(userID, isWinner)
	if err != nil {
		log.Printf("Couldn't update winner status for user %d in Postgres, %v", userID, err)
	}
	return err
}

// Сбросить статусы победителя у всех пользователей
func (s *SyncService) ResetAllUsersWinnerStatus() error {
	err := s.redis.ResetAllUsersWinnerStatus()
	if err != nil {
		log.Printf("Couldn't update users winner statuses in Redis, %v", err)
	}
	err = s.postgres.ResetAllUsersWinnerStatus()
	if err != nil {
		log.Printf("Couldn't update users winner statuses in Postgres, %v", err)
	}
	return err
}
