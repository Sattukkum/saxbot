package database

import (
	"fmt"
	"log"
	"time"

	"saxbot/domain"
	"saxbot/environment"
	"slices"

	"gorm.io/gorm"
)

// Получить пользователя по Telegram UserID, создает нового если не найден
func (p *PostgresRepository) GetUser(userID int64) (domain.User, error) {
	var userPg UserPostgres
	err := p.db.Where("user_id = ?", userID).First(&userPg).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("Creating new user %d (not found in database)", userID)
			admins := environment.GetAdmins()

			newUser := UserPostgres{
				UserID:    userID,
				FirstName: "",
				Username:  "",
				IsAdmin:   slices.Contains(admins, userID),
				Warns:     0,
				Status:    "active",
				IsWinner:  false,
				AdminPref: "",
			}

			err = p.db.Create(&newUser).Error
			if err != nil {
				return domain.User{}, fmt.Errorf("failed to create user: %v", err)
			}

			log.Printf("Created new user %d, IsAdmin: %t", userID, newUser.IsAdmin)
			return userFromPostgresToDomain(&newUser), nil
		}
		log.Printf("Failed to get user %d, %v", userID, err)
		return domain.User{}, fmt.Errorf("failed to get user: %v", err)
	}
	user := userFromPostgresToDomain(&userPg)
	p.UpdateUserAdminStatus(&user)
	err = p.SaveUser(&user)
	if err != nil {
		log.Printf("Failed to update admin status is Postgres for user %d: %v", userID, err)
	}
	log.Printf("Got user %d from Postgres\nParams:\nUsername:%s\nWarns:%d", user.UserID, user.Username, user.Warns)
	return user, nil
}

// Получить пользователя по username
func (p *PostgresRepository) GetUserByUsername(username string) (*domain.User, error) {
	var userPg UserPostgres
	err := p.db.Where("username = ?", username).First(&userPg).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by username: %v", err)
	}

	user := userFromPostgresToDomain(&userPg)

	return &user, nil
}

// Получить всех пользователей из базы данных
func (p *PostgresRepository) GetAllUsers() ([]domain.User, error) {
	var users []UserPostgres
	err := p.db.Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %v", err)
	}
	var usersDomain []domain.User
	for _, user := range users {
		usersDomain = append(usersDomain, userFromPostgresToDomain(&user))
	}
	return usersDomain, nil
}

// Сохранить пользователя в базе данных
func (p *PostgresRepository) SaveUser(user *domain.User) error {
	userPg := userFromDomainToPostgres(user)
	return p.db.Save(&userPg).Error
}

// Обновить количество предупреждений пользователя
func (p *PostgresRepository) UpdateUserWarns(userID int64, delta int) error {
	user, err := p.GetUser(userID)
	if err != nil {
		return err
	}

	user.Warns += delta
	if user.Warns < 0 {
		user.Warns = 0
	}

	return p.SaveUser(&user)
}

// Обновить username пользователя
func (p *PostgresRepository) UpdateUsername(userID int64, username string) error {
	result := p.db.Model(&UserPostgres{}).Where("user_id = ?", userID).Update("username", username)
	if result.Error != nil {
		return fmt.Errorf("failed to update username: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user with id %d not found", userID)
	}
	return nil
}

// Обновить админский статус пользователя на основе переменной окружения ADMINS
func (p *PostgresRepository) UpdateUserAdminStatus(user *domain.User) bool {
	admins := environment.GetAdmins()
	if len(admins) == 0 {
		return false
	}

	newAdminStatus := slices.Contains(admins, user.UserID)
	if user.IsAdmin != newAdminStatus {
		log.Printf("Updating admin status for user %d: %t -> %t", user.UserID, user.IsAdmin, newAdminStatus)
		user.IsAdmin = newAdminStatus
		return true
	}
	return false
}

// Обновить админский статус для всех пользователей
func (p *PostgresRepository) RefreshAllUsersAdminStatus() error {
	log.Printf("Starting admin status refresh for all users...")

	var usersPg []UserPostgres
	users, err := p.GetAllUsers()
	if err != nil {
		return fmt.Errorf("failed to get users: %v", err)
	}

	updatedCount := 0
	for _, user := range users {
		p.UpdateUserAdminStatus(&user)
		err = p.SaveUser(&user)
		if err != nil {
			log.Printf("Failed to save updated user data for %d: %v", user.UserID, err)
			continue
		}
		updatedCount++
	}

	log.Printf("Admin status refresh completed. Updated %d users out of %d total.", updatedCount, len(usersPg))
	return nil
}

// Установить статус победителя для пользователя
func (p *PostgresRepository) SetUserWinnerStatus(userID int64, isWinner bool) error {
	result := p.db.Model(&UserPostgres{}).Where("user_id = ?", userID).Update("is_winner", isWinner)
	if result.Error != nil {
		return fmt.Errorf("failed to update winner status: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user with id %d not found", userID)
	}
	return nil
}

// Сбросить состояние IsWinner в false у всех пользователей
func (p *PostgresRepository) ResetAllUsersWinnerStatus() error {
	log.Printf("Starting winner status reset for all users...")

	result := p.db.Model(&UserPostgres{}).Where("is_winner = ?", true).Update("is_winner", false)
	if result.Error != nil {
		return fmt.Errorf("failed to reset winner status: %v", result.Error)
	}

	log.Printf("Winner status reset completed. Updated %d users.", result.RowsAffected)
	return nil
}

