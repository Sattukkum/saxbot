package activities

import (
	"fmt"
	"log"
	"math/rand"
	"saxbot/admins"
	"saxbot/database"
	"saxbot/messages"
	textcases "saxbot/text_cases"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v4"
)

type QuoteQuiz struct {
	Quote      string
	SongName   string
	QuizTime   time.Time
	IsClip     bool
	ScreenPath string
}

type QuizManager struct {
	mu             sync.RWMutex
	TodayQuiz      QuoteQuiz
	QuizRunning    bool
	QuizAlreadyWas bool
	WinnerID       int64
	IsLastQuizClip bool
	QuizChatID     int64
}

// Thread-safe helpers
func (qm *QuizManager) GetState() (todayQuiz QuoteQuiz, quizRunning, quizAlreadyWas bool, winnerID int64, isLastQuizClip bool, quizChatID int64) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	return qm.TodayQuiz, qm.QuizRunning, qm.QuizAlreadyWas, qm.WinnerID, qm.IsLastQuizClip, qm.QuizChatID
}

func (qm *QuizManager) SetTodayQuiz(quiz QuoteQuiz) {
	qm.mu.Lock()
	qm.TodayQuiz = quiz
	qm.mu.Unlock()
}

func (qm *QuizManager) SetQuizRunning(running bool) {
	qm.mu.Lock()
	qm.QuizRunning = running
	qm.mu.Unlock()
}

func (qm *QuizManager) SetQuizAlreadyWas(done bool) {
	qm.mu.Lock()
	qm.QuizAlreadyWas = done
	qm.mu.Unlock()
}

func (qm *QuizManager) SetIsLastQuizClip(isClip bool) {
	qm.mu.Lock()
	qm.IsLastQuizClip = isClip
	qm.mu.Unlock()
}

func (qm *QuizManager) SetWinnerID(winnerID int64) {
	qm.mu.Lock()
	qm.WinnerID = winnerID
	qm.mu.Unlock()
}

func (qm *QuizManager) UpdateTodayQuiz(update func(q *QuoteQuiz)) {
	qm.mu.Lock()
	update(&qm.TodayQuiz)
	qm.mu.Unlock()
}

func (qm *QuizManager) ResetForNewDay() {
	qm.mu.Lock()
	qm.QuizAlreadyWas = false
	qm.QuizRunning = false
	qm.TodayQuiz = QuoteQuiz{}
	qm.mu.Unlock()
}

func (qm *QuizManager) IsRunning() bool {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	return qm.QuizRunning
}

func (qm *QuizManager) Winner() int64 {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	return qm.WinnerID
}

// Используем московское время (UTC+3)
var MoscowTZ = time.FixedZone("Moscow", 3*60*60)

func estimateQuizTime() time.Time {
	now := time.Now().In(MoscowTZ)

	randomHour := rand.Intn(11) + 10

	randomMinute := rand.Intn(60)

	quizTime := time.Date(now.Year(), now.Month(), now.Day(), randomHour, randomMinute, 0, 0, MoscowTZ)

	return quizTime
}

func getTodayQuiz(isLastQuizClip bool) QuoteQuiz {
	if isLastQuizClip {
		quote, songName := textcases.GetRandomQuote()
		quizTime := estimateQuizTime()

		return QuoteQuiz{
			Quote:    quote,
			SongName: songName,
			QuizTime: quizTime,
		}
	}
	return getClipQuiz()
}

func getNewQuiz(db *database.PostgresRepository, isLastQuizClip bool) (todayQuiz QuoteQuiz) {
	todayQuiz = getTodayQuiz(isLastQuizClip)

	log.Printf("Generated quiz: Quote='%s', SongName='%s', Time=%s, IsClip='%t', ScreenPath='%s'", todayQuiz.Quote, todayQuiz.SongName, todayQuiz.QuizTime.Format("15:04"), todayQuiz.IsClip, todayQuiz.ScreenPath)

	if err := db.SaveQuizData(todayQuiz.Quote, todayQuiz.SongName, todayQuiz.ScreenPath, todayQuiz.IsClip, todayQuiz.QuizTime); err != nil {
		log.Printf("Ошибка сохранения данных квиза: %v", err)
	} else {
		log.Printf("Установлены и сохранены полные данные квиза на сегодня: Quote='%s', SongName='%s', Time=%s, IsClip='%t', ScreenPath='%s'",
			todayQuiz.Quote, todayQuiz.SongName, todayQuiz.QuizTime.Format("15:04"), todayQuiz.IsClip, todayQuiz.ScreenPath)
	}
	return todayQuiz
}

