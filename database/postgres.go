package database

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type PostgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Инициализировать подключение к PostgreSQL
func InitPostgreSQL(host, user, password, dbname string, port int, sslmode string) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=Europe/Moscow",
		host, user, password, dbname, port, sslmode)

	var db *gorm.DB
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return db, fmt.Errorf("failed to connect to PostgreSQL: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return db, fmt.Errorf("failed to get SQL DB instance: %v", err)
	}

	err = sqlDB.Ping()
	if err != nil {
		return db, fmt.Errorf("failed to ping PostgreSQL: %v", err)
	}

	log.Println("Successfully connected to PostgreSQL")
	return db, nil
}

// Закрыть подключение к PostgreSQL
func ClosePostgreSQL(db *gorm.DB) error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// Выполнить автоматическую миграцию всех моделей
func AutoMigrate(db *gorm.DB) error {
	log.Println("Starting database migration...")

	err := db.AutoMigrate(
		&User{},
		&Quiz{},
		&Admin{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %v", err)
	}

	log.Println("Database migration completed successfully")
	return nil
}

// Получить статистику базы данных
func (p *PostgresRepository) GetDatabaseStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var userCount int64
	err := p.db.Model(&User{}).Count(&userCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %v", err)
	}
	stats["total_users"] = userCount

	var activeUserCount int64
	err = p.db.Model(&User{}).Where("status = ?", "active").Count(&activeUserCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count active users: %v", err)
	}
	stats["active_users"] = activeUserCount

	var adminCount int64
	err = p.db.Model(&User{}).Where("is_admin = ?", true).Count(&adminCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count admins: %v", err)
	}
	stats["admin_users"] = adminCount

	var quizCount int64
	err = p.db.Model(&Quiz{}).Count(&quizCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count quizzes: %v", err)
	}
	stats["total_quizzes"] = quizCount

	var activeQuizCount int64
	err = p.db.Model(&Quiz{}).Where("is_active = ?", true).Count(&activeQuizCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count active quizzes: %v", err)
	}
	stats["active_quizzes"] = activeQuizCount

	return stats, nil
}

// Проверить состояние подключения к базе данных
func (p *PostgresRepository) HealthCheck() error {
	if p.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	sqlDB, err := p.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB instance: %v", err)
	}

	err = sqlDB.Ping()
	if err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	return nil
}

// Настроить пул соединений
func (p *PostgresRepository) SetConnectionPool(maxIdleConns, maxOpenConns int) error {
	if p.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	sqlDB, err := p.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get SQL DB instance: %v", err)
	}

	sqlDB.SetMaxIdleConns(maxIdleConns)

	sqlDB.SetMaxOpenConns(maxOpenConns)

	log.Printf("Connection pool configured: MaxIdleConns=%d, MaxOpenConns=%d", maxIdleConns, maxOpenConns)
	return nil
}
