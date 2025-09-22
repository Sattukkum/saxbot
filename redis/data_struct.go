package redis

// Структура для хранения данных пользователя
type UserRedis struct {
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
	IsAdmin   bool   `json:"is_admin"`
	Warns     int    `json:"warns"`
	Status    string `json:"status"`
	IsWinner  bool   `json:"is_winner"`
	AdminPref string `json:"admin_pref"`
}