// Сохранить данные квиза на определенную дату
func (p *PostgresRepository) SaveQuizData(quote, songName string, quizTime time.Time) error {
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	quizTimeInMoscow := quizTime.In(moscowTZ)
	date := time.Date(quizTimeInMoscow.Year(), quizTimeInMoscow.Month(), quizTimeInMoscow.Day(), 0, 0, 0, 0, time.UTC)

	var existingQuiz QuizPostgres
	err := p.db.Where("date = ?", date).First(&existingQuiz).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing quiz: %v", err)
	}

	if err == gorm.ErrRecordNotFound {
		quiz := domain.Quiz{
			Date:     date,
			Quote:    quote,
			Song:     songName,
			QuizTime: quizTimeInMoscow,
			IsActive: true,
		}

		quizPg := quizFromDomainToPostgres(&quiz)
		err = p.db.Create(&quizPg).Error
		if err != nil {
			return fmt.Errorf("failed to create quiz: %v", err)
		}

		log.Printf("Created new quiz for date %s", date.Format("2006-01-02"))
	} else {
		existingQuiz.Quote = quote
		existingQuiz.SongName = songName
		existingQuiz.QuizTime = quizTimeInMoscow
		existingQuiz.IsActive = true

		err = p.db.Save(&existingQuiz).Error
		if err != nil {
			return fmt.Errorf("failed to update quiz: %v", err)
		}

		log.Printf("Updated quiz for date %s", date.Format("2006-01-02"))
	}

	return nil
}

// Получить данные квиза на сегодня
func (p *PostgresRepository) LoadQuizData() (string, string, time.Time, error) {
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ)
	date := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	var quizPg QuizPostgres
	err := p.db.Where("date = ? AND is_active = ?", date, true).First(&quizPg).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", "", time.Time{}, fmt.Errorf("quiz data for today not found")
		}
		return "", "", time.Time{}, fmt.Errorf("failed to load quiz data: %v", err)
	}

	return quizPg.Quote, quizPg.SongName, quizPg.QuizTime, nil
}

// Получить квиз по дате
func (p *PostgresRepository) GetQuizByDate(date time.Time) (*domain.Quiz, error) {
	normalizedDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	var quizPg QuizPostgres
	err := p.db.Where("date = ?", normalizedDate).First(&quizPg).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get quiz by date: %v", err)
	}

	quiz := quizFromPostgresToDomain(&quizPg)

	return &quiz, nil
}

// Получить все активные квизы
func (p *PostgresRepository) GetActiveQuizzes() ([]domain.Quiz, error) {
	var quizzesPg []QuizPostgres
	err := p.db.Where("is_active = ?", true).Order("date DESC").Find(&quizzesPg).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get active quizzes: %v", err)
	}
	var quizzes []domain.Quiz
	for _, quiz := range quizzesPg {
		quizzes = append(quizzes, quizFromPostgresToDomain(&quiz))
	}
	return quizzes, nil
}

// Завершить квиз
func (p *PostgresRepository) DeactivateQuiz(quizID uint) error {
	result := p.db.Model(&QuizPostgres{}).Where("id = ?", quizID).Update("is_active", false)
	if result.Error != nil {
		return fmt.Errorf("failed to deactivate quiz: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("quiz with id %d not found", quizID)
	}
	return nil
}

// Проверить, был ли квиз сегодня уже проведен
func (p *PostgresRepository) GetQuizAlreadyWas() (bool, error) {
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ)
	date := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	var quizPg QuizPostgres
	err := p.db.Where("date = ?", date).First(&quizPg).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check quiz status: %v", err)
	}

	return !quizPg.IsActive, nil
}

// Пометить квиз как завершенный
func (p *PostgresRepository) SetQuizAlreadyWas() error {
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ)
	date := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	log.Printf("Attempting to mark quiz as completed for date: %s", date.Format("2006-01-02"))

	result := p.db.Model(&QuizPostgres{}).Where("date = ? AND is_active = ?", date, true).Update("is_active", false)
	if result.Error != nil {
		log.Printf("Failed to mark quiz as completed: %v", result.Error)
		return fmt.Errorf("failed to mark quiz as completed: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		log.Printf("No active quiz found for today (%s) to mark as completed", date.Format("2006-01-02"))

		var quizPg QuizPostgres
		err := p.db.Where("date = ?", date).First(&quizPg).Error
		if err == gorm.ErrRecordNotFound {
			log.Printf("No quiz exists for today - this might be normal if quiz wasn't created yet")
		} else if err != nil {
			log.Printf("Error checking quiz existence: %v", err)
		} else {
			log.Printf("Quiz exists but is already inactive (is_active = %t)", quizPg.IsActive)
		}
	} else {
		log.Printf("Successfully marked today's quiz as completed (affected %d rows)", result.RowsAffected)
	}

	return nil
}

// Принудительно завершить квиз
func (p *PostgresRepository) ForceCompleteQuiz() error {
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ)
	date := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	log.Printf("Force completing quiz for date: %s", date.Format("2006-01-02"))

	result := p.db.Model(&QuizPostgres{}).Where("date = ?", date).Update("is_active", false)
	if result.Error != nil {
		log.Printf("Failed to force complete quiz: %v", result.Error)
		return fmt.Errorf("failed to force complete quiz: %v", result.Error)
	}

	log.Printf("Force completed quiz for today (affected %d rows)", result.RowsAffected)
	return nil
}
