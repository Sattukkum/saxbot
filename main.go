package main

import (
	"flag"
	"fmt"
	"log"
	"saxbot/activities"
	"saxbot/admins"
	"saxbot/environment"
	"saxbot/messages"
	redisClient "saxbot/redis"
	textcases "saxbot/text_cases"
	"slices"
	"strings"
	"time"

	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v4"
)

var lastKatyaMessage time.Time
var photoFlag = false
var todayQuiz activities.QuoteQuiz
var quizRunning = false
var quizAlreadyWas = false

func main() {
	godotenv.Load()

	environment := environment.GetMainEnvironment()
	botToken := environment.Token
	allowedChats := environment.AllowedChats
	katyaID := environment.KatyaID
	// adminsList := environment.Admins
	redisHost := environment.RedisHost
	redisPort := environment.RedisPort
	// redisDB := environment.RedisDB
	quizChatID := environment.QuizChatID

	// Флаги командной строки
	clearRedis := flag.Bool("clear-redis", false, "Очистить базу данных Redis и выйти")
	showInfo := flag.Bool("info", false, "Показать информацию о базе данных Redis и выйти")
	flag.Parse()

	if redisHost == "" {
		redisHost = "localhost"
	}
	if redisPort == "" {
		redisPort = "6379"
	}
	redisAddr := redisHost + ":" + redisPort

	// Инициализируем подключение к Redis
	err := redisClient.InitRedis(redisAddr, "", 0)
	if err != nil {
		log.Fatalf("Не удалось подключиться к Redis: %v", err)
	}
	defer redisClient.CloseRedis()

	if *showInfo {
		redisClient.ShowInfo()
		return
	}

	if *clearRedis {
		redisClient.ClearRedis()
		return
	}

	log.Printf("Обновляем админские права пользователей из переменной окружения ADMINS...")
	err = redisClient.RefreshAllUsersAdminStatus()
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

	go func() {
		for {
			time.Sleep(30 * time.Second)
			photoFlag = false
		}
	}()

	go func() {
		moscowTZ := time.FixedZone("Moscow", 3*60*60)

		// Получить данные квиза из Redis
		todayQuiz, lastQuizDate := activities.GetQuizData()

		// Получить флаг "квиз уже был" из Redis
		if wasQuiz, err := redisClient.GetQuizAlreadyWas(); err == nil {
			quizAlreadyWas = wasQuiz
			if wasQuiz {
				log.Printf("Квиз сегодня уже был проведен")
			}
		} else {
			log.Printf("Не удалось загрузить флаг квиза из Redis: %v", err)
		}

		quizRunning = false

		for {
			now := time.Now().In(moscowTZ)
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, moscowTZ)

			// Определить новое время квиза (раз в сутки)
			if !today.Equal(lastQuizDate) {
				todayQuiz = activities.GetNewQuiz()
				quizAlreadyWas = false
				lastQuizDate = today
			}

			// Если время квиза не установлено (например, при первом запуске), устанавливаем его
			if todayQuiz.QuizTime.IsZero() {
				todayQuiz.QuizTime = activities.EstimateQuizTime()
				// Сохраняем в Redis для консистентности
				if err := redisClient.SaveQuizData(todayQuiz.Quote, todayQuiz.SongName, todayQuiz.QuizTime); err != nil {
					log.Printf("Ошибка сохранения времени квиза в Redis: %v", err)
				}
			}

			log.Printf("now: %s, todayQuiz.QuizTime: %s", now.Format("15:04"), todayQuiz.QuizTime.Format("15:04"))
			log.Printf("quizAlreadyWas: %v, quizRunning: %v", quizAlreadyWas, quizRunning)

			if now.After(todayQuiz.QuizTime) && !quizAlreadyWas && !quizRunning {
				if todayQuiz.Quote == "" || todayQuiz.SongName == "" {
					quote, songName := textcases.GetRandomQuote()
					todayQuiz.Quote = quote
					todayQuiz.SongName = songName
				}

				admins.RemovePref(bot, &tele.Chat{ID: quizChatID})

				quizRunning = true
				log.Printf("Starting quiz in chat %d", quizChatID)
				_, err = bot.Send(tele.ChatID(quizChatID), textcases.QuizAnnouncement, &tele.SendOptions{ThreadID: 0})
				if err != nil {
					log.Printf("Failed to send quiz intro message: %v", err)
				}
				time.Sleep(100 * time.Millisecond)
				quoteMessage := fmt.Sprintf("Сегодняшняя цитата:\n%s", todayQuiz.Quote)
				log.Printf("Sending quote message: %s", quoteMessage)
				_, err = bot.Send(tele.ChatID(quizChatID), quoteMessage, &tele.SendOptions{ThreadID: 0})
				if err != nil {
					log.Printf("Failed to send quiz question message: %v", err)
				}
			}

			time.Sleep(1 * time.Minute)
		}
	}()

	// Очистка истекших ключей из памяти
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			err := redisClient.CleanupExpiredKeys()
			if err != nil {
				log.Printf("Error during cleanup: %v", err)
			}
		}
	}()

	lastKatyaMessage = time.Now().Add(-30 * time.Minute)

	bot.Handle(tele.OnText, func(c tele.Context) error {
		log.Printf("Received message: '%s' from user %d in chat %d", c.Message().Text, c.Message().Sender.ID, c.Message().Chat.ID)

		if !slices.Contains(allowedChats, c.Message().Chat.ID) {
			log.Printf("Получил сообщение в чат %d. Ожидаются чаты %v", c.Message().Chat.ID, allowedChats)
			return nil
		}

		messageThreadID := c.Message().ThreadID

		userID := c.Message().Sender.ID
		isReply := c.Message().IsReply()
		var replyToID int64
		var replyToUserData *redisClient.UserData

		if isReply {
			replyToID = c.Message().ReplyTo.Sender.ID
		}

		userData, err := redisClient.GetUser(userID)
		if err != nil {
			log.Printf("Failed to get user data: %v", err)
			return nil
		}
		if userData.Username != c.Message().Sender.Username {
			userData.Username = c.Message().Sender.Username
			redisClient.SetUser(userID, userData)
			redisClient.SetUserPersistent(userID, userData)
		}

		if userData.Status == "muted" {
			bot.Delete(c.Message())
			return nil
		}

		if userData.Status == "banned" {
			userData.Status = "active"
			redisClient.SetUser(userID, userData)
			redisClient.SetUserPersistent(userID, userData)
			messages.SendMessage(c, fmt.Sprintf("@%s, тебя разбанили, но это можно исправить. Веди себя хорошо", userData.Username), messageThreadID)
		}

		if isReply {
			replyToUserData, err = redisClient.GetUser(replyToID)
			if err != nil {
				log.Printf("Failed to get reply to user data: %v", err)
				return nil
			}
			if replyToUserData.Username != c.Message().ReplyTo.Sender.Username {
				replyToUserData.Username = c.Message().ReplyTo.Sender.Username
				redisClient.SetUser(replyToID, replyToUserData)
				redisClient.SetUserPersistent(replyToID, replyToUserData)
			}
		}

		if userID == katyaID {
			if time.Since(lastKatyaMessage) > 30*time.Minute {
				lastKatyaMessage = time.Now()
				messages.ReplyMessage(c, "🚨ВНИМАНИЕ! АЛАРМ!🚨 КАТЕНЬКА В ЧАТЕ!💀 ЭТО НЕ УЧЕБНАЯ ТРЕВОГА! ПОВТОРЯЮ, ЭТО НЕ УЧЕБНАЯ ТРЕВОГА!⛔\n❗ВСЕМ ОБЯЗАТЕЛЬНО СЛУШАТЬСЯ КАТЕНЬКУ❗", messageThreadID)
			}
			lastKatyaMessage = time.Now()
		}

		if userData.IsAdmin || userID == katyaID || userData.IsWinner {
			switch c.Message().Text {
			case "Предупреждение", "предупреждение", "ПРЕДУПРЕЖДЕНИЕ":
				if isReply {
					replyToUserData.Warns++
					redisClient.SetUser(replyToID, replyToUserData)
					redisClient.SetUserPersistent(replyToID, replyToUserData)
					var text string
					if strings.EqualFold(c.Message().ReplyTo.Text, "Лена") {
						text = textcases.GetWarnCase(c.Message().ReplyTo.Sender.Username, true)
					} else {
						text = textcases.GetWarnCase(c.Message().ReplyTo.Sender.Username, false)
					}
					return messages.ReplyToOriginalMessage(c, text, messageThreadID)
				} else {
					return messages.ReplyMessage(c, "Ты кого предупреждаешь?", messageThreadID)
				}
			case "Извинись", "извинись", "ИЗВИНИСЬ":
				if isReply {
					return messages.ReplyToOriginalMessage(c, "Извинись дон. Скажи, что ты был не прав дон. Или имей в виду — на всю оставшуюся жизнь у нас с тобой вражда", messageThreadID)
				}
			case "Пошел нахуй", "пошел нахуй", "Пошла нахуй", "пошла нахуй", "/ban":
				if isReply && userID != katyaID && (userData.IsAdmin || !userData.IsWinner) {
					if replyToUserData.IsAdmin {
						return messages.ReplyMessage(c, "Ты не можешь банить других админов, соси писос", messageThreadID)
					}
					user := c.Message().ReplyTo.Sender
					chatMember := &tele.ChatMember{User: user, Role: tele.Member}
					admins.BanUser(bot, c.Message().Chat, chatMember)
					bot.Delete(c.Message().ReplyTo)
					return messages.ReplyMessage(c, fmt.Sprintf("@%s идет нахуй из чатика", user.Username), messageThreadID)
				} else {
					if userID == katyaID {
						return messages.ReplyMessage(c, "Катенька, зачиллься, остынь, успокойся, не надо так", messageThreadID)
					}
					return messages.ReplyMessage(c, "Банхаммер готов. Кого послать нахуй?", messageThreadID)
				}
			case "Мут", "мут", "Ебало завали", "ебало завали", "/mute":
				if isReply && userID != katyaID && (userData.IsAdmin || !userData.IsWinner) {
					if replyToUserData.IsAdmin {
						return messages.ReplyMessage(c, "Ты не можешь мутить других админов, соси писос", messageThreadID)
					}
					user := c.Message().ReplyTo.Sender
					chatMember := &tele.ChatMember{User: user, Role: tele.Member, Rights: tele.Rights{
						CanSendMessages: false,
					}}
					admins.MuteUser(bot, c.Chat(), chatMember)
					return messages.ReplyMessage(c, fmt.Sprintf("@%s помолчит полчасика и подумает о своем поведении", replyToUserData.Username), messageThreadID)
				} else {
					if userID == katyaID {
						return messages.ReplyMessage(c, "Катенька, зачиллься, остынь, успокойся, не надо так", messageThreadID)
					}
					return messages.ReplyMessage(c, "Кого мутить?", messageThreadID)
				}
			case "Размут", "размут", "/unmute":
				if isReply {
					chatMember := &tele.ChatMember{User: c.Message().ReplyTo.Sender, Role: tele.Member, Rights: tele.Rights{
						CanSendMessages: true,
					}}
					admins.UnmuteUser(bot, c.Chat(), chatMember)
					return messages.ReplyMessage(c, fmt.Sprintf("@%s размучен. А то че как воды в рот набрал", replyToUserData.Username), messageThreadID)
				} else {
					return messages.ReplyMessage(c, "Кого размутить?", messageThreadID)
				}
			case "Нацик", "нацик", "НАЦИК":
				if isReply && userID != katyaID && (userData.IsAdmin || !userData.IsWinner) {
					if replyToUserData.IsAdmin {
						return messages.ReplyMessage(c, "Ты не можешь банить других админов, соси писос", messageThreadID)
					}
					user := c.Message().ReplyTo.Sender
					messages.ReplyToOriginalMessage(c, fmt.Sprintf("@%s, скажи ауфидерзейн своим нацистским яйцам!", user.Username), messageThreadID)
					time.Sleep(1 * time.Second)
					chatMember := &tele.ChatMember{User: user, Role: tele.Member}
					admins.BanUser(bot, c.Message().Chat, chatMember)
					bot.Delete(c.Message().ReplyTo)
					return messages.SendMessage(c, fmt.Sprintf("@%s идет нахуй из чатика", user.Username), messageThreadID)
				} else {
					if userID == katyaID {
						return messages.ReplyMessage(c, "Катенька, зачиллься, остынь, успокойся, не надо так", messageThreadID)
					}
					return messages.ReplyMessage(c, "Кому яйца жмут?", messageThreadID)
				}
			}
		}
		switch c.Message().Text {
		case "🎰":
			bot.Delete(c.Message())
			return nil
		case "Инфа", "инфа", "/info":
			text := textcases.GetInfo()
			return messages.SendMessage(c, text, messageThreadID)
		case "Админ", "админ", "/report":
			if isReply {
				return messages.ReplyToOriginalMessage(c, fmt.Sprintf("@%s вызывает админов. В чатике дичь\n@fatiurs, @puwyb, @murmuIlya, @RavenMxL", userData.Username), messageThreadID)
			} else {
				return messages.SendMessage(c, fmt.Sprintf("@%s вызывает админов. В чатике дичь\n@fatiurs, @puwyb, @murmuIlya, @RavenMxL", userData.Username), messageThreadID)
			}
		case "Преды", "преды", "/warns":
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
		if quizRunning {
			if strings.EqualFold(c.Message().Text, todayQuiz.SongName) {
				quizRunning = false
				quizAlreadyWas = true
				redisClient.SetQuizAlreadyWas()
				winnerTitle := textcases.GetRandomTitle()
				messages.ReplyMessage(c, fmt.Sprintf("Правильно! Песня: %s", todayQuiz.SongName), messageThreadID)
				time.Sleep(100 * time.Millisecond)
				messages.ReplyMessage(c, fmt.Sprintf("Поздравляем, %s! Ты победил и получил титул %s до следующего квиза!", c.Message().Sender.Username, winnerTitle), messageThreadID)
				chatMember := &tele.ChatMember{User: c.Message().Sender, Role: tele.Member}
				admins.SetPref(bot, c.Chat(), chatMember, winnerTitle)
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

		userData, err := redisClient.GetUser(joinedUser.ID)
		if err != nil {
			log.Printf("Failed to get user data: %v", err)
			return nil
		}
		if userData.Username != joinedUser.Username {
			userData.Username = joinedUser.Username
			redisClient.SetUser(joinedUser.ID, userData)
			redisClient.SetUserPersistent(joinedUser.ID, userData)
		}

		return messages.SendMessage(c, fmt.Sprintf(`Добро пожаловать, @%s! Ты присоединился к чатику братства нежити. Напиши команду "Инфа", чтобы узнать, как тут все устроено`, userData.Username), c.Message().ThreadID)
	})

	bot.Handle(tele.OnPhoto, func(c tele.Context) error {
		log.Printf("Received photo from user %d in chat %d", c.Message().Sender.ID, c.Message().Chat.ID)

		if !slices.Contains(allowedChats, c.Message().Chat.ID) {
			return nil
		}

		userData, err := redisClient.GetUser(c.Message().Sender.ID)
		if err != nil {
			log.Printf("Failed to get user data: %v", err)
			return nil
		}
		if userData.Username != c.Message().Sender.Username {
			userData.Username = c.Message().Sender.Username
			redisClient.SetUser(c.Message().Sender.ID, userData)
			redisClient.SetUserPersistent(c.Message().Sender.ID, userData)
		}

		userID := c.Message().Sender.ID

		if userID == katyaID && !photoFlag {
			photoFlag = true
			lastKatyaMessage = time.Now()
			return messages.ReplyMessage(c, "💖 СРОЧНО ВСЕМ ЛЮБОВАТЬСЯ НОВОЙ ФОТОЧКОЙ КАТЕНЬКИ! 💖\n😠 ЗА НЕГАТИВНЫЕ РЕАКЦИИ ПОЛУЧИТЕ ПРЕДУПРЕЖДЕНИЕ! 😠", c.Message().ThreadID)
		}

		return nil
	})

	bot.Handle(tele.OnDice, func(c tele.Context) error {
		log.Printf("Received dice from user %d in chat %d", c.Message().Sender.ID, c.Message().Chat.ID)
		if c.Message().Dice.Type == tele.Slot.Type && !admins.IsAdmin(c.Message().Sender.ID) {
			bot.Delete(c.Message())
		}
		return nil
	})

	bot.Start()
}