func getClipQuiz() QuoteQuiz {
	clipName := textcases.GetRandomClip()
	quizTime := estimateQuizTime()
	basePath := "images/clips/%s/%d.jpg"
	randImage := rand.Intn(7) + 1
	for key, value := range clipName {
		screenPath := fmt.Sprintf(basePath, value, randImage)
		return QuoteQuiz{
			SongName:   key,
			ScreenPath: screenPath,
			QuizTime:   quizTime,
			IsClip:     true,
		}
	}
	return QuoteQuiz{
		SongName:   "",
		ScreenPath: "",
		QuizTime:   quizTime,
		IsClip:     true,
	}
}

func ManageQuiz(rep *database.PostgresRepository, bot *tele.Bot, quizManager *QuizManager) {
	moscowTZ := time.FixedZone("Moscow", 3*60*60)

	var lastQuizDate time.Time
	now := time.Now().In(moscowTZ)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, moscowTZ)
	lastQuiz, err := rep.GetLastCompletedQuiz()
	if err != nil {
		log.Printf("Не удалось загрузить данные последнего квиза")
	}

	if lastQuiz != nil {
		lastQuizDate = lastQuiz.Date.In(moscowTZ)
		quizManager.SetIsLastQuizClip(lastQuiz.IsClip)
		if !today.After(lastQuizDate) {
			quizManager.SetQuizAlreadyWas(true)
			quizManager.SetTodayQuiz(QuoteQuiz{
				Quote:      lastQuiz.Quote,
				SongName:   lastQuiz.SongName,
				QuizTime:   lastQuiz.QuizTime,
				IsClip:     lastQuiz.IsClip,
				ScreenPath: lastQuiz.ScreenPath,
			})
			log.Printf("Квиз сегодня уже был проведен")
		}
	}

	quizManager.SetQuizRunning(false)

	// Текущий день для отслеживания смены суток
	currentDay := today

	for {
		now = time.Now().In(moscowTZ)
		// Пересчитываем "сегодня" каждый цикл
		today = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, moscowTZ)

		todayQuiz, quizRunning, quizAlreadyWas, _, isLastQuizClip, _ := quizManager.GetState()

		// Если наступили новые сутки, сбрасываем состояние квиза
		if !today.Equal(currentDay) {
			currentDay = today
			quizManager.ResetForNewDay()
			// Обновляем isLastQuizClip из базы данных при смене дня
			lastQuiz, err := rep.GetLastCompletedQuiz()
			if err == nil && lastQuiz != nil {
				quizManager.SetIsLastQuizClip(lastQuiz.IsClip)
				isLastQuizClip = lastQuiz.IsClip
			} else {
				quizManager.SetIsLastQuizClip(false)
				isLastQuizClip = false
				log.Printf("Не удалось загрузить данные последнего квиза, устанавливаем isLastQuizClip в false")
			}
			todayQuiz = QuoteQuiz{}
			quizRunning = false
			quizAlreadyWas = false
		}

		// Если на сегодня нет сгенерированного времени квиза и квиз ещё не проводился — создаём
		if !quizAlreadyWas && todayQuiz.QuizTime.IsZero() {
			newQuiz := getNewQuiz(rep, isLastQuizClip)
			quizManager.SetTodayQuiz(newQuiz)
			todayQuiz = newQuiz
		}

		log.Printf("now: %s, todayQuiz.QuizTime: %s", now.Format("15:04"), todayQuiz.QuizTime.Format("15:04"))
		log.Printf("quizAlreadyWas: %v, quizRunning: %v", quizAlreadyWas, quizRunning)

		if now.After(todayQuiz.QuizTime) && !quizAlreadyWas && !quizRunning {
			// Проверяем и заполняем данные только для квизов-цитат (не клипов)
			if !todayQuiz.IsClip && (todayQuiz.Quote == "" || todayQuiz.SongName == "") {
				quote, songName := textcases.GetRandomQuote()
				quizManager.UpdateTodayQuiz(func(q *QuoteQuiz) {
					q.Quote = quote
					q.SongName = songName
				})
				todayQuiz.Quote = quote
				todayQuiz.SongName = songName
				log.Printf("Установлены данные квиза: Quote='%s', SongName='%s'", quote, songName)
			} else if todayQuiz.IsClip && (todayQuiz.ScreenPath == "" || todayQuiz.SongName == "") {
				clipName := textcases.GetRandomClip()
				for key, value := range clipName {
					quizManager.UpdateTodayQuiz(func(q *QuoteQuiz) {
						q.ScreenPath = value
						q.SongName = key
					})
					todayQuiz.ScreenPath = value
					todayQuiz.SongName = key
				}
				log.Printf("Установлены данные квиза: ScreenPath='%s', SongName='%s'", todayQuiz.ScreenPath, todayQuiz.SongName)
			}

			_, _, _, _, _, quizChatID := quizManager.GetState()
			admins.RemovePref(bot, &tele.Chat{ID: quizChatID}, rep)

			quizManager.SetQuizRunning(true)
			log.Printf("Starting quiz in chat %d", quizChatID)
			if !todayQuiz.IsClip {
				_, err = bot.Send(tele.ChatID(quizChatID), textcases.QuizAnnouncement, &tele.SendOptions{ThreadID: 0})
				if err != nil {
					log.Printf("Failed to send quiz intro message: %v", err)
				}
				time.Sleep(100 * time.Millisecond)
				quoteMessage := fmt.Sprintf("Сегодняшняя цитата:\n%s", todayQuiz.Quote)
				_, err = bot.Send(tele.ChatID(quizChatID), quoteMessage, &tele.SendOptions{ThreadID: 0})
				if err != nil {
					log.Printf("Failed to send quiz question message: %v", err)
				}
			} else {
				screen := &tele.Photo{
					File:    tele.FromDisk(todayQuiz.ScreenPath),
					Caption: textcases.QuizClipAnnouncement,
				}
				_, err = bot.Send(tele.ChatID(quizChatID), screen, &tele.SendOptions{ThreadID: 0})
				if err != nil {
					log.Printf("Failed to send quiz clip intro message: %v", err)
				}
			}
		}

		winner, _ := rep.GetQuizWinnerID()
		quizManager.SetWinnerID(winner)
		time.Sleep(1 * time.Minute)
	}
}

