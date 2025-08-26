package redis

// UserData структура для хранения данных пользователя
type UserData struct {
	Username   string `json:"username"`
	IsAdmin    bool   `json:"is_admin"`
	Reputation int    `json:"reputation"`
	Warns      int    `json:"warns"`
	Status     string `json:"status"`
}
