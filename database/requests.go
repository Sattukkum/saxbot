package database

import (
	"errors"
	"fmt"
	"log"
	"time"

	"saxbot/environment"
	"slices"

	"gorm.io/gorm"
)

var MoscowTZ = time.FixedZone("Moscow", 3*60*60)

// Получить пользователя по Telegram UserID, создает нового если не найден
func (p *PostgresRepository) GetUser(userID int64) (User, error) {
	var user User
	err := p.db.Where("user_id = ?", userID).First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("Creating new user %d (not found in database)", userID)

			newUser := User{
				UserID:    userID,
				FirstName: "",
				Username:  "",
				Warns:     0,
				Status:    "active",
			}

			err = p.db.Create(&newUser).Error
			if err != nil {
				return User{}, fmt.Errorf("failed to create user: %v", err)
			}

			log.Printf("Created new user %d", userID)
			return newUser, nil
		}
		log.Printf("Failed to get user %d, %v", userID, err)
		return User{}, fmt.Errorf("failed to get user: %v", err)
	}
	log.Printf("Got user %d from Postgres\nParams:\nUsername:%s\nWarns:%d", user.UserID, user.Username, user.Warns)
	return user, nil
}

// Получить пользователя по username
func (p *PostgresRepository) GetUserByUsername(username string) (*User, error) {
	var user User
	err := p.db.Where("username = ?", username).First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by username: %v", err)
	}

	return &user, nil
}

// Получить всех пользователей из базы данных
func (p *PostgresRepository) GetAllUsers() ([]User, error) {
	var users []User
	err := p.db.Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %v", err)
	}

	return users, nil
}

// Сохранить пользователя в базе данных
func (p *PostgresRepository) SaveUser(user *User) error {
	return p.db.Save(user).Error
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
	result := p.db.Model(&User{}).Where("user_id = ?", userID).Update("username", username)
	if result.Error != nil {
		return fmt.Errorf("failed to update username: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user with id %d not found", userID)
	}
	return nil
}

// Обновить админский статус пользователя на основе переменной окружения ADMINS
func (p *PostgresRepository) IsUserAdmin(user *User) bool {
	admins := environment.GetAdmins()
	if len(admins) == 0 {
		return false
	}

	newAdminStatus := slices.Contains(admins, user.UserID)
	return newAdminStatus
}

// Обновить админский статус для всех пользователей
func (p *PostgresRepository) RefreshAllUsersAdminStatus() error {
	log.Printf("Starting admin status refresh for all users...")

	users, err := p.GetAllUsers()
	if err != nil {
		return fmt.Errorf("failed to get users: %v", err)
	}

	updatedCount := 0
	errCount := 0
	for _, user := range users {
		err = nil
		isAdmin := p.IsUserAdmin(&user)
		if isAdmin && !p.IsAdmin(user.UserID) {
			err = p.SaveAdmin(user, "junior")
			updatedCount++
		} else if !isAdmin && p.IsAdmin(user.UserID) {
			err = p.RemoveAdmin(user.UserID)
			updatedCount++
		}
		if err != nil {
			log.Printf("failed to update admin status for user %d: %v", user.UserID, err)
			errCount++
		}
	}

	log.Printf("Admin status refresh completed. Updated %d users out of %d total.", updatedCount, len(users))
	if errCount != 0 {
		return fmt.Errorf("got %d errors during update", errCount)
	}
	return nil
}

// Сохранить данные квиза на определенную дату
func (p *PostgresRepository) SaveQuizData(quote, songName, screenPath string, isClip bool, quizTime time.Time) error {
	quizTimeInMoscow := quizTime.In(MoscowTZ)
	date := time.Date(quizTimeInMoscow.Year(), quizTimeInMoscow.Month(), quizTimeInMoscow.Day(), 0, 0, 0, 0, time.UTC)

	var existingQuiz Quiz
	err := p.db.Where("date = ?", date).First(&existingQuiz).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing quiz: %v", err)
	}

	if err == gorm.ErrRecordNotFound {
		quiz := Quiz{
			Date:       date,
			Quote:      quote,
			SongName:   songName,
			QuizTime:   quizTimeInMoscow,
			IsActive:   true,
			IsClip:     isClip,
			ScreenPath: screenPath,
		}

		err = p.db.Create(&quiz).Error
		if err != nil {
			return fmt.Errorf("failed to create quiz: %v", err)
		}

		log.Printf("Created new quiz for date %s", date.Format("2006-01-02"))
	} else {
		existingQuiz.Quote = quote
		existingQuiz.SongName = songName
		existingQuiz.QuizTime = quizTimeInMoscow
		existingQuiz.IsActive = true
		existingQuiz.IsClip = isClip
		existingQuiz.ScreenPath = screenPath
		err = p.db.Save(&existingQuiz).Error
		if err != nil {
			return fmt.Errorf("failed to update quiz: %v", err)
		}

		log.Printf("Updated quiz for date %s", date.Format("2006-01-02"))
	}

	return nil
}

