package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	redisClient "saxbot/redis"
	textcases "saxbot/text_cases"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v4"
)

var katyaFlag = false
var photoFlag = false

// sendMessage отправляет сообщение с учетом топика (если есть)
func sendMessage(c tele.Context, text string, threadID int) error {
	if threadID != 0 {
		log.Printf("Attempting to send message to thread %d: %s", threadID, text)

		// Попробуем несколько вариантов отправки

		// Вариант 1: С ThreadID
		opts := &tele.SendOptions{
			ThreadID: threadID,
		}
		_, err := c.Bot().Send(c.Chat(), text, opts)
		if err != nil {
			log.Printf("Method 1 failed (ThreadID): %v", err)

			// Вариант 2: Попробуем ответить на исходное сообщение (если это reply)
			if c.Message() != nil {
				replyOpts := &tele.SendOptions{
					ReplyTo: c.Message(),
				}
				_, err2 := c.Bot().Send(c.Chat(), text, replyOpts)
				if err2 == nil {
					log.Printf("Method 2 succeeded (ReplyTo)")
					return nil
				}
				log.Printf("Method 2 failed (ReplyTo): %v", err2)
			}

			// Вариант 3: Обычная отправка без параметров
			log.Printf("Fallback: sending without any special parameters")
			return c.Send(text)
		}
		log.Printf("Method 1 succeeded (ThreadID)")
		return err
	}
	// Обычная отправка
	return c.Send(text)
}

// replyToOriginalMessage отвечает на исходное сообщение (на которое отвечал админ)
func replyToOriginalMessage(c tele.Context, text string, threadID int) error {
	if !c.Message().IsReply() {
		// Если это не ответ, используем обычную отправку
		return sendMessage(c, text, threadID)
	}

	originalMessage := c.Message().ReplyTo
	if threadID != 0 {
		log.Printf("Attempting to reply to original message in thread %d: %s", threadID, text)

		// Попробуем несколько вариантов ответа на исходное сообщение

		// Вариант 1: С ThreadID и ReplyTo на исходное сообщение
		opts := &tele.SendOptions{
			ThreadID: threadID,
			ReplyTo:  originalMessage,
		}
		_, err := c.Bot().Send(c.Chat(), text, opts)
		if err != nil {
			log.Printf("Original reply method 1 failed (ThreadID+ReplyTo original): %v", err)

			// Вариант 2: Только ReplyTo на исходное сообщение, без ThreadID
			replyOpts := &tele.SendOptions{
				ReplyTo: originalMessage,
			}
			_, err2 := c.Bot().Send(c.Chat(), text, replyOpts)
			if err2 == nil {
				log.Printf("Original reply method 2 succeeded (ReplyTo original only)")
				return nil
			}
			log.Printf("Original reply method 2 failed (ReplyTo original only): %v", err2)

			// Вариант 3: Обычная отправка в тред
			log.Printf("Fallback: using sendMessage")
			return sendMessage(c, text, threadID)
		}
		log.Printf("Original reply method 1 succeeded (ThreadID+ReplyTo original)")
		return err
	}
	// Обычный ответ на исходное сообщение
	replyOpts := &tele.SendOptions{
		ReplyTo: originalMessage,
	}
	_, err := c.Bot().Send(c.Chat(), text, replyOpts)
	return err
}

