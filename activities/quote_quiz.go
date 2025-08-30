package activities

import (
	"math/rand"
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
