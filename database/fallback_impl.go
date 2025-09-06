package database

import (
	redisClient "saxbot/redis"
)

// PostgreSQLFallbackImpl реализует интерфейс PostgreSQLFallback
type PostgreSQLFallbackImpl struct{}

// GetUser получает пользователя из PostgreSQL для Redis fallback
func (p *PostgreSQLFallbackImpl) GetUser(userID int64) (*redisClient.PostgreSQLUser, error) {
	// Используем существующую функцию GetUser из database пакета
	user, err := GetUser(userID)
	if err != nil {
		return nil, err
	}

	// Конвертируем в структуру, ожидаемую Redis
	return &redisClient.PostgreSQLUser{
		Username:  user.Username,
		IsAdmin:   user.IsAdmin,
		Warns:     user.Warns,
		Status:    user.Status,
		IsWinner:  user.IsWinner,
		AdminPref: user.AdminPref,
	}, nil
}

// GetQuizData получает данные квиза из PostgreSQL для Redis fallback
func (p *PostgreSQLFallbackImpl) GetQuizData() (string, string, error) {
	quote, songName, _, err := LoadQuizData()
	return quote, songName, err
}

// InitRedisFallback инициализирует fallback интерфейс для Redis
func InitRedisFallback() {
	fallback := &PostgreSQLFallbackImpl{}
	redisClient.SetPostgreSQLFallback(fallback)
}