// replyMessage отвечает на сообщение с учетом топика (если есть)
func replyMessage(c tele.Context, text string, threadID int) error {
	if threadID != 0 {
		log.Printf("Attempting to reply to thread %d: %s", threadID, text)

		// Попробуем несколько вариантов ответа

		// Вариант 1: С ThreadID и ReplyTo
		opts := &tele.SendOptions{
			ThreadID: threadID,
			ReplyTo:  c.Message(),
		}
		_, err := c.Bot().Send(c.Chat(), text, opts)
		if err != nil {
			log.Printf("Reply method 1 failed (ThreadID+ReplyTo): %v", err)

			// Вариант 2: Только ReplyTo, без ThreadID
			replyOpts := &tele.SendOptions{
				ReplyTo: c.Message(),
			}
			_, err2 := c.Bot().Send(c.Chat(), text, replyOpts)
			if err2 == nil {
				log.Printf("Reply method 2 succeeded (ReplyTo only)")
				return nil
			}
			log.Printf("Reply method 2 failed (ReplyTo only): %v", err2)

			// Вариант 3: Обычный ответ
			log.Printf("Fallback: using standard reply")
			return c.Reply(text)
		}
		log.Printf("Reply method 1 succeeded (ThreadID+ReplyTo)")
		return err
	}
	// Обычный ответ
	return c.Reply(text)
}

