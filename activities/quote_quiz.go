package activities

import (
	"log"
	"math/rand"
	"saxbot/database"
	textcases "saxbot/text_cases"
	"time"
)

type QuoteQuiz struct {
	Quote    string
	SongName string
	QuizTime time.Time
}

func EstimateQuizTime() time.Time {
	// Используем московское время (UTC+3)
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	now := time.Now().In(moscowTZ)

	randomHour := rand.Intn(11) + 10

	quizTime := time.Date(now.Year(), now.Month(), now.Day(), randomHour, 0, 0, 0, moscowTZ)

	return quizTime
}

func GetTodayQuiz() QuoteQuiz {
	quote, songName := textcases.GetRandomQuote()
	quizTime := EstimateQuizTime()

	return QuoteQuiz{
		Quote:    quote,
		SongName: songName,
		QuizTime: quizTime,
	}
}

func GetQuizData(db *database.PostgresRepository) (quiz QuoteQuiz, lastQuizDate time.Time) {
	if quote, songName, savedTime, pgErr := db.LoadQuizData(); pgErr == nil {
		moscowTZ := time.FixedZone("Moscow", 3*60*60)
		QuizTime := savedTime.In(moscowTZ)
		today := time.Date(savedTime.Year(), savedTime.Month(), savedTime.Day(), 0, 0, 0, 0, moscowTZ)
		lastQuizDate := today
		log.Printf("Загружены полные данные квиза из PostgreSQL: Quote='%s', SongName='%s', Time=%s", quote, songName, QuizTime.Format("15:04"))
		quiz := QuoteQuiz{
			Quote:    quote,
			SongName: songName,
			QuizTime: QuizTime,
		}
		return quiz, lastQuizDate
	} else {
		moscowTZ := time.FixedZone("Moscow", 3*60*60)
		yesterday := time.Now().In(moscowTZ).AddDate(0, 0, -1)
		yesterdayDate := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, moscowTZ)
		return QuoteQuiz{}, yesterdayDate
	}
}

func GetNewQuiz(db *database.PostgresRepository) (todayQuiz QuoteQuiz) {
	todayQuiz = GetTodayQuiz()

	log.Printf("Generated quiz: Quote='%s', SongName='%s', Time=%s", todayQuiz.Quote, todayQuiz.SongName, todayQuiz.QuizTime.Format("15:04"))

	if err := db.SaveQuizData(todayQuiz.Quote, todayQuiz.SongName, todayQuiz.QuizTime); err != nil {
		log.Printf("Ошибка сохранения данных квиза: %v", err)
	} else {
		log.Printf("Установлены и сохранены полные данные квиза на сегодня: Quote='%s', SongName='%s', Time=%s",
			todayQuiz.Quote, todayQuiz.SongName, todayQuiz.QuizTime.Format("15:04"))
	}
	return todayQuiz
}
