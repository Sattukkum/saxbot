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

func ManageTrackOfTheDay(bot *tele.Bot, m *QuizManager, rep *database.PostgresRepository, postGate chan struct{}, postDone chan struct{}) {
	const (
		singles      = 5
		ussr_mixtape = 8
		rest         = 10
	)
	album := rand.Intn(5) + 1
	for {
		now := time.Now().In(MoscowTZ)
		todayTwoPm := time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 0, 0, MoscowTZ)
		todayFivePm := time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, MoscowTZ)

		if m.QuizRunning {
			time.Sleep(10 * time.Minute)
			continue
		}

		if now.Before(todayTwoPm) || now.After(todayFivePm) {
			time.Sleep(1 * time.Hour)
			continue
		}

		var trackMax int
		switch album {
		case 1:
			trackMax = singles
		case 2:
			trackMax = ussr_mixtape
		case 3, 4, 5:
			trackMax = rest
		default:
			trackMax = singles // На всякий случай - самое маленькое возможное количество
		}

		track := rand.Intn(trackMax) + 1

		audioData := textcases.GetTrack(album, track, rep)
		if audioData.ID == 0 {
			log.Printf("failed to get audio by album ID %d and track number %d", album, track)
			continue
		}

		caption := fmt.Sprintf(`Хой! Песня дня сегодня — <b>%s</b>
Слушаем всем чатом и делимся мнениями!

<b>Комментарий от Ника:</b>

%s`, audioData.Name, audioData.Description)

		if audioData.ClipURL != "" {
			caption = fmt.Sprintf("%s\n\n<b><a href=\"%s\">Смотреть клип</a></b>", caption, audioData.ClipURL)
		}
		audio := &tele.Audio{
			File: tele.File{
				FileID: audioData.FileID,
			},
			Caption: caption,
		}
		opts := &tele.SendOptions{
			ParseMode: tele.ModeHTML,
			ThreadID:  0,
		}
		<-postGate
		if _, err := bot.Send(tele.ChatID(m.QuizChatID), audio, opts); err != nil {
			log.Printf("failed to send track of the day: %v", err)
			postDone <- struct{}{}
			continue
		}
		postDone <- struct{}{}
		album += 1
		if album > 5 {
			album = 1
		}
		time.Sleep(24 * time.Hour)
	}
}
