package domain

import (
	"time"
)

type User struct {
	UserID    int64
	FirstName string
	Username  string
	IsAdmin   bool
	Warns     int
	Status    string
	IsWinner  bool
	AdminPref string
}

type Quiz struct {
	Date     time.Time
	Song     string
	Quote    string
	QuizTime time.Time
	IsActive bool
}