// Получить данные квиза на сегодня
func (p *PostgresRepository) LoadQuizData() (*Quiz, error) {
	today := time.Now().In(MoscowTZ)
	date := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	var quiz Quiz
	err := p.db.Preload("Winner").Where("date = ? AND is_active = ?", date, true).First(&quiz).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("quiz data for today not found: %v", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load quiz data: %v", err)
	}

	return &quiz, nil
}

// Получить квиз по дате
func (p *PostgresRepository) GetQuizByDate(date time.Time) (*Quiz, error) {
	normalized := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	var quiz Quiz
	err := p.db.Preload("Winner").Where("date = ?", normalized).First(&quiz).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get quiz by date: %v", err)
	}

	return &quiz, nil
}

// Получить все активные квизы
func (p *PostgresRepository) GetActiveQuizzes() ([]Quiz, error) {
	var quizzes []Quiz

	err := p.db.Where("is_active = ?", true).Order("date DESC").Find(&quizzes).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get active quizzes: %v", err)
	}

	return quizzes, nil
}

// Установить победителя и завершить квиз
func (p *PostgresRepository) SetQuizWinner(quizID uint, winnerID int64) error {
	result := p.db.Model(&Quiz{}).Where("id = ?", quizID).Updates(map[string]interface{}{
		"winner_id": winnerID,
		"is_active": false,
	})
	if result.Error != nil {
		return fmt.Errorf("failed to set winner for quiz %d: %v", quizID, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("quiz with id %d not found", quizID)
	}
	log.Printf("Winner %d successfully set for quiz %d", winnerID, quizID)
	return nil
}

// Получить последний завершенный квиз
func (p *PostgresRepository) GetLastCompletedQuiz() (*Quiz, error) {
	var quiz Quiz

	err := p.db.
		Preload("Winner").
		Where("is_active = ?", false).
		Order("date DESC").
		First(&quiz).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // можно вернуть nil, nil — если нет завершённых квизов
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get last completed quiz: %v", err)
	}

	return &quiz, nil
}

// Завершить квиз
func (p *PostgresRepository) DeactivateQuiz(quizID uint) error {
	result := p.db.Model(&Quiz{}).Where("id = ?", quizID).Update("is_active", false)
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
	today := time.Now().In(MoscowTZ)
	date := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	lastQuiz, err := p.GetLastCompletedQuiz()
	if err != nil {
		return false, fmt.Errorf("failed to check quiz status: %v", err)
	}
	if lastQuiz == nil {
		return false, nil
	}
	if date.After(lastQuiz.Date) {
		return false, nil
	}
	return true, nil
}

// Пометить квиз как завершенный
func (p *PostgresRepository) SetQuizAlreadyWas() error {
	today := time.Now().In(MoscowTZ)
	date := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	log.Printf("Attempting to mark quiz as completed for date: %s", date.Format("2006-01-02"))

	result := p.db.Model(&Quiz{}).Where("date = ? AND is_active = ?", date, true).Update("is_active", false)
	if result.Error != nil {
		log.Printf("Failed to mark quiz as completed: %v", result.Error)
		return fmt.Errorf("failed to mark quiz as completed: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		log.Printf("No active quiz found for today (%s) to mark as completed", date.Format("2006-01-02"))

		var quiz Quiz
		err := p.db.Where("date = ?", date).First(&quiz).Error
		if err == gorm.ErrRecordNotFound {
			log.Printf("No quiz exists for today - this might be normal if quiz wasn't created yet")
		} else if err != nil {
			log.Printf("Error checking quiz existence: %v", err)
		} else {
			log.Printf("Quiz exists but is already inactive (is_active = %t)", quiz.IsActive)
		}
	} else {
		log.Printf("Successfully marked today's quiz as completed (affected %d rows)", result.RowsAffected)
	}

	return nil
}

// Принудительно завершить квиз
func (p *PostgresRepository) ForceCompleteQuiz() error {
	today := time.Now().In(MoscowTZ)
	date := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	log.Printf("Force completing quiz for date: %s", date.Format("2006-01-02"))

	result := p.db.Model(&Quiz{}).Where("date = ?", date).Update("is_active", false)
	if result.Error != nil {
		log.Printf("Failed to force complete quiz: %v", result.Error)
		return fmt.Errorf("failed to force complete quiz: %v", result.Error)
	}

	log.Printf("Force completed quiz for today (affected %d rows)", result.RowsAffected)
	return nil
}

// Получить ID победителя последнего завершенного квиза
func (p *PostgresRepository) GetQuizWinnerID() (int64, error) {
	lastQuiz, err := p.GetLastCompletedQuiz()
	if err != nil {
		return 0, fmt.Errorf("failed to get last quiz winner id: %v", err)
	}
	if lastQuiz == nil {
		return 0, nil
	}
	return lastQuiz.WinnerID, nil
}

// Получить всех админов
func (p *PostgresRepository) GetAdmins() ([]Admin, error) {
	var admins []Admin
	err := p.db.Find(&admins).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get all admins: %v", err)
	}
	return admins, nil
}

