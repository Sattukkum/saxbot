package main

import (
	"context"
	"fmt"
	"log"
	"saxbot/activities"
	"saxbot/admins"
	"saxbot/database"
	"saxbot/domain"
	"saxbot/environment"
	"saxbot/messages"
	redisClient "saxbot/redis"
	"saxbot/sync"
	textcases "saxbot/text_cases"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	tele "gopkg.in/telebot.v4"
	"gorm.io/gorm"
)

var todayQuiz activities.QuoteQuiz
var quizRunning = false
var quizAlreadyWas = false

func main() {
	godotenv.Load()

	mainEnv := environment.GetMainEnvironment()
	botToken := mainEnv.Token
	allowedChats := mainEnv.AllowedChats
	// adminsList := mainEnv.Admins
	redisHost := mainEnv.RedisHost
	redisPort := mainEnv.RedisPort
	// redisDB := mainEnv.RedisDB
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

	redisAddr := redisHost + ":" + strconv.Itoa(redisPort)
	var err error
	var redis *redis.Client
	var db *gorm.DB
	ctx := context.Background()

	// подключение к Redis
	redis, err = redisClient.InitRedis(redisAddr, "", 0)
	if err != nil {
		log.Fatalf("Не удалось подключиться к Redis: %v", err)
	}
	defer redisClient.CloseRedis(redis)

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

	sync, redisRep, pgRep := sync.NewSyncService(redis, ctx, db)

	log.Printf("Обновляем админские права пользователей из переменной окружения ADMINS...")
	err = sync.RefreshAllUsersAdminStatus()
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
		todayQuiz, lastQuizDate = activities.GetQuizData(pgRep)

		if wasQuiz, err := pgRep.GetQuizAlreadyWas(); err == nil {
			quizAlreadyWas = wasQuiz
			if wasQuiz {
				log.Printf("Квиз сегодня уже был проведен")
			}
		} else {
			log.Printf("Не удалось загрузить флаг квиза: %v", err)
		}

		quizRunning = false

		for {
			now := time.Now().In(moscowTZ)
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, moscowTZ)

			if !today.Equal(lastQuizDate) {
				todayQuiz = activities.GetNewQuiz(pgRep)
				quizAlreadyWas = false
				lastQuizDate = today
			}

			if todayQuiz.QuizTime.IsZero() {
				todayQuiz.QuizTime = activities.EstimateQuizTime()
				if err := pgRep.SaveQuizData(todayQuiz.Quote, todayQuiz.SongName, todayQuiz.QuizTime); err != nil {
					log.Printf("Ошибка сохранения времени квиза: %v", err)
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

				admins.RemovePref(bot, &tele.Chat{ID: quizChatID}, sync)

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

			time.Sleep(1 * time.Minute)
		}
	}()

	// Очистка истекших ключей из памяти
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			err := redisRep.CleanupExpiredKeys()
			if err != nil {
				log.Printf("Error during cleanup: %v", err)
			}
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
		isReply := c.Message().IsReply()
		appeal := "@" + c.Message().Sender.Username
		if appeal == "@" {
			appeal = c.Message().Sender.FirstName
		}
		var replyToID int64
		var replyToUserData domain.User
		var replyToAppeal string

		if isReply {
			replyToID = c.Message().ReplyTo.Sender.ID
			replyToAppeal = "@" + c.Message().ReplyTo.Sender.Username
			if replyToAppeal == "@" {
				replyToAppeal = c.Message().ReplyTo.Sender.FirstName
			}
		}

		userData, err := sync.GetUser(userID)
		if err != nil {
			log.Printf("Failed to get user data: %v", err)
			return nil
		}
		if userData.Username != c.Message().Sender.Username || userData.FirstName != c.Message().Sender.FirstName {
			userData.Username = c.Message().Sender.Username
			userData.FirstName = c.Message().Sender.FirstName
			if err := sync.SaveUser(&userData); err != nil {
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
			if err := sync.SaveUser(&userData); err != nil {
				log.Printf("Failed to save persistent status update for user %d: %v", userID, err)
			}
			messages.ReplyMessage(c, fmt.Sprintf("%s, тебя разбанили, но это можно исправить. Веди себя хорошо", appeal), messageThreadID)
		}

		if isReply {
			replyToUserData, err = sync.GetUser(replyToID)
			if err != nil {
				log.Printf("Failed to get reply to user data: %v", err)
				return nil
			}
			if replyToUserData.Username != c.Message().ReplyTo.Sender.Username {
				replyToUserData.Username = c.Message().ReplyTo.Sender.Username
				if err := sync.SaveUser(&replyToUserData); err != nil {
					log.Printf("Failed to save persistent username update for reply user %d: %v", replyToID, err)
				}
			}
		}

		if userData.IsAdmin || userData.IsWinner {
			switch c.Message().Text {
			case "Предупреждение", "предупреждение", "ПРЕДУПРЕЖДЕНИЕ":
				if isReply {
					if err := sync.UpdateUserWarns(replyToID, 1); err != nil {
						log.Printf("Failed to save warns increase for user %d: %v", replyToID, err)
					} else {
						replyToUserData.Warns++
					}
					var text string
					if strings.EqualFold(c.Message().ReplyTo.Text, "Лена") {
						text = textcases.GetWarnCase(replyToAppeal, true)
					} else {
						text = textcases.GetWarnCase(replyToAppeal, false)
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
				if isReply && (userData.IsAdmin || !userData.IsWinner) {
					if replyToUserData.IsAdmin {
						return messages.ReplyMessage(c, "Ты не можешь банить других админов, соси писос", messageThreadID)
					}
					user := c.Message().ReplyTo.Sender
					chatMember := &tele.ChatMember{User: user, Role: tele.Member}
					admins.BanUser(bot, c.Message().Chat, chatMember, sync)
					bot.Delete(c.Message().ReplyTo)
					return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика", replyToAppeal), messageThreadID)
				} else {
					return messages.ReplyMessage(c, "Банхаммер готов. Кого послать нахуй?", messageThreadID)
				}
			case "Мут", "мут", "Ебало завали", "ебало завали", "/mute":
				if isReply && (userData.IsAdmin || !userData.IsWinner) {
					if replyToUserData.IsAdmin {
						return messages.ReplyMessage(c, "Ты не можешь мутить других админов, соси писос", messageThreadID)
					}
					user := c.Message().ReplyTo.Sender
					chatMember := &tele.ChatMember{User: user, Role: tele.Member, Rights: tele.Rights{
						CanSendMessages: false,
					}}
					admins.MuteUser(bot, c.Chat(), chatMember, sync)
					return messages.ReplyMessage(c, fmt.Sprintf("%s помолчит полчасика и подумает о своем поведении", replyToAppeal), messageThreadID)
				} else {
					return messages.ReplyMessage(c, "Кого мутить?", messageThreadID)
				}
			case "Размут", "размут", "/unmute":
				if isReply {
					chatMember := &tele.ChatMember{User: c.Message().ReplyTo.Sender, Role: tele.Member, Rights: tele.Rights{
						CanSendMessages: true,
					}}
					admins.UnmuteUser(bot, c.Chat(), chatMember, sync)
					return messages.ReplyMessage(c, fmt.Sprintf("%s размучен. А то че как воды в рот набрал", replyToAppeal), messageThreadID)
				} else {
					return messages.ReplyMessage(c, "Кого размутить?", messageThreadID)
				}
			case "Нацик", "нацик", "НАЦИК":
				if isReply && (userData.IsAdmin || !userData.IsWinner) {
					if replyToUserData.IsAdmin {
						return messages.ReplyMessage(c, "Ты не можешь банить других админов, соси писос", messageThreadID)
					}
					user := c.Message().ReplyTo.Sender
					messages.ReplyToOriginalMessage(c, fmt.Sprintf("%s, скажи ауфидерзейн своим нацистским яйцам!", replyToAppeal), messageThreadID)
					time.Sleep(1 * time.Second)
					chatMember := &tele.ChatMember{User: user, Role: tele.Member}
					admins.BanUser(bot, c.Message().Chat, chatMember, sync)
					bot.Delete(c.Message().ReplyTo)
					return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика", replyToAppeal), messageThreadID)
				} else {
					return messages.ReplyMessage(c, "Кому яйца жмут?", messageThreadID)
				}
			}
		}
		switch c.Message().Text {
		case "Инфа", "инфа", "/info":
			text := textcases.GetInfo()
			return messages.ReplyMessage(c, text, messageThreadID)
		case "Админ", "админ", "/report":
			log.Printf("Got an admin command from %d", userData.UserID)
			if isReply {
				return messages.ReplyToOriginalMessage(c, textcases.GetAdminsCommand(appeal, adminsUsernames), messageThreadID)
			} else {
				return messages.ReplyMessage(c, textcases.GetAdminsCommand(appeal, adminsUsernames), messageThreadID)
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
			log.Printf("Quiz running: %v", quizRunning)
			log.Print(c.Message().Text)
			log.Print(todayQuiz.SongName)
			if strings.EqualFold(c.Message().Text, todayQuiz.SongName) {
				quizRunning = false
				quizAlreadyWas = true
				pgRep.SetQuizAlreadyWas()
				winnerTitle := textcases.GetRandomTitle()
				messages.ReplyMessage(c, fmt.Sprintf("Правильно! Песня: %s", todayQuiz.SongName), messageThreadID)
				time.Sleep(100 * time.Millisecond)
				messages.ReplyMessage(c, fmt.Sprintf("Поздравляем, %s! Ты победил и получил титул %s до следующего квиза!", appeal, winnerTitle), messageThreadID)
				chatMember := &tele.ChatMember{User: c.Message().Sender, Role: tele.Member}
				admins.SetPref(bot, c.Chat(), chatMember, winnerTitle, sync)
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

		userData, err := sync.GetUser(joinedUser.ID)
		if err != nil {
			log.Printf("Failed to get user data: %v", err)
			return nil
		}
		if userData.Username != joinedUser.Username || userData.FirstName != joinedUser.FirstName {
			userData.Username = joinedUser.Username
			userData.FirstName = joinedUser.FirstName
			if err := sync.SaveUser(&userData); err != nil {
				log.Printf("Failed to save persistent username update for joined user %d: %v", joinedUser.ID, err)
			}
		}

		appeal := "@" + joinedUser.Username
		if appeal == "@" {
			appeal = joinedUser.FirstName
		}

		return messages.ReplyMessage(c, fmt.Sprintf(`Добро пожаловать, %s! Ты присоединился к чатику братства нежити. Напиши команду "Инфа", чтобы узнать, как тут все устроено`, appeal), c.Message().ThreadID)
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
