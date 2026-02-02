package activities

import (
	"log"
	"math/rand"
	textcases "saxbot/text_cases"
	"time"

	tele "gopkg.in/telebot.v4"
)

func ManageAds(bot *tele.Bot, r *rand.Rand, m *QuizManager) {
	moscowTZ := time.FixedZone("Moscow", 3*60*60)
	var previousTheme int = 1
	var currentTheme int
	var imagePath string
	var caption string
	for {
		now := time.Now().In(moscowTZ)
		from := time.Date(now.Year(), now.Month(), now.Day(), 10, 30, 0, 0, moscowTZ)
		to := time.Date(now.Year(), now.Month(), now.Day(), 22, 30, 0, 0, moscowTZ)
		if m.QuizRunning {
			time.Sleep(10 * time.Minute)
			continue
		}
		if now.After(from) && now.Before(to) {
			imagePath, caption, currentTheme = textcases.GetAd(previousTheme, r)
			log.Printf("imagePath: %s", imagePath)
			log.Printf("caption: %s", caption)
			photo := &tele.Photo{
				File:    tele.FromDisk(imagePath),
				Caption: caption,
			}
			opts := &tele.SendOptions{
				ParseMode: tele.ModeHTML,
				ThreadID:  0,
			}
			_, err := bot.Send(tele.ChatID(m.QuizChatID), photo, opts)
			if err != nil {
				log.Printf("не получилось отправить объявление в чат! %v", err)
			}
		} else {
			log.Printf("Текущее время вне диапазона объявлений, пропускаем... %s", now.Format("15:04"))
		}
		time.Sleep(3 * time.Hour)
		previousTheme = currentTheme
	}
}
