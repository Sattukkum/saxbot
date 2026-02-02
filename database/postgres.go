package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var MoscowTZ = time.FixedZone("Moscow", 3*60*60)

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
		Logger:                                   logger.Default.LogMode(logger.Info),
		DisableForeignKeyConstraintWhenMigrating: true, // не создавать FK при миграции — в quizzes могут быть winner_id, которых нет в users (0 или удалённые)
	})
	if err != nil {
		return db, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return db, fmt.Errorf("failed to get SQL DB instance: %w", err)
	}

	err = sqlDB.Ping()
	if err != nil {
		return db, fmt.Errorf("failed to ping PostgreSQL: %w", err)
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

	// Удаляем FK на winner_id, если есть — в quizzes могут быть winner_id=0 или удалённые пользователи, из-за чего миграция падает
	if err := db.Exec("ALTER TABLE quizzes DROP CONSTRAINT IF EXISTS fk_quizzes_winner").Error; err != nil {
		return fmt.Errorf("failed to drop quizzes winner FK (if any): %w", err)
	}

	err := db.AutoMigrate(
		&User{},
		&Channel{},
		&Quiz{},
		&Admin{},
		&Audio{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// GORM AutoMigrate в существующей БД иногда не создаёт новые таблицы — создаём audios явно
	if !db.Migrator().HasTable(&Audio{}) {
		log.Println("Creating audios table explicitly (was missing)...")
		if err := db.Migrator().CreateTable(&Audio{}); err != nil {
			return fmt.Errorf("failed to create audios table: %w", err)
		}
		log.Println("audios table created")
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
		return nil, fmt.Errorf("failed to count users: %w", err)
	}
	stats["total_users"] = userCount

	var activeUserCount int64
	err = p.db.Model(&User{}).Where("status = ?", "active").Count(&activeUserCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count active users: %w", err)
	}
	stats["active_users"] = activeUserCount

	var adminCount int64
	err = p.db.Model(&User{}).Where("is_admin = ?", true).Count(&adminCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count admins: %w", err)
	}
	stats["admin_users"] = adminCount

	var quizCount int64
	err = p.db.Model(&Quiz{}).Count(&quizCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count quizzes: %w", err)
	}
	stats["total_quizzes"] = quizCount

	var activeQuizCount int64
	err = p.db.Model(&Quiz{}).Where("is_active = ?", true).Count(&activeQuizCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count active quizzes: %w", err)
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
		return fmt.Errorf("failed to get SQL DB instance: %w", err)
	}

	err = sqlDB.Ping()
	if err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
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
		return fmt.Errorf("failed to get SQL DB instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(maxIdleConns)

	sqlDB.SetMaxOpenConns(maxOpenConns)

	log.Printf("Connection pool configured: MaxIdleConns=%d, MaxOpenConns=%d", maxIdleConns, maxOpenConns)
	return nil
}