// Сохранить админа
func (p *PostgresRepository) SaveAdmin(user User, adminRole string) error {
	admin := Admin{
		ID:        user.UserID,
		User:      user,
		AdminRole: adminRole,
	}
	err := p.db.Save(&admin).Error
	if err != nil {
		return fmt.Errorf("failed to set user as admin: %v", err)
	}
	log.Printf("User %d successfully set as admin", user.UserID)
	return nil
}

// Проверить, является ли пользователь админом
func (p *PostgresRepository) IsAdmin(userID int64) bool {
	var admin Admin
	err := p.db.Where("id = ?", userID).First(&admin).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Printf("failed to figure out if user %d is admin: %v", userID, err)
	}
	return err == nil
}

// Получить роль админа
func (p *PostgresRepository) GetAdminRole(userID int64) (string, error) {
	var admin Admin
	err := p.db.Where("id = ?", userID).First(&admin).Error
	if err != nil {
		return "", fmt.Errorf("failed to get %d admin role: %v", userID, err)
	}
	return admin.AdminRole, nil
}

// Удалить админа
func (p *PostgresRepository) RemoveAdmin(userID int64) error {
	var admin Admin
	err := p.db.Where("id = ?", userID).Delete(&admin).Error
	if err != nil {
		return fmt.Errorf("failed to remove user %d from admins: %v", userID, err)
	}
	log.Printf("User %d successfully removed from admins", userID)
	return nil
}

// Продвинуть админа
func (p *PostgresRepository) PromoteAdmin(userID int64, delta string) error {
	var admin Admin
	err := p.db.Where("id = ?", userID).First(&admin).Error
	if err != nil {
		return fmt.Errorf("failed to get %d from admins: %v", userID, err)
	}
	if delta == "+" && admin.AdminRole == "junior" {
		admin.AdminRole = "senior"
	} else if delta == "-" && admin.AdminRole == "senior" {
		admin.AdminRole = "junior"
	} else {
		return fmt.Errorf("delta %s can't be used to change role %s", delta, admin.AdminRole)
	}
	err = p.SaveAdmin(admin.User, admin.AdminRole)
	if err != nil {
		return err
	}
	return nil
}

// Обновить количество сообщений пользователя
func (p *PostgresRepository) UpdateUserMessageCount(userID int64, delta int) error {
	user, err := p.GetUser(userID)
	if err != nil {
		return err
	}
	user.MessageCount += delta
	return p.SaveUser(&user)
}

// Получить количество сообщений пользователя
func (p *PostgresRepository) GetUserMessageCount(userID int64) (int, error) {
	user, err := p.GetUser(userID)
	if err != nil {
		return 0, err
	}
	return user.MessageCount, nil
}

// Очистить количество сообщений у всех пользователей
func (p *PostgresRepository) ClearAllUsersMessageCount() error {
	err := p.db.Model(&User{}).Update("message_count", 0).Error
	if err != nil {
		return fmt.Errorf("failed to clear all users message count: %v", err)
	}
	return nil
}

// Получить топ 5 пользователей с наибольшим количеством сообщений
func (p *PostgresRepository) GetUsersWithTopActivity() ([]User, error) {
	var users []User
	err := p.db.Order("message_count DESC").Limit(5).Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users with top activity: %v", err)
	}
	return users, nil
}

func (p *PostgresRepository) GetUserBirthday(userID int64) (time.Time, error) {
	var user User
	err := p.db.Where("user_id = ?", userID).First(&user).Error
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get user birthday: %v", err)
	}
	return user.Birthday, nil
}

func (p *PostgresRepository) UpdateUserBirthday(userID int64, birthday time.Time) error {
	user, err := p.GetUser(userID)
	if err != nil {
		return err
	}
	user.Birthday = birthday
	return p.SaveUser(&user)
}

func (p *PostgresRepository) GetUsersWithBirthdayToday() ([]User, error) {
	var users []User
	now := time.Now().In(MoscowTZ)
	month := int(now.Month())
	day := now.Day()

	err := p.db.
		Where("EXTRACT(MONTH FROM birthday) = ? AND EXTRACT(DAY FROM birthday) = ?", month, day).
		Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users with birthday today: %v", err)
	}
	return users, nil
}
