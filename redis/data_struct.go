package redis

// UserData структура для хранения данных пользователя
type UserData struct {
	Username  string `json:"username"`
	IsAdmin   bool   `json:"is_admin"`
	Warns     int    `json:"warns"`
	Status    string `json:"status"`
	IsWinner  bool   `json:"is_winner"`
	AdminPref string `json:"admin_pref"`
}
