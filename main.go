package main

import (
	"fmt"
	"log"
	"math/rand"
	"saxbot/activities"
	"saxbot/admins"
	"saxbot/database"
	"saxbot/environment"
	"saxbot/messages"
	"strconv"

	textcases "saxbot/text_cases"
	"slices"
	"strings"
	"time"

	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v4"
	"gorm.io/gorm"
)

var todayQuiz activities.QuoteQuiz
var quizRunning = false
var quizAlreadyWas = false
var winnerID int64

func main() {
	godotenv.Load()

	mainEnv := environment.GetMainEnvironment()
	botToken := mainEnv.Token
	allowedChats := mainEnv.AllowedChats
	adminsList := mainEnv.Admins
	quizChatID := mainEnv.QuizChatID
	adminsUsernames := mainEnv.AdminsUsernames

	if quizChatID == 0 {
		panic("TARGET_CHAT is not set")
	}
	if botToken == "" {
		panic("BOT_TOKEN is not set")
	}
	if allowedChats == nil {
		panic("ALLOWED_CHATS is not set")
	}

	var err error
	var db *gorm.DB

	// подключение к PostgreSQL
	pgEnv := environment.GetPostgreSQLEnvironment()

	db, err = database.InitPostgreSQL(pgEnv.Host, pgEnv.User, pgEnv.Password, pgEnv.Database, pgEnv.Port, pgEnv.SSLMode)
	if err != nil {
		panic(fmt.Sprintf("Предупреждение: не удалось подключиться к PostgreSQL: %v", err))
	} else {
		defer database.ClosePostgreSQL(db)

		err = database.AutoMigrate(db)
		if err != nil {
			log.Printf("Предупреждение: не удалось выполнить миграцию PostgreSQL: %v", err)
		}
	}

	rep := database.NewPostgresRepository(db)

	log.Printf("Обновляем админские права пользователей из переменной окружения ADMINS...")
	err = rep.RefreshAllUsersAdminStatus()
	if err != nil {
		log.Printf("Предупреждение: не удалось обновить админские права: %v", err)
	}

	pref := tele.Settings{
		Token:  botToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	// Управление квизом
	go func() {
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
			if !today.After(lastQuizDate) {
				quizAlreadyWas = true
				todayQuiz = activities.QuoteQuiz{
					Quote:    lastQuiz.Quote,
					SongName: lastQuiz.SongName,
					QuizTime: lastQuiz.QuizTime,
				}
				log.Printf("Квиз сегодня уже был проведен")
			}
		}

		quizRunning = false

		// Текущий день для отслеживания смены суток
		currentDay := today

		for {
			now = time.Now().In(moscowTZ)
			// Пересчитываем "сегодня" каждый цикл
			today = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, moscowTZ)

			// Если наступили новые сутки, сбрасываем состояние квиза
			if !today.Equal(currentDay) {
				currentDay = today
				quizAlreadyWas = false
				quizRunning = false
				todayQuiz = activities.QuoteQuiz{}
			}

			// Если на сегодня нет сгенерированного времени квиза и квиз ещё не проводился — создаём
			if !quizAlreadyWas && todayQuiz.QuizTime.IsZero() {
				todayQuiz = activities.GetNewQuiz(rep)
			}

			log.Printf("now: %s, todayQuiz.QuizTime: %s", now.Format("15:04"), todayQuiz.QuizTime.Format("15:04"))
			log.Printf("quizAlreadyWas: %v, quizRunning: %v", quizAlreadyWas, quizRunning)

			if now.After(todayQuiz.QuizTime) && !quizAlreadyWas && !quizRunning {
				if todayQuiz.Quote == "" || todayQuiz.SongName == "" {
					quote, songName := textcases.GetRandomQuote()
					todayQuiz.Quote = quote
					todayQuiz.SongName = songName
				}

				admins.RemovePref(bot, &tele.Chat{ID: quizChatID}, rep)

				quizRunning = true
				log.Printf("Starting quiz in chat %d", quizChatID)
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
			}

			winnerID, _ = rep.GetQuizWinnerID()
			time.Sleep(1 * time.Minute)
		}
	}()

	// Управление объявлениями
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	go func() {
		moscowTZ := time.FixedZone("Moscow", 3*60*60)
		var previousTheme int = 1
		var currentTheme int
		var imagePath string
		var caption string
		for {
			now := time.Now().In(moscowTZ)
			from := time.Date(now.Year(), now.Month(), now.Day(), 10, 30, 0, 0, moscowTZ)
			to := time.Date(now.Year(), now.Month(), now.Day(), 22, 30, 0, 0, moscowTZ)
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
				_, err := bot.Send(tele.ChatID(quizChatID), photo, opts)
				if err != nil {
					log.Printf("не получилось отправить объявление в чат! %v", err)
				}
			} else {
				log.Printf("Текущее время вне диапазона объявлений, пропускаем... %s", now.Format("15:04"))
			}
			time.Sleep(3 * time.Hour)
			previousTheme = currentTheme
		}
	}()

	bot.Handle(tele.OnText, func(c tele.Context) error {
		log.Printf("Received message: '%s' from user %d in chat %d", c.Message().Text, c.Message().Sender.ID, c.Message().Chat.ID)

		if !slices.Contains(allowedChats, c.Message().Chat.ID) {
			log.Printf("Получил сообщение в чат %d. Ожидаются чаты %v", c.Message().Chat.ID, allowedChats)
			return nil
		}

		messageThreadID := c.Message().ThreadID

		userID := c.Message().Sender.ID
		var chatID int64
		var chatAdmin = false
		if c.Message().SenderChat != nil {
			chatID = c.Message().SenderChat.ID
			log.Printf("!!! Сообщение от канала %d", chatID)
			if slices.Contains(adminsList, chatID) {
				chatAdmin = true
			}
		}
		isReply := c.Message().IsReply()
		appeal := "@" + c.Message().Sender.Username
		if appeal == "@" {
			appeal = c.Message().Sender.FirstName
		}
		var replyToID int64
		var replyToUserData database.User
		var replyToAppeal string

		if isReply {
			replyToID = c.Message().ReplyTo.Sender.ID
			replyToAppeal = "@" + c.Message().ReplyTo.Sender.Username
			if replyToAppeal == "@" {
				replyToAppeal = c.Message().ReplyTo.Sender.FirstName
			}
		}

		userData, err := rep.GetUser(userID)
		if err != nil {
			log.Printf("Failed to get user data: %v", err)
			return nil
		}
		if userData.Username != c.Message().Sender.Username || userData.FirstName != c.Message().Sender.FirstName {
			userData.Username = c.Message().Sender.Username
			userData.FirstName = c.Message().Sender.FirstName
			if err := rep.SaveUser(&userData); err != nil {
				log.Printf("Failed to save persistent username update for user %d: %v", userID, err)
			}
		}

		if userData.Status == "muted" {
			bot.Delete(c.Message())
			return nil
		}

		if userData.Status == "banned" {
			if c.Message().OriginalSender != nil || c.Message().OriginalChat != nil {
				log.Printf("Получено пересланное сообщение от забаненного пользователя %d, автоматический разбан не выполняется", userID)
				return nil
			}

			userData.Status = "active"
			if err := rep.SaveUser(&userData); err != nil {
				log.Printf("Failed to save persistent status update for user %d: %v", userID, err)
			}
			messages.ReplyMessage(c, fmt.Sprintf("%s, тебя разбанили, но это можно исправить. Веди себя хорошо", appeal), messageThreadID)
		}

		if isReply {
			replyToUserData, err = rep.GetUser(replyToID)
			if err != nil {
				log.Printf("Failed to get reply to user data: %v", err)
				return nil
			}
			if replyToUserData.Username != c.Message().ReplyTo.Sender.Username {
				replyToUserData.Username = c.Message().ReplyTo.Sender.Username
				if err := rep.SaveUser(&replyToUserData); err != nil {
					log.Printf("Failed to save persistent username update for reply user %d: %v", replyToID, err)
				}
			}
		}

		replyToAdmin := false
		if isReply {
			replyToAdmin = rep.IsAdmin(replyToUserData.UserID)
		}
		var adminRole = "junior"
		isAdmin := rep.IsAdmin(userData.UserID)
		if isAdmin {
			adminRole, err = rep.GetAdminRole(userData.UserID)
			if err != nil {
				log.Printf("failed to get admin role, consider it junior")
				adminRole = "junior"
			}
		}
		if chatAdmin {
			adminRole = "senior"
		}

		isWinner := userData.UserID == winnerID

		text := strings.ToLower(c.Message().Text)

		if isAdmin || isWinner || chatAdmin {
			switch text {
			case "предупреждение":
				if isReply {
					if err := rep.UpdateUserWarns(replyToID, 1); err != nil {
						log.Printf("Failed to save warns increase for user %d: %v", replyToID, err)
					} else {
						replyToUserData.Warns++
					}
					var text string
					if strings.EqualFold(c.Message().ReplyTo.Text, "Лена") || strings.EqualFold(c.Message().ReplyTo.Text, "Елена") || strings.EqualFold(c.Message().ReplyTo.Text, "Елена Вячеславовна") {
						text = textcases.GetWarnCase(replyToAppeal, true)
					} else {
						text = textcases.GetWarnCase(replyToAppeal, false)
					}
					return messages.ReplyToOriginalMessage(c, text, messageThreadID)
				} else {
					return messages.ReplyMessage(c, "Ты кого предупреждаешь?", messageThreadID)
				}
			case "извинись":
				if isReply {
					return messages.ReplyToOriginalMessage(c, "Извинись дон. Скажи, что ты был не прав дон. Или имей в виду — на всю оставшуюся жизнь у нас с тобой вражда", messageThreadID)
				}
			case "пошел нахуй", "пошла нахуй", "пошёл нахуй", "иди нахуй", "/ban":
				if adminRole == "senior" {
					if isReply {
						if replyToAdmin {
							return messages.ReplyMessage(c, "Ты не можешь банить других админов, соси писос", messageThreadID)
						}
						user := c.Message().ReplyTo.Sender
						chatMember := &tele.ChatMember{User: user, Role: tele.Member}
						admins.BanUser(bot, c.Message().Chat, chatMember, rep)
						bot.Delete(c.Message().ReplyTo)
						return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика", replyToAppeal), messageThreadID)
					} else {
						return messages.ReplyMessage(c, "Банхаммер готов. Кого послать нахуй?", messageThreadID)
					}
				}
			case "размут", "/unmute":
				if isReply {
					chatMember := &tele.ChatMember{User: c.Message().ReplyTo.Sender, Role: tele.Member, Rights: tele.Rights{
						CanSendMessages: true,
					}}
					admins.UnmuteUser(bot, c.Chat(), chatMember, rep)
					return messages.ReplyMessage(c, fmt.Sprintf("%s размучен. А то че как воды в рот набрал", replyToAppeal), messageThreadID)
				} else {
					return messages.ReplyMessage(c, "Кого размутить?", messageThreadID)
				}
			case "нацик":
				if adminRole == "senior" {
					if isReply {
						if replyToAdmin {
							return messages.ReplyMessage(c, "Ты не можешь банить других админов, соси писос", messageThreadID)
						}
						user := c.Message().ReplyTo.Sender
						messages.ReplyToOriginalMessage(c, fmt.Sprintf("%s, скажи ауфидерзейн своим нацистским яйцам!", replyToAppeal), messageThreadID)
						time.Sleep(1 * time.Second)
						chatMember := &tele.ChatMember{User: user, Role: tele.Member}
						admins.BanUser(bot, c.Message().Chat, chatMember, rep)
						bot.Delete(c.Message().ReplyTo)
						return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика", replyToAppeal), messageThreadID)
					} else {
						return messages.ReplyMessage(c, "Кому яйца жмут?", messageThreadID)
					}
				}
			}
		}
		switch text {
		case "инфа", "/info":
			text := textcases.GetInfo()
			return messages.ReplyFormattedHTML(c, text, messageThreadID)
		case "админ", "/report":
			log.Printf("Got an admin command from %d", userData.UserID)
			if isReply {
				return messages.ReplyToOriginalMessage(c, textcases.GetAdminsCommand(appeal, adminsUsernames), messageThreadID)
			} else {
				return messages.ReplyMessage(c, textcases.GetAdminsCommand(appeal, adminsUsernames), messageThreadID)
			}
		case "преды", "/warns":
			switch {
			case userData.Warns == 0:
				return messages.ReplyMessage(c, "Тебя ещё не предупреждали? Срочно предупредите его!", messageThreadID)
			case userData.Warns > 0 && userData.Warns < 10:
				return messages.ReplyMessage(c, fmt.Sprintf("У тебя %d предупреждений. Помни, предупрежден — значит предупрежден", userData.Warns), messageThreadID)
			case userData.Warns >= 10 && userData.Warns < 100:
				return messages.ReplyMessage(c, fmt.Sprintf("У тебя %d предупреждений. Этот парень совсем слов не понимает?", userData.Warns), messageThreadID)
			case userData.Warns >= 100 && userData.Warns < 1000:
				return messages.ReplyMessage(c, fmt.Sprintf("У тебя %d предупреждений. Я от тебя в светлом ахуе. Ты когда-нибудь перестанешь?", userData.Warns), messageThreadID)
			case userData.Warns >= 1000:
				return messages.ReplyMessage(c, fmt.Sprintf("У тебя %d предупреждений. Ты постиг нирвану и вышел за пределы сознания. Тебя больше ничто не остановит", userData.Warns), messageThreadID)
			}
		}

		var durationMinutes uint = 30 // стандартное значение
		var isMuteCommand bool = false
		parts := strings.Fields(text)

		if len(parts) > 0 {
			prefix := parts[0]

			if prefix == "мут" || prefix == "ебало" || prefix == "/mute" {
				// Проверяем, есть ли число в конце
				if len(parts) > 1 {
					lastPart := parts[len(parts)-1]
					lastPart = strings.Replace(lastPart, "-", "", 1)
					if mins, err := strconv.Atoi(lastPart); err == nil && mins > 0 {
						durationMinutes = uint(mins)
						isMuteCommand = true
					}
				}
			}
		}

		if isMuteCommand {
			if isReply && (isAdmin || chatAdmin) {
				if replyToAdmin {
					return messages.ReplyMessage(c, "Ты не можешь мутить других админов, соси писос", messageThreadID)
				}

				user := c.Message().ReplyTo.Sender
				chatMember := &tele.ChatMember{
					User: user,
					Role: tele.Member,
					Rights: tele.Rights{
						CanSendMessages: false,
					},
				}

				admins.MuteUser(bot, c.Chat(), chatMember, rep, durationMinutes)
				return messages.ReplyMessage(c, fmt.Sprintf("%s помолчит %d минут и подумает о своем поведении", replyToAppeal, durationMinutes), messageThreadID)
			} else {
				return messages.ReplyMessage(c, "Кого мутить?", messageThreadID)
			}
		}

		if quizRunning {
			log.Printf("Quiz running: %v", quizRunning)
			log.Print(c.Message().Text)
			log.Print(todayQuiz.SongName)
			if strings.EqualFold(c.Message().Text, todayQuiz.SongName) {
				quizRunning = false
				quizAlreadyWas = true
				rep.SetQuizAlreadyWas()
				winnerTitle := textcases.GetRandomTitle()
				messages.ReplyMessage(c, fmt.Sprintf("Правильно! Песня: %s", todayQuiz.SongName), messageThreadID)
				time.Sleep(100 * time.Millisecond)
				messages.ReplyMessage(c, fmt.Sprintf("Поздравляем, %s! Ты победил и получил титул %s до следующего квиза!", appeal, winnerTitle), messageThreadID)
				chatMember := &tele.ChatMember{User: c.Message().Sender, Role: tele.Member}
				admins.SetPref(bot, c.Chat(), chatMember, winnerTitle)
				quiz, _ := rep.GetLastCompletedQuiz()
				err = rep.SetQuizWinner(quiz.ID, userData.UserID)
				if err != nil {
					log.Printf("failed to set user %d as a quiz winner %v", userData.UserID, err)
				}
			}
		}
		return nil
	})

	bot.Handle(tele.OnUserJoined, func(c tele.Context) error {
		joinedUser := c.Message().UserJoined
		log.Printf("User %d joined chat %d", joinedUser.ID, c.Message().Chat.ID)

		if !slices.Contains(allowedChats, c.Message().Chat.ID) {
			return nil
		}

		userData, err := rep.GetUser(joinedUser.ID)
		if err != nil {
			log.Printf("Failed to get user data: %v", err)
			return nil
		}
		if userData.Username != joinedUser.Username || userData.FirstName != joinedUser.FirstName {
			userData.Username = joinedUser.Username
			userData.FirstName = joinedUser.FirstName
			if err := rep.SaveUser(&userData); err != nil {
				log.Printf("Failed to save persistent username update for joined user %d: %v", joinedUser.ID, err)
			}
		}

		appeal := "@" + joinedUser.Username
		if appeal == "@" {
			appeal = joinedUser.FirstName
		}

		return messages.ReplyMessage(c, fmt.Sprintf(`Добро пожаловать, %s! Ты присоединился к чатику братства нежити. Напиши команду "Инфа", чтобы узнать, как тут все устроено`, appeal), c.Message().ThreadID)
	})

	bot.Start()
}
