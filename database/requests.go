package database

import (
	"fmt"
	"log"
	"time"

	"saxbot/environment"
	"slices"

	"gorm.io/gorm"
)

// Получить пользователя по Telegram UserID, создает нового если не найден
func GetUser(userID int64) (*User, error) {
	var user User
	err := DB.Where("user_id = ?", userID).First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Пользователь не найден - создаем нового
			log.Printf("Creating new user %d (not found in database)", userID)
			admins := environment.GetAdmins()

			newUser := User{
				UserID:    userID,
				Username:  "",
				IsAdmin:   slices.Contains(admins, userID),
				Warns:     0,
				Status:    "active",
				IsWinner:  false,
				AdminPref: "",
			}

			err = DB.Create(&newUser).Error
			if err != nil {
				return nil, fmt.Errorf("failed to create user: %v", err)
			}

			log.Printf("Created new user %d, IsAdmin: %t", userID, newUser.IsAdmin)
			return &newUser, nil
		}
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	// Проверяем и обновляем админский статус
	if updateUserAdminStatus(userID, &user) {
		err = DB.Save(&user).Error
		if err != nil {
			log.Printf("Failed to update admin status for user %d: %v", userID, err)
		}
	}

	return &user, nil
}

// Сохранить пользователя в базе данных
func SaveUser(user *User) error {
	return DB.Save(user).Error
}

// Обновить количество предупреждений пользователя
func UpdateUserWarns(userID int64, delta int) error {
	user, err := GetUser(userID)
	if err != nil {
		return err
	}

	user.Warns += delta
	if user.Warns < 0 {
		user.Warns = 0
	}

	return DB.Save(user).Error
}

// Обновить админский статус пользователя на основе переменной окружения ADMINS
func updateUserAdminStatus(userID int64, user *User) bool {
	admins := environment.GetAdmins()
	if len(admins) == 0 {
		return false
	}

	newAdminStatus := slices.Contains(admins, userID)
	if user.IsAdmin != newAdminStatus {
		log.Printf("Updating admin status for user %d: %t -> %t", userID, user.IsAdmin, newAdminStatus)
		user.IsAdmin = newAdminStatus
		return true
	}
	return false
}

// Обновить админский статус для всех пользователей
func RefreshAllUsersAdminStatus() error {
	log.Printf("Starting admin status refresh for all users...")

	var users []User
	err := DB.Find(&users).Error
	if err != nil {
		return fmt.Errorf("failed to get users: %v", err)
	}

	updatedCount := 0
	for _, user := range users {
		if updateUserAdminStatus(user.UserID, &user) {
			err = DB.Save(&user).Error
			if err != nil {
				log.Printf("Failed to save updated user data for %d: %v", user.UserID, err)
				continue
			}
			updatedCount++
		}
	}

	log.Printf("Admin status refresh completed. Updated %d users out of %d total.", updatedCount, len(users))
	return nil
}

// Сбросить состояние IsWinner в false у всех пользователей
func ResetAllUsersWinnerStatus() error {
	log.Printf("Starting winner status reset for all users...")

	result := DB.Model(&User{}).Where("is_winner = ?", true).Update("is_winner", false)
	if result.Error != nil {
		return fmt.Errorf("failed to reset winner status: %v", result.Error)
	}

	log.Printf("Winner status reset completed. Updated %d users.", result.RowsAffected)
	return nil
}

// Получить всех пользователей из базы данных
func GetAllUsers() ([]User, error) {
	var users []User
	err := DB.Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %v", err)
	}
	return users, nil
}

// Сохранить данные квиза на определенную дату
func SaveQuizData(quote, songName string, quizTime time.Time) error {
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	quizTimeInMoscow := quizTime.In(moscowTZ)
	date := time.Date(quizTimeInMoscow.Year(), quizTimeInMoscow.Month(), quizTimeInMoscow.Day(), 0, 0, 0, 0, time.UTC)

	var existingQuiz Quiz
	err := DB.Where("date = ?", date).First(&existingQuiz).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing quiz: %v", err)
	}

	if err == gorm.ErrRecordNotFound {
		quiz := Quiz{
			Date:     date,
			Quote:    quote,
			SongName: songName,
			QuizTime: quizTimeInMoscow,
			IsActive: true,
		}

		err = DB.Create(&quiz).Error
		if err != nil {
			return fmt.Errorf("failed to create quiz: %v", err)
		}

		log.Printf("Created new quiz for date %s", date.Format("2006-01-02"))
	} else {
		existingQuiz.Quote = quote
		existingQuiz.SongName = songName
		existingQuiz.QuizTime = quizTimeInMoscow
		existingQuiz.IsActive = true

		err = DB.Save(&existingQuiz).Error
		if err != nil {
			return fmt.Errorf("failed to update quiz: %v", err)
		}

		log.Printf("Updated quiz for date %s", date.Format("2006-01-02"))
	}

	return nil
}

