package main

import (
	"fmt"
	"log"
	"math/rand"
	"saxbot/activities"
	"saxbot/database"
	"saxbot/environment"
	"saxbot/handlers"

	"time"

	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v4"
	"gorm.io/gorm"
)

func main() {
	godotenv.Load()

	// Получение переменных окружения
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

	// Подключение к PostgreSQL
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
	quizManager := &activities.QuizManager{
		TodayQuiz:      activities.QuoteQuiz{},
		QuizRunning:    false,
		QuizAlreadyWas: false,
		WinnerID:       0,
		IsLastQuizClip: false,
		QuizChatID:     quizChatID,
	}
	go activities.ManageQuiz(rep, bot, quizManager)

	// Управление объявлениями
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	go activities.ManageAds(bot, quizChatID, r)

	// Управление поздравлениями
	go activities.ManageCongratulations(bot, quizChatID, rep)

	// Инициализация обработчика сообщений чата
	chatMessageHandler := handlers.ChatMessageHandler{
		AllowedChats:    allowedChats,
		AdminsList:      adminsList,
		AdminsUsernames: adminsUsernames,
		QuizManager:     quizManager,
		Rep:             rep,
		Bot:             bot,
		UserStates:      make(map[int64]string),
	}

	// Обработка текстовых сообщений
	bot.Handle(tele.OnText, func(c tele.Context) error {
		if c.Chat().Type == tele.ChatPrivate {
			return handlers.HandlePrivateMessage(c, &chatMessageHandler)
		}
		return handlers.HandleChatMessage(c, &chatMessageHandler)
	})

	// Обработка событий присоединения пользователей к чату
	bot.Handle(tele.OnUserJoined, func(c tele.Context) error {
		return handlers.HandleUserJoined(c, &chatMessageHandler)
	})

	// Обработка колбэков от инлайн-меню
	bot.Handle(tele.OnCallback, func(c tele.Context) error {
		return handlers.HandleCallback(c, &chatMessageHandler)
	})

	bot.Start()
}
