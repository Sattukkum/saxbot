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
		}

		if userData.Status == "muted" {
			bot.Delete(c.Message())
			return nil
		}

		if userData.Status == "banned" {
			userData.Status = "active"
			redisClient.SetUser(userID, userData)
			return c.Send(fmt.Sprintf("@%s, тебя разбанили, но это можно исправить. Веди себя хорошо", userData.Username))
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
			}
		}

		if userID == katyaIDInt && !katyaFlag {
			katyaFlag = true
			return c.Reply("🚨ВНИМАНИЕ! АЛАРМ!🚨 КАТЕНЬКА В ЧАТЕ!💀 ЭТО НЕ УЧЕБНАЯ ТРЕВОГА, ПОВТОРЯЮ, ЭТО НЕ УЧЕБНАЯ ТРЕВОГА!⛔")
		}

		if userData.IsAdmin {
			switch c.Message().Text {
			case "Предупреждение", "предупреждение":
				if isReply {
					replyToUserData.Warns++
					redisClient.SetUser(replyToID, replyToUserData)
					text := textcases.GetWarnCase(c.Message().ReplyTo.Sender.Username)
					return c.Send(text)
				} else {
					return c.Reply("Ты кого предупреждаешь?")
				}
			case "Извинись", "извинись", "ИЗВИНИСЬ":
				if isReply {
					return c.Send("Извинись дон. Скажи, что ты был не прав дон. Или имей в виду - на всю оставшуюся жизнь у нас с тобой вражда")
				}
			case "Пошел нахуй", "пошел нахуй", "Пошла нахуй", "пошла нахуй", "/ban":
				if isReply {
					if replyToUserData.IsAdmin {
						return c.Reply("Ты не можешь банить других админов, соси писос")
					}
					user := c.Message().ReplyTo.Sender
					chatMember := &tele.ChatMember{User: user, Role: tele.Member}
					bot.Ban(c.Message().Chat, chatMember)
					bot.Delete(c.Message().ReplyTo)
					replyToUserData.Status = "banned"
					redisClient.SetUser(replyToID, replyToUserData)
					return c.Send(fmt.Sprintf("@%s идет нахуй из чатика", user.Username))
				} else {
					return c.Reply("Банхаммер готов. Кого послать нахуй?")
				}
			case "Мут", "мут", "Ебало завали", "ебало завали", "/mute":
				if isReply {
					if replyToUserData.IsAdmin {
						return c.Reply("Ты не можешь мутить других админов, соси писос")
					}
					replyToUserData.Status = "muted"
					redisClient.SetUser(replyToID, replyToUserData)
					go func() {
						time.Sleep(30 * time.Minute)
						replyToUserData.Status = "active"
						redisClient.SetUser(replyToID, replyToUserData)
					}()
					return c.Send(fmt.Sprintf("@%s помолчит полчасика и подумает о своем поведении", replyToUserData.Username))
				} else {
					return c.Reply("Кого мутить?")
				}
			}
		}
		if isReply {
			switch c.Message().Text {
			case "+":
				redisClient.UpdateUserReputation(replyToID, 1)
				return c.Send(fmt.Sprintf("@%s повышает репутацию @%s на +1 (до %d)", userData.Username, replyToUserData.Username, replyToUserData.Reputation+1))
			case "-":
				redisClient.UpdateUserReputation(replyToID, -1)
				return c.Send(fmt.Sprintf("@%s понижает репутацию @%s на -1 (до %d)", userData.Username, replyToUserData.Username, replyToUserData.Reputation-1))
			}
		}
		switch c.Message().Text {
		case "Инфа", "инфа", "/info":
			text := textcases.GetInfo()
			return c.Send(text)
		case "Репа", "репа", "/rep":
			switch {
			case userData.Reputation == 0:
				return c.Reply("У тебя нет репутации. Ты новенький, но скоро нежить о тебе услышит")
			case userData.Reputation > 0 && userData.Reputation < 10:
				return c.Reply(fmt.Sprintf("У тебя %d репутации. Ты уже начал свой путь по кладбищу", userData.Reputation))
			case userData.Reputation >= 10 && userData.Reputation < 100:
				return c.Reply(fmt.Sprintf("У тебя %d репутации. Ты уважаемый член кладбищенской братии", userData.Reputation))
			case userData.Reputation >= 100:
				return c.Reply(fmt.Sprintf("У тебя %d репутации. Тобой гордится вся нежить!", userData.Reputation))
			case userData.Reputation < 0 && userData.Reputation > -10:
				return c.Reply(fmt.Sprintf("У тебя %d репутации. Нежить относится к тебе с подозрением, но ты еще можешь исправить ситуацию", userData.Reputation))
			case userData.Reputation <= -10 && userData.Reputation > -100:
				return c.Reply(fmt.Sprintf("У тебя %d репутации. Таких на нашем кладбище не уважают. Срочно делай что-нибудь", userData.Reputation))
			case userData.Reputation <= -100:
				return c.Reply(fmt.Sprintf("У тебя %d репутации. Ты вообще не нежить, ты либерал простой", userData.Reputation))
			}
		case "Преды", "преды", "/warns":
			switch {
			case userData.Warns == 0:
				return c.Reply("Тебя ещё не предупреждали? Срочно предупредите его!")
			case userData.Warns > 0 && userData.Warns < 10:
				return c.Reply(fmt.Sprintf("У тебя %d предупреждений. Помни, предупрежден - значит предупрежден", userData.Warns))
			case userData.Warns >= 10 && userData.Warns < 100:
				return c.Reply(fmt.Sprintf("У тебя %d предупреждений. Этот парень совсем слов не понимает?", userData.Warns))
			case userData.Warns >= 100 && userData.Warns < 1000:
				return c.Reply(fmt.Sprintf("У тебя %d предупреждений. Я от тебя в светлом ахуе. Ты когда-нибудь перестанешь?", userData.Warns))
			case userData.Warns >= 1000:
				return c.Reply(fmt.Sprintf("У тебя %d предупреждений. Ты постиг нирвану и вышел за пределы сознания. Тебя больше ничто не остановит", userData.Warns))
			}
		}
		return nil
	})

	bot.Handle(tele.OnUserJoined, func(c tele.Context) error {
		log.Printf("User %d joined chat %d", c.Message().Sender.ID, c.Message().Chat.ID)

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
		}

		return c.Send(fmt.Sprintf(`Добро пожаловать, @%s! Ты присоединился к чатику братству нежити. Напиши в чатик команду "Инфа", чтобы узнать, как тут все устроено`, userData.Username))
	})

	bot.Start()
}
