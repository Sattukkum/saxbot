package redis

import "fmt"

// PostgreSQLFallback интерфейс для получения данных из PostgreSQL
type PostgreSQLFallback interface {
	GetUser(userID int64) (*PostgreSQLUser, error)
	GetQuizData() (string, string, error)
}

var postgresqlFallback PostgreSQLFallback

// SetPostgreSQLFallback устанавливает реализацию интерфейса для fallback
func SetPostgreSQLFallback(fallback PostgreSQLFallback) {
	postgresqlFallback = fallback
}

// getFromPostgreSQL получает пользователя из PostgreSQL через интерфейс
func getFromPostgreSQL(userID int64) (*PostgreSQLUser, error) {
	if postgresqlFallback == nil {
		return nil, fmt.Errorf("PostgreSQL fallback not configured")
	}
	return postgresqlFallback.GetUser(userID)
}

// PostgreSQLUser представляет пользователя из PostgreSQL
type PostgreSQLUser struct {
	Username  string
	IsAdmin   bool
	Warns     int
	Status    string
	IsWinner  bool
	AdminPref string
}