// Получить данные квиза на сегодня
func LoadQuizData() (string, string, time.Time, error) {
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ)
	date := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	var quiz Quiz
	err := DB.Where("date = ? AND is_active = ?", date, true).First(&quiz).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", "", time.Time{}, fmt.Errorf("quiz data for today not found")
		}
		return "", "", time.Time{}, fmt.Errorf("failed to load quiz data: %v", err)
	}

	return quiz.Quote, quiz.SongName, quiz.QuizTime, nil
}

// Получить квиз по дате
func GetQuizByDate(date time.Time) (*Quiz, error) {
	normalizedDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	var quiz Quiz
	err := DB.Where("date = ?", normalizedDate).First(&quiz).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get quiz by date: %v", err)
	}

	return &quiz, nil
}

// Получить все активные квизы
func GetActiveQuizzes() ([]Quiz, error) {
	var quizzes []Quiz
	err := DB.Where("is_active = ?", true).Order("date DESC").Find(&quizzes).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get active quizzes: %v", err)
	}
	return quizzes, nil
}

// Завершить квиз
func DeactivateQuiz(quizID uint) error {
	result := DB.Model(&Quiz{}).Where("id = ?", quizID).Update("is_active", false)
	if result.Error != nil {
		return fmt.Errorf("failed to deactivate quiz: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("quiz with id %d not found", quizID)
	}
	return nil
}

// Получить пользователя по username
func GetUserByUsername(username string) (*User, error) {
	var user User
	err := DB.Where("username = ?", username).First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by username: %v", err)
	}

	return &user, nil
}

// Обновить username пользователя
func UpdateUsername(userID int64, username string) error {
	result := DB.Model(&User{}).Where("user_id = ?", userID).Update("username", username)
	if result.Error != nil {
		return fmt.Errorf("failed to update username: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user with id %d not found", userID)
	}
	return nil
}

// Установить статус победителя для пользователя
func SetUserWinnerStatus(userID int64, isWinner bool) error {
	result := DB.Model(&User{}).Where("user_id = ?", userID).Update("is_winner", isWinner)
	if result.Error != nil {
		return fmt.Errorf("failed to update winner status: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user with id %d not found", userID)
	}
	return nil
}

// Проверить, был ли квиз сегодня уже проведен
func GetQuizAlreadyWas() (bool, error) {
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ)
	date := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	var quiz Quiz
	err := DB.Where("date = ?", date).First(&quiz).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check quiz status: %v", err)
	}

	return !quiz.IsActive, nil
}

// Пометить квиз как завершенный
func SetQuizAlreadyWas() error {
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ)
	date := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	log.Printf("Attempting to mark quiz as completed for date: %s", date.Format("2006-01-02"))

	result := DB.Model(&Quiz{}).Where("date = ? AND is_active = ?", date, true).Update("is_active", false)
	if result.Error != nil {
		log.Printf("Failed to mark quiz as completed: %v", result.Error)
		return fmt.Errorf("failed to mark quiz as completed: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		log.Printf("No active quiz found for today (%s) to mark as completed", date.Format("2006-01-02"))

		var quiz Quiz
		err := DB.Where("date = ?", date).First(&quiz).Error
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
func ForceCompleteQuiz() error {
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	today := time.Now().In(moscowTZ)
	date := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	log.Printf("Force completing quiz for date: %s", date.Format("2006-01-02"))

	result := DB.Model(&Quiz{}).Where("date = ?", date).Update("is_active", false)
	if result.Error != nil {
		log.Printf("Failed to force complete quiz: %v", result.Error)
		return fmt.Errorf("failed to force complete quiz: %v", result.Error)
	}

	log.Printf("Force completed quiz for today (affected %d rows)", result.RowsAffected)
	return nil
}
