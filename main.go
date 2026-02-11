package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"saxbot/activities"
	"saxbot/admins"
	"saxbot/database"
	"saxbot/environment"
	"saxbot/handlers"
	"strconv"
	"strings"

	"time"

	"github.com/joho/godotenv"
	tele "gopkg.in/telebot.v4"
	"gorm.io/gorm"
)

var PostAllowed = true

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
			log.Fatalf("Миграция PostgreSQL не выполнена (таблицы users, channels, quizzes, admins, audios должны существовать): %v", err)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	postGate := make(chan struct{}, 1)
	postGate <- struct{}{} // первый пост сразу
	postDone := make(chan struct{})

	go StartPostCooldown(ctx, postGate, postDone, 20*time.Minute)

	go activities.ManageQuiz(rep, bot, quizManager, postGate, postDone)

	// Даем секунду менеджеру квизов, чтоб определить при перезапуске бота, идет квиз или нет
	time.Sleep(time.Second)

	// Управление объявлениями
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	go activities.ManageAds(bot, r, quizManager, postGate, postDone)

	// Управление поздравлениями
	go activities.ManageCongratulations(bot, rep, quizManager, postGate, postDone)

	// Управление "треком дня"
	go activities.ManageTrackOfTheDay(bot, quizManager, rep, postGate, postDone)

	chat := &tele.Chat{ID: quizChatID}

	// Размут пользователей по таймеру
	go func() {
		for {
			admins.UnmuteUsersByTime(bot, chat, rep)
			time.Sleep(time.Minute)
		}
	}()

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

	bot.Handle(tele.OnAudio, func(c tele.Context) error {
		if c.Sender().ID != 979772599 {
			return nil
		}
		audio := c.Message().Audio
		text := c.Message().Caption
		if text == "" {
			return c.Reply("Текст не найден")
		}
		parts := strings.Split(text, "\n")
		if len(parts) != 4 {
			return c.Reply("Неверный формат текста")
		}
		albumID, err := strconv.Atoi(parts[0])
		if err != nil {
			return c.Reply(fmt.Sprintf("Неверный формат альбома: %v", err))
		}
		trackNumber, err := strconv.Atoi(parts[1])
		if err != nil {
			return c.Reply(fmt.Sprintf("Неверный формат трека: %v", err))
		}
		name := parts[2]
		description := parts[3]
		audioData := &database.Audio{
			AlbumID:     albumID,
			TrackNumber: trackNumber,
			Name:        name,
			Description: description,
			FileID:      audio.File.FileID,
			UniqueID:    audio.File.UniqueID,
		}
		err = rep.SaveAudio(audioData)
		if err != nil {
			return c.Reply(fmt.Sprintf("Ошибка при сохранении трека: %v", err))
		}
		return c.Reply("Трек сохранен")
	})

	bot.Handle(tele.OnChannelPost, func(c tele.Context) error {
		return handlers.HandleChannelPost(c, &chatMessageHandler)
	})

	bot.Start()
}

func StartPostCooldown(
	ctx context.Context,
	gate chan struct{},
	postDone chan struct{},
	cooldown time.Duration,
) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-postDone:
			// пост сделан → ждём cooldown и снова выдаём токен
			go func() {
				timer := time.NewTimer(cooldown)
				defer timer.Stop()

				<-timer.C
				gate <- struct{}{}
			}()
		}
	}
}
