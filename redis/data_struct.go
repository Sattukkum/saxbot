package redis

import "time"

// Структура для хранения данных пользователя
type UserData struct {
	Username  string `json:"username"`
	IsAdmin   bool   `json:"is_admin"`
	Warns     int    `json:"warns"`
	Status    string `json:"status"`
	IsWinner  bool   `json:"is_winner"`
	AdminPref string `json:"admin_pref"`
}

// Структура для хранения данных квиза
type QuizData struct {
	Quote    string    `json:"quote"`
	SongName string    `json:"song_name"`
	QuizTime time.Time `json:"quiz_time"`
}