func ManageRunningQuiz(rep *database.PostgresRepository, bot *tele.Bot, quizManager *QuizManager, c tele.Context, appeal string) {
	_, quizRunning, _, _, _, _ := quizManager.GetState()
	log.Printf("Quiz running: %v", quizRunning)
	log.Print(c.Message().Text)
	todayQuiz, _, _, _, _, _ := quizManager.GetState()
	log.Print(todayQuiz.SongName)
	if strings.EqualFold(c.Message().Text, todayQuiz.SongName) {
		quizManager.SetQuizRunning(false)
		quizManager.SetQuizAlreadyWas(true)
		rep.SetQuizAlreadyWas()
		winnerTitle := textcases.GetRandomTitle()
		messages.ReplyMessage(c, fmt.Sprintf("Правильно! Песня: %s", todayQuiz.SongName), c.Message().ThreadID)
		time.Sleep(100 * time.Millisecond)
		messages.ReplyMessage(c, fmt.Sprintf("Поздравляем, %s! Ты победил и получил титул %s до следующего квиза!", appeal, winnerTitle), c.Message().ThreadID)
		chatMember := &tele.ChatMember{User: c.Message().Sender, Role: tele.Member}
		admins.SetPref(bot, c.Chat(), chatMember, winnerTitle)
		quiz, err := rep.GetLastCompletedQuiz()
		if err != nil {
			log.Printf("failed to get last completed quiz: %v", err)
		} else if quiz != nil {
			err = rep.SetQuizWinner(quiz.ID, c.Message().Sender.ID)
			if err != nil {
				log.Printf("failed to set user %d as a quiz winner %v", c.Message().Sender.ID, err)
			}
		}
		// Обновляем isLastQuizClip после завершения квиза для правильного чередования
		quizManager.SetIsLastQuizClip(todayQuiz.IsClip)
	}
}
