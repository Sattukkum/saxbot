package database

import (
	"time"

	"gorm.io/gorm"
)

// User представляет пользователя бота в Postgres
type User struct {
	UserID       int64          `gorm:"primaryKey" json:"user_id"`
	FirstName    string         `gorm:"size:255" json:"first_name"`
	Username     string         `gorm:"size:255" json:"username"`
	Warns        int            `gorm:"default:0" json:"warns"`
	Status       string         `gorm:"size:50;default:'active'" json:"status"`
	MessageCount int            `gorm:"default:0" json:"message_count"`
	Birthday     time.Time      `gorm:"default:null" json:"birthday"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// Quiz представляет данные квиза в Postgres
type Quiz struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	Date       time.Time      `gorm:"uniqueIndex;not null" json:"date"` // Дата квиза (без времени)
	Quote      string         `gorm:"type:text" json:"quote"`
	SongName   string         `gorm:"size:500" json:"song_name"`
	QuizTime   time.Time      `gorm:"not null" json:"quiz_time"` // Время проведения квиза
	IsActive   bool           `gorm:"default:true" json:"is_active"`
	WinnerID   int64          `gorm:"default:0" json:"winner_id"`
	Winner     User           `gorm:"foreignKey:WinnerID;references:UserID" json:"winner,omitempty"`
	IsClip     bool           `gorm:"default:false" json:"is_clip"`
	ScreenPath string         `gorm:"size:500" json:"screen_path"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// Admin представляет админов в Postgres
type Admin struct {
	ID        int64  `gorm:"primaryKey" json:"id"`
	User      User   `gorm:"foreignKey:ID;references:UserID" json:"admin,omitempty"`
	AdminRole string `gorm:"size:500,default:'junior'" json:"admin_role"` // Два уровня - junior и senior. Джунам нельзя банить через бота
}

func (User) TableName() string {
	return "users"
}

func (Quiz) TableName() string {
	return "quizzes"
}

func (Admin) TableName() string {
	return "admins"
}
