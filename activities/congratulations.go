package activities

import (
	"fmt"
	"log"
	"math/rand"
	"saxbot/database"
	textcases "saxbot/text_cases"
	"time"

	tele "gopkg.in/telebot.v4"
)

func ManageCongratulations(bot *tele.Bot, quizChatID int64, rep *database.PostgresRepository) {
	for {
		now := time.Now().In(MoscowTZ)
		todayTenAm := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, MoscowTZ)
		if now.Before(todayTenAm) {
			time.Sleep(1 * time.Hour)
			continue
		}
		users, err := rep.GetUsersWithBirthdayToday()
		if err != nil {
			log.Printf("failed to get users with birthday today: %v", err)
			continue
		}
		message := textcases.GetCongratulationsMessage()
		if len(users) > 0 {
			for _, user := range users {
				if user.Username == "" {
					message += fmt.Sprintf("🎉 <b>%s</b> 🎉\n", user.FirstName)
				} else {
					message += fmt.Sprintf("🎉 <b>@%s</b> 🎉\n", user.Username)
				}
			}
			r := rand.Intn(3) + 1
			imagePath := fmt.Sprintf("images/birthday/birthday%d.jpg", r)
			photo := &tele.Photo{
				File:    tele.FromDisk(imagePath),
				Caption: message,
			}
			opts := &tele.SendOptions{
				ParseMode: tele.ModeHTML,
				ThreadID:  0,
			}
			if _, err := bot.Send(tele.ChatID(quizChatID), photo, opts); err != nil {
				log.Printf("failed to send birthday congratulations: %v", err)
				continue
			}
			time.Sleep(3 * time.Second)
			bot.Send(tele.ChatID(quizChatID), "Товарищ! Если хочешь, чтобы бот и тебя поздравил с днем рождения, напиши мне (КПСС боту) в личные сообщения", opts)
		}
		time.Sleep(24 * time.Hour)
	}
}