func main() {
	godotenv.Load()

	// Определяем флаги командной строки
	clearRedis := flag.Bool("clear-redis", false, "Очистить базу данных Redis при запуске")
	showInfo := flag.Bool("info", false, "Показать информацию о базе данных Redis и выйти")
	flag.Parse()

	// Получаем параметры подключения к Redis из переменных окружения
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
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

	// Показать информацию о базе данных
	if *showInfo {
		keys, err := redisClient.GetAllKeys()
		if err != nil {
			log.Fatalf("Ошибка получения ключей: %v", err)
		}

		fmt.Printf("Информация о Redis базе данных:\n")
		fmt.Printf("Всего ключей: %d\n", len(keys))

		if len(keys) > 0 {
			fmt.Printf("Ключи:\n")
			for i, key := range keys {
				if i >= 10 { // Показываем только первые 10 ключей
					fmt.Printf("   ... и еще %d ключей\n", len(keys)-10)
					break
				}
				fmt.Printf("   - %s\n", key)
			}
		} else {
			fmt.Printf("База данных пуста\n")
		}
		return
	}

	if *clearRedis {
		fmt.Printf("Очищаем базу данных Redis...\n")

		// Показываем что было до очистки
		keys, err := redisClient.GetAllKeys()
		if err == nil {
			fmt.Printf("Найдено ключей для удаления: %d\n", len(keys))
		}

		err = redisClient.FlushAll()
		if err != nil {
			log.Fatalf("Ошибка очистки Redis: %v", err)
		}

		fmt.Printf("База данных Redis очищена!\n")

		// Проверяем что действительно очистилось
		keys, err = redisClient.GetAllKeys()
		if err == nil {
			fmt.Printf("Ключей после очистки: %d\n", len(keys))
		}
	}

	// Обновляем админские статусы всех существующих пользователей при запуске
	log.Printf("Обновляем админские права пользователей из переменной окружения ADMINS...")
	err = redisClient.RefreshAllUsersAdminStatus()
	if err != nil {
		log.Printf("Предупреждение: не удалось обновить админские права: %v", err)
	}

	pref := tele.Settings{
		Token:  os.Getenv("BOT_TOKEN"),
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	go func() {
		for {
			time.Sleep(1 * time.Hour)
			katyaFlag = false
		}
	}()

	go func() {
		for {
			time.Sleep(30 * time.Second)
			photoFlag = false
		}
	}()

	// Горутина для периодической очистки истекших ключей из памяти
	go func() {
		for {
			time.Sleep(10 * time.Minute) // Очищаем каждые 10 минут
			err := redisClient.CleanupExpiredKeys()
			if err != nil {
				log.Printf("Error during cleanup: %v", err)
			}
		}
	}()

	// Проверяем обязательные переменные окружения
	allowedChats := os.Getenv("ALLOWED_CHATS")

	katyaID := os.Getenv("KATYA_ID")

	// Парсим разрешённые чаты
	allowedChatsSlice := strings.Split(allowedChats, ",")
	var allowedChatsInts []int64
	for i, s := range allowedChatsSlice {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		chatID, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			log.Printf("Ошибка парсинга чата #%d '%s': %v", i+1, s, err)
			continue
		}
		allowedChatsInts = append(allowedChatsInts, chatID)
	}

	// Парсим ID Кати
	katyaIDInt, _ := strconv.ParseInt(strings.TrimSpace(katyaID), 10, 64)

	bot.Handle(tele.OnText, func(c tele.Context) error {
		log.Printf("Received message: '%s' from user %d in chat %d", c.Message().Text, c.Message().Sender.ID, c.Message().Chat.ID)

		if !slices.Contains(allowedChatsInts, c.Message().Chat.ID) {
			log.Printf("Получил сообщение в чат %d. Ожидаются чаты %v", c.Message().Chat.ID, allowedChatsInts)
			return nil
		}

		// Проверяем, является ли чат форумом с топиками
		var messageThreadID int
		message := c.Message()

		// Детальное логирование для отладки
		log.Printf("Message details: ThreadID=%d, Chat.Type=%s, Chat.ID=%d",
			message.ThreadID, message.Chat.Type, message.Chat.ID)

		if message.ThreadID != 0 {
			messageThreadID = message.ThreadID
			log.Printf("Message is in thread %d", messageThreadID)
		} else if message.Chat.Type == tele.ChatSuperGroup {
			// Для супергрупп с топиками может потребоваться другой подход
			log.Printf("SuperGroup chat detected, checking for forum topics")
		}

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
			return sendMessage(c, fmt.Sprintf("@%s, тебя разбанили, но это можно исправить. Веди себя хорошо", userData.Username), messageThreadID)
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

		if userID == katyaIDInt && !katyaFlag {
			katyaFlag = true
			return replyMessage(c, "🚨ВНИМАНИЕ! АЛАРМ!🚨 КАТЕНЬКА В ЧАТЕ!💀 ЭТО НЕ УЧЕБНАЯ ТРЕВОГА! ПОВТОРЯЮ, ЭТО НЕ УЧЕБНАЯ ТРЕВОГА!⛔\n❗ВСЕМ ОБЯЗАТЕЛЬНО СЛУШАТЬСЯ КАТЕНЬКУ❗", messageThreadID)
		}

		if userData.IsAdmin || userID == katyaIDInt {
			switch c.Message().Text {
			case "Предупреждение", "предупреждение":
				if isReply {
					replyToUserData.Warns++
					redisClient.SetUser(replyToID, replyToUserData)
					redisClient.SetUserPersistent(replyToID, replyToUserData)
					text := textcases.GetWarnCase(c.Message().ReplyTo.Sender.Username)
					return replyToOriginalMessage(c, text, messageThreadID)
				} else {
					return replyMessage(c, "Ты кого предупреждаешь?", messageThreadID)
				}
			case "Извинись", "извинись", "ИЗВИНИСЬ":
				if isReply {
					return replyToOriginalMessage(c, "Извинись дон. Скажи, что ты был не прав дон. Или имей в виду — на всю оставшуюся жизнь у нас с тобой вражда", messageThreadID)
				}
			case "Пошел нахуй", "пошел нахуй", "Пошла нахуй", "пошла нахуй", "/ban":
				if isReply && userID != katyaIDInt {
					if replyToUserData.IsAdmin {
						return replyMessage(c, "Ты не можешь банить других админов, соси писос", messageThreadID)
					}
					user := c.Message().ReplyTo.Sender
					chatMember := &tele.ChatMember{User: user, Role: tele.Member}
					bot.Ban(c.Message().Chat, chatMember)
					bot.Delete(c.Message().ReplyTo)
					replyToUserData.Status = "banned"
					redisClient.SetUser(replyToID, replyToUserData)
					redisClient.SetUserPersistent(replyToID, replyToUserData)
					return sendMessage(c, fmt.Sprintf("@%s идет нахуй из чатика", user.Username), messageThreadID)
				} else {
					if userID == katyaIDInt {
						return replyMessage(c, "Катенька, зачиллься, остынь, успокойся, не надо так", messageThreadID)
					}
					return replyMessage(c, "Банхаммер готов. Кого послать нахуй?", messageThreadID)
				}
			case "Мут", "мут", "Ебало завали", "ебало завали", "/mute":
				if isReply && userID != katyaIDInt {
					if replyToUserData.IsAdmin {
						return replyMessage(c, "Ты не можешь мутить других админов, соси писос", messageThreadID)
					}
					replyToUserData.Status = "muted"
					redisClient.SetUser(replyToID, replyToUserData)
					redisClient.SetUserPersistent(replyToID, replyToUserData)
					go func() {
						time.Sleep(30 * time.Minute)
						replyToUserData.Status = "active"
						redisClient.SetUser(replyToID, replyToUserData)
						redisClient.SetUserPersistent(replyToID, replyToUserData)
					}()
					return sendMessage(c, fmt.Sprintf("@%s помолчит полчасика и подумает о своем поведении", replyToUserData.Username), messageThreadID)
				} else {
					if userID == katyaIDInt {
						return replyMessage(c, "Катенька, зачиллься, остынь, успокойся, не надо так", messageThreadID)
					}
					return replyMessage(c, "Кого мутить?", messageThreadID)
				}
			case "Размут", "размут", "/unmute":
				if isReply {
					replyToUserData.Status = "active"
					redisClient.SetUser(replyToID, replyToUserData)
					redisClient.SetUserPersistent(replyToID, replyToUserData)
					return sendMessage(c, fmt.Sprintf("@%s размучен. А то че как воды в рот набрал", replyToUserData.Username), messageThreadID)
				} else {
					return replyMessage(c, "Кого размутить?", messageThreadID)
				}
			case "Нацик":
				if isReply && userID != katyaIDInt {
					if replyToUserData.IsAdmin {
						return replyMessage(c, "Ты не можешь банить других админов, соси писос", messageThreadID)
					}
					user := c.Message().ReplyTo.Sender
					replyToOriginalMessage(c, fmt.Sprintf("@%s, скажи ауфидерзейн своим нацистским яйцам!", user.Username), messageThreadID)
					time.Sleep(1 * time.Second)
					chatMember := &tele.ChatMember{User: user, Role: tele.Member}
					bot.Ban(c.Message().Chat, chatMember)
					bot.Delete(c.Message().ReplyTo)
					replyToUserData.Status = "banned"
					redisClient.SetUser(replyToID, replyToUserData)
					redisClient.SetUserPersistent(replyToID, replyToUserData)
					return sendMessage(c, fmt.Sprintf("@%s идет нахуй из чатика", user.Username), messageThreadID)
				} else {
					if userID == katyaIDInt {
						return replyMessage(c, "Катенька, зачиллься, остынь, успокойся, не надо так", messageThreadID)
					}
					return replyMessage(c, "Кому яйца жмут?", messageThreadID)
				}
			}
		}
		switch c.Message().Text {
		case "Инфа", "инфа", "/info":
			text := textcases.GetInfo()
			return sendMessage(c, text, messageThreadID)
		case "Админ", "админ", "/report":
			if isReply {
				return replyToOriginalMessage(c, fmt.Sprintf("@%s вызывает админов. В чатике дичь\n@fatiurs, @puwyb, @murmuIlya, @OlegIksha", userData.Username), messageThreadID)
			} else {
				return sendMessage(c, fmt.Sprintf("@%s вызывает админов. В чатике дичь\n@fatiurs, @puwyb, @murmuIlya, @OlegIksha", userData.Username), messageThreadID)
			}
			/*
				case "Репа", "репа", "/rep":
					switch {
					case userData.Reputation == 0:
						return replyMessage(c, "У тебя нет репутации. Ты новенький, но скоро нежить о тебе услышит", messageThreadID)
					case userData.Reputation > 0 && userData.Reputation < 10:
						return replyMessage(c, fmt.Sprintf("У тебя %d репутации. Ты уже начал свой путь по кладбищу", userData.Reputation), messageThreadID)
					case userData.Reputation >= 10 && userData.Reputation < 100:
						return replyMessage(c, fmt.Sprintf("У тебя %d репутации. Ты уважаемый член кладбищенской братии", userData.Reputation), messageThreadID)
					case userData.Reputation >= 100:
						return replyMessage(c, fmt.Sprintf("У тебя %d репутации. Тобой гордится вся нежить!", userData.Reputation), messageThreadID)
					case userData.Reputation < 0 && userData.Reputation > -10:
						return replyMessage(c, fmt.Sprintf("У тебя %d репутации. Нежить относится к тебе с подозрением, но ты еще можешь исправить ситуацию", userData.Reputation), messageThreadID)
					case userData.Reputation <= -10 && userData.Reputation > -100:
						return replyMessage(c, fmt.Sprintf("У тебя %d репутации. Таких на нашем кладбище не уважают. Срочно делай что-нибудь", userData.Reputation), messageThreadID)
					case userData.Reputation <= -100:
						return replyMessage(c, fmt.Sprintf("У тебя %d репутации. Ты вообще не нежить, ты либерал простой", userData.Reputation), messageThreadID)
					}
			*/
		case "Преды", "преды", "/warns":
			switch {
			case userData.Warns == 0:
				return replyMessage(c, "Тебя ещё не предупреждали? Срочно предупредите его!", messageThreadID)
			case userData.Warns > 0 && userData.Warns < 10:
				return replyMessage(c, fmt.Sprintf("У тебя %d предупреждений. Помни, предупрежден — значит предупрежден", userData.Warns), messageThreadID)
			case userData.Warns >= 10 && userData.Warns < 100:
				return replyMessage(c, fmt.Sprintf("У тебя %d предупреждений. Этот парень совсем слов не понимает?", userData.Warns), messageThreadID)
			case userData.Warns >= 100 && userData.Warns < 1000:
				return replyMessage(c, fmt.Sprintf("У тебя %d предупреждений. Я от тебя в светлом ахуе. Ты когда-нибудь перестанешь?", userData.Warns), messageThreadID)
			case userData.Warns >= 1000:
				return replyMessage(c, fmt.Sprintf("У тебя %d предупреждений. Ты постиг нирвану и вышел за пределы сознания. Тебя больше ничто не остановит", userData.Warns), messageThreadID)
			}
		}
		return nil
	})

	bot.Handle(tele.OnUserJoined, func(c tele.Context) error {
		joinedUser := c.Message().UserJoined
		log.Printf("User %d joined chat %d", joinedUser.ID, c.Message().Chat.ID)

		if !slices.Contains(allowedChatsInts, c.Message().Chat.ID) {
			return nil
		}

		// Проверяем, является ли чат форумом с топиками
		var messageThreadID int
		if c.Message().ThreadID != 0 {
			messageThreadID = c.Message().ThreadID
			log.Printf("User joined in thread %d", messageThreadID)
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

		return sendMessage(c, fmt.Sprintf(`Добро пожаловать, @%s! Ты присоединился к чатику братства нежити. Напиши команду "Инфа", чтобы узнать, как тут все устроено`, userData.Username), messageThreadID)
	})

	bot.Handle(tele.OnPhoto, func(c tele.Context) error {
		log.Printf("Received photo from user %d in chat %d", c.Message().Sender.ID, c.Message().Chat.ID)

		if !slices.Contains(allowedChatsInts, c.Message().Chat.ID) {
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

		messageThreadID := c.Message().ThreadID
		userID := c.Message().Sender.ID

		if userID == katyaIDInt && !photoFlag {
			photoFlag = true
			return replyMessage(c, "💖 СРОЧНО ВСЕМ ЛЮБОВАТЬСЯ НОВОЙ ФОТОЧКОЙ КАТЕНЬКИ! 💖\n😠 ЗА НЕГАТИВНЫЕ РЕАКЦИИ ПОЛУЧИТЕ ПРЕДУПРЕЖДЕНИЕ! 😠", messageThreadID)
		}

		return nil
	})

	bot.Start()
}
