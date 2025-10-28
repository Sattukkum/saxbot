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

// Используем московское время (UTC+3)
var MoscowTZ = time.FixedZone("Moscow", 3*60*60)

func EstimateQuizTime() time.Time {
	now := time.Now().In(MoscowTZ)

	randomHour := rand.Intn(11) + 10

	quizTime := time.Date(now.Year(), now.Month(), now.Day(), randomHour, 0, 0, 0, MoscowTZ)

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
	if quiz, pgErr := db.LoadQuizData(); pgErr == nil {
		QuizTime := quiz.QuizTime.In(MoscowTZ)
		log.Printf("Загружены полные данные квиза из PostgreSQL: Quote='%s', SongName='%s', Time=%s", quiz.Quote, quiz.SongName, QuizTime.Format("15:04"))
		quiz := QuoteQuiz{
			Quote:    quiz.Quote,
			SongName: quiz.Quote,
			QuizTime: QuizTime,
		}
		previousQuiz, err := db.GetLastCompletedQuiz()
		if err != nil {
			log.Printf("failed to get last quiz: %v", err)
			yesterday := time.Now().In(MoscowTZ).AddDate(0, 0, -1)
			lastQuizDate = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, MoscowTZ)
			return quiz, lastQuizDate
		}
		lastQuizDate = previousQuiz.Date
		return quiz, lastQuizDate
	} else {
		newQuiz := GetNewQuiz(db)
		previousQuiz, err := db.GetLastCompletedQuiz()
		if err != nil {
			log.Printf("failed to get last quiz: %v", err)
			yesterday := time.Now().In(MoscowTZ).AddDate(0, 0, -1)
			lastQuizDate = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, MoscowTZ)
			return newQuiz, lastQuizDate
		}
		lastQuizDate = previousQuiz.Date
		return newQuiz, lastQuizDate
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
