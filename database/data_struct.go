package database

import (
	"time"

	"gorm.io/gorm"
)

// User представляет пользователя бота в базе данных PostgreSQL
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    int64          `gorm:"uniqueIndex;not null" json:"user_id"` // Telegram User ID
	Username  string         `gorm:"size:255" json:"username"`
	IsAdmin   bool           `gorm:"default:false" json:"is_admin"`
	Warns     int            `gorm:"default:0" json:"warns"`
	Status    string         `gorm:"size:50;default:'active'" json:"status"`
	IsWinner  bool           `gorm:"default:false" json:"is_winner"`
	AdminPref string         `gorm:"size:255" json:"admin_pref"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// Quiz представляет данные квиза в базе данных PostgreSQL
type Quiz struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Date      time.Time      `gorm:"uniqueIndex;not null" json:"date"` // Дата квиза (без времени)
	Quote     string         `gorm:"type:text;not null" json:"quote"`
	SongName  string         `gorm:"size:500;not null" json:"song_name"`
	QuizTime  time.Time      `gorm:"not null" json:"quiz_time"` // Время проведения квиза
	IsActive  bool           `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName возвращает имя таблицы для User
func (User) TableName() string {
	return "users"
}

// TableName возвращает имя таблицы для Quiz
func (Quiz) TableName() string {
	return "quizzes"
}

// BeforeCreate вызывается перед созданием пользователя
func (u *User) BeforeCreate(tx *gorm.DB) error {
	// Устанавливаем значения по умолчанию, если они не заданы
	if u.Status == "" {
		u.Status = "active"
	}
	return nil
}

// BeforeCreate вызывается перед созданием квиза
func (q *Quiz) BeforeCreate(tx *gorm.DB) error {
	// Нормализуем дату (убираем время, оставляем только дату)
	if !q.Date.IsZero() {
		q.Date = time.Date(q.Date.Year(), q.Date.Month(), q.Date.Day(), 0, 0, 0, 0, time.UTC)
	}
	return nil
}

// IsAdminUser проверяет, является ли пользователь администратором
func (u *User) IsAdminUser() bool {
	return u.IsAdmin
}

// HasMaxWarns проверяет, достиг ли пользователь максимального количества предупреждений
func (u *User) HasMaxWarns(maxWarns int) bool {
	return u.Warns >= maxWarns
}

// IsActiveUser проверяет, активен ли пользователь
func (u *User) IsActiveUser() bool {
	return u.Status == "active"
}

// IsBanned проверяет, заблокирован ли пользователь
func (u *User) IsBanned() bool {
	return u.Status == "banned"
}
