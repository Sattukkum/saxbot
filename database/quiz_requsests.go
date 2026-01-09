package database

import (
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// Сохранить данные квиза на определенную дату
func (p *PostgresRepository) SaveQuizData(quote, songName, screenPath string, isClip bool, quizTime time.Time) error {
	quizTimeInMoscow := quizTime.In(MoscowTZ)
	date := time.Date(quizTimeInMoscow.Year(), quizTimeInMoscow.Month(), quizTimeInMoscow.Day(), 0, 0, 0, 0, time.UTC)

	var existingQuiz Quiz
	err := p.db.Where("date = ?", date).First(&existingQuiz).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing quiz: %w", err)
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
			return fmt.Errorf("failed to create quiz: %w", err)
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
			return fmt.Errorf("failed to update quiz: %w", err)
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
		return nil, fmt.Errorf("quiz data for today not found: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load quiz data: %w", err)
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
		return nil, fmt.Errorf("failed to get quiz by date: %w", err)
	}

	return &quiz, nil
}

// Получить все активные квизы
func (p *PostgresRepository) GetActiveQuizzes() ([]Quiz, error) {
	var quizzes []Quiz

	err := p.db.Where("is_active = ?", true).Order("date DESC").Find(&quizzes).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get active quizzes: %w", err)
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
		return fmt.Errorf("failed to set winner for quiz %d: %w", quizID, result.Error)
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
		return nil, fmt.Errorf("failed to get last completed quiz: %w", err)
	}

	return &quiz, nil
}

// Завершить квиз
func (p *PostgresRepository) DeactivateQuiz(quizID uint) error {
	result := p.db.Model(&Quiz{}).Where("id = ?", quizID).Update("is_active", false)
	if result.Error != nil {
		return fmt.Errorf("failed to deactivate quiz: %w", result.Error)
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
		return false, fmt.Errorf("failed to check quiz status: %w", err)
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
		return fmt.Errorf("failed to mark quiz as completed: %w", result.Error)
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
		return fmt.Errorf("failed to force complete quiz: %w", result.Error)
	}

	log.Printf("Force completed quiz for today (affected %d rows)", result.RowsAffected)
	return nil
}

// Получить ID победителя последнего завершенного квиза
func (p *PostgresRepository) GetQuizWinnerID() (int64, error) {
	lastQuiz, err := p.GetLastCompletedQuiz()
	if err != nil {
		return 0, fmt.Errorf("failed to get last quiz winner id: %w", err)
	}
	if lastQuiz == nil {
		return 0, nil
	}
	return lastQuiz.WinnerID, nil
}
