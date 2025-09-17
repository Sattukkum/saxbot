package activities

import (
	"log"
	"math/rand"
	"saxbot/database"
	redisClient "saxbot/redis"
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

func GetQuizData() (quiz QuoteQuiz, lastQuizDate time.Time) {
	if quote, songName, savedTime, err := redisClient.LoadQuizData(); err == nil {
		moscowTZ := time.FixedZone("Moscow", 3*60*60)
		Quote := quote
		SongName := songName
		QuizTime := savedTime.In(moscowTZ)
		today := time.Date(savedTime.Year(), savedTime.Month(), savedTime.Day(), 0, 0, 0, 0, moscowTZ)
		lastQuizDate := today
		log.Printf("Загружены полные данные квиза из Redis: Quote='%s', SongName='%s', Time=%s",
			Quote, SongName, QuizTime.Format("15:04"))
		quiz := QuoteQuiz{
			Quote:    Quote,
			SongName: SongName,
			QuizTime: QuizTime,
		}
		return quiz, lastQuizDate
	} else {
		log.Printf("Не удалось загрузить данные квиза из Redis: %v", err)

		if quote, songName, savedTime, pgErr := database.LoadQuizData(); pgErr == nil {
			moscowTZ := time.FixedZone("Moscow", 3*60*60)
			QuizTime := savedTime.In(moscowTZ)
			today := time.Date(savedTime.Year(), savedTime.Month(), savedTime.Day(), 0, 0, 0, 0, moscowTZ)
			lastQuizDate := today
			log.Printf("Загружены полные данные квиза из PostgreSQL (fallback): Quote='%s', SongName='%s', Time=%s",
				quote, songName, QuizTime.Format("15:04"))

			redisClient.SaveQuizData(quote, songName, savedTime)

			quiz := QuoteQuiz{
				Quote:    quote,
				SongName: songName,
				QuizTime: QuizTime,
			}
			return quiz, lastQuizDate
		} else {
			log.Printf("Не удалось загрузить данные квиза из PostgreSQL: %v", pgErr)
		}

		moscowTZ := time.FixedZone("Moscow", 3*60*60)
		yesterday := time.Now().In(moscowTZ).AddDate(0, 0, -1)
		yesterdayDate := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, moscowTZ)
		return QuoteQuiz{}, yesterdayDate
	}
}

func GetNewQuiz() (todayQuiz QuoteQuiz) {
	if err := redisClient.ClearQuizAlreadyWas(); err != nil {
		log.Printf("Ошибка очистки флага квиза в Redis: %v", err)
	} else {
		log.Printf("Очищен флаг 'квиз уже был' для нового дня")
	}

	todayQuiz = GetTodayQuiz()

	log.Printf("Generated quiz: Quote='%s', SongName='%s', Time=%s", todayQuiz.Quote, todayQuiz.SongName, todayQuiz.QuizTime.Format("15:04"))

	if todayQuiz.Quote == "" || todayQuiz.SongName == "" {
		log.Printf("ПРЕДУПРЕЖДЕНИЕ: Сгенерированный квиз содержит пустые данные!")
		log.Printf("Возможно, проблема с функцией GetRandomQuote() или данными в SongQuotes")
	}

	if err := database.SaveQuizDataSync(todayQuiz.Quote, todayQuiz.SongName, todayQuiz.QuizTime); err != nil {
		log.Printf("Ошибка сохранения данных квиза: %v", err)
	} else {
		log.Printf("Установлены и сохранены полные данные квиза на сегодня: Quote='%s', SongName='%s', Time=%s",
			todayQuiz.Quote, todayQuiz.SongName, todayQuiz.QuizTime.Format("15:04"))
	}
	return todayQuiz
}
