package handlers

import (
	"fmt"
	"log"
	"saxbot/admins"
	"saxbot/database"
	"saxbot/messages"
	textcases "saxbot/text_cases"
	"slices"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"
)

func HandleChatMessage(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	msg := c.Message()
	var logMsg string
	if msg.SenderChat != nil {
		logMsg = fmt.Sprintf("Received message: '%s' from channel %d in chat %d", msg.Text, msg.SenderChat.ID, msg.Chat.ID)
	} else {
		logMsg = fmt.Sprintf("Received message: '%s' from user %d in chat %d", msg.Text, msg.Sender.ID, msg.Chat.ID)
	}
	log.Println(logMsg)

	if !slices.Contains(chatMessageHandler.AllowedChats, msg.Chat.ID) {
		log.Printf("Получил сообщение в чат %d. Ожидаются чаты %v", msg.Chat.ID, chatMessageHandler.AllowedChats)
		return nil
	}

	// Инициализируем структуру ChatMessage
	chatMessage, err := initChatMessage(c, chatMessageHandler)
	if err != nil {
		log.Printf("Failed to initialize chat message: %v", err)
		return nil
	}

	// Сохраняем ссылку на ChatMessage в handler для использования в других функциях
	chatMessageHandler.ChatMessage = chatMessage

	// Обрабатываем сообщения от каналов отдельно
	if chatMessage.IsFromChannel() {
		return handleChannelChatMessage(c, chatMessageHandler)
	}

	// Обработка сообщений от пользователей
	userData := chatMessage.UserData()
	if userData == nil {
		log.Printf("UserData is nil, skipping message processing")
		return nil
	}

	if userData.Status == "muted" {
		chatMessageHandler.Bot.Delete(c.Message())
		return nil
	}

	if userData.Status == "banned" {
		if c.Message().OriginalSender != nil || c.Message().OriginalChat != nil {
			log.Printf("Получено пересланное сообщение от забаненного пользователя %d, автоматический разбан не выполняется", chatMessage.Sender().ID)
			return nil
		}

		userData.Status = "active"
		if err := chatMessageHandler.Rep.SaveUser(userData); err != nil {
			log.Printf("Failed to save persistent status update for user %d: %v", chatMessage.Sender().ID, err)
		}
		messages.ReplyMessage(c, fmt.Sprintf("%s, тебя разбанили, но это можно исправить. Веди себя хорошо", chatMessage.Appeal()), chatMessage.ThreadID())
	}

	// Обновляем счетчик сообщений
	chatMessageHandler.Rep.UpdateUserMessageCount(userData.UserID, 1)

	// Определяем, является ли отправитель админом или победителем
	isAdmin := chatMessageHandler.Rep.IsAdmin(userData.UserID)
	isWinnerOnly := chatMessage.IsWinner() && !isAdmin && !chatMessage.ChatAdmin()
	canUseAdminCommands := isAdmin || chatMessage.IsWinner() || chatMessage.ChatAdmin()

	// Маршрутизируем в соответствующий обработчик
	if canUseAdminCommands {
		return handleAdminChatMessage(c, chatMessageHandler, isWinnerOnly)
	} else {
		return handleUserChatMessage(c, chatMessageHandler)
	}
}

// HandlePrivateMessage обрабатывает личные сообщения от пользователей
func HandlePrivateMessage(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	log.Printf("Received private message: '%s' from user %d", c.Message().Text, c.Message().Sender.ID)

	chatMessage, err := initPrivateMessage(c, chatMessageHandler)
	if err != nil {
		log.Printf("Failed to initialize chat message: %v", err)
		return nil
	}
	chatMessageHandler.ChatMessage = chatMessage

	// Проверяем статус пользователя
	userData := chatMessage.UserData()
	if userData == nil {
		log.Printf("UserData is nil, skipping message processing")
		return nil
	}

	// Определяем, является ли отправитель админом или победителем
	isAdmin := chatMessageHandler.Rep.IsAdmin(userData.UserID)
	isWinnerOnly := chatMessage.IsWinner() && !isAdmin && !chatMessage.ChatAdmin()
	canUseAdminCommands := isAdmin || chatMessage.IsWinner() || chatMessage.ChatAdmin()

	if canUseAdminCommands {
		return handleAdminPrivateMessage(c, chatMessageHandler, isWinnerOnly)
	} else {
		return handleUserPrivateMessage(c, chatMessageHandler)
	}
}

// handleChannelChatMessage обрабатывает сообщения от каналов в чатах
func handleChannelChatMessage(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}

	channelData := chatMsg.ChannelData()
	if channelData == nil {
		log.Printf("ChannelData is nil, skipping message processing")
		return nil
	}

	// Проверяем статус канала
	if channelData.Status == "muted" || channelData.Status == "banned" {
		chatMessageHandler.Bot.Delete(c.Message())
		return nil
	}

	// Каналы-админы могут использовать админские команды
	if chatMsg.ChatAdmin() {
		return handleAdminChatMessage(c, chatMessageHandler, false)
	}

	// Обычные каналы обрабатываются как пользовательские сообщения
	return handleUserChatMessage(c, chatMessageHandler)
}

func HandleUserJoined(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	joinedUser := c.Message().UserJoined
	log.Printf("User %d joined chat %d", joinedUser.ID, c.Message().Chat.ID)

	if !slices.Contains(chatMessageHandler.AllowedChats, c.Message().Chat.ID) {
		return nil
	}

	userData, err := chatMessageHandler.Rep.GetUser(joinedUser.ID)
	if err != nil {
		log.Printf("Failed to get user data: %v", err)
		return nil
	}
	if userData.Username != joinedUser.Username || userData.FirstName != joinedUser.FirstName {
		userData.Username = joinedUser.Username
		userData.FirstName = joinedUser.FirstName
		if err := chatMessageHandler.Rep.SaveUser(&userData); err != nil {
			log.Printf("Failed to save persistent username update for joined user %d: %v", joinedUser.ID, err)
		}
	}

	// Сохраняем оригинальный статус до изменений
	originalStatus := userData.Status

	appeal := "@" + joinedUser.Username
	if appeal == "@" {
		appeal = joinedUser.FirstName
	}

	// Проверяем, является ли пользователь новым (статус "active") или уже был замучен/рестриктнут/забанен ранее
	isNewUser := (originalStatus == "active" || originalStatus == "new_user")

	if isNewUser {
		// Мутим нового пользователя
		userData.Status = "new_user"
		if err := chatMessageHandler.Rep.SaveUser(&userData); err != nil {
			log.Printf("Failed to save new_user status for joined user %d: %v", joinedUser.ID, err)
		}

		// Ограничиваем права пользователя в чате
		chatMember := &tele.ChatMember{
			User: joinedUser,
			Role: tele.Member,
			Rights: tele.Rights{
				CanSendMessages: false,
			},
		}
		if err := chatMessageHandler.Bot.Restrict(c.Message().Chat, chatMember); err != nil {
			log.Printf("Failed to restrict user %d: %v", joinedUser.ID, err)
		}

		// Сохраняем State пользователя как нового пользователя
		chatMessageHandler.SetUserState(joinedUser.ID, "new_user")

		// Показываем кнопку для размута
		menu := &tele.ReplyMarkup{ResizeKeyboard: true}
		btnJoin := menu.Data("Я не бот!", "join")
		menu.Inline(menu.Row(btnJoin))
		opts := &tele.SendOptions{
			ReplyMarkup: menu,
			ThreadID:    c.Message().ThreadID,
		}

		go autokick(chatMessageHandler.Bot, c.Message().Chat, chatMember, chatMessageHandler.Rep)
		return c.Reply(textcases.GetUserJoinedMessage(appeal), opts)
	} else {
		// Пользователь был замучен/рестриктнут/забанен ранее
		// Применяем ограничения согласно текущему статусу
		chatMember := &tele.ChatMember{
			User: joinedUser,
			Role: tele.Member,
		}

		switch originalStatus {
		case "muted":
			// Ограничиваем отправку сообщений
			chatMember.Rights = tele.Rights{
				CanSendMessages: false,
			}
		case "restricted":
			// Ограничиваем отправку медиа
			chatMember.Rights = tele.Rights{
				CanSendMessages:  true,
				CanSendMedia:     false,
				CanSendAudios:    false,
				CanSendVideos:    false,
				CanSendPhotos:    false,
				CanSendDocuments: false,
				CanSendOther:     false,
			}
		case "banned":
			// Бан - не применяем ограничения через Restrict, так как пользователь забанен
			log.Printf("User %d is banned, skipping restrictions", joinedUser.ID)
			return nil
		default:
			// Для других статусов применяем стандартные ограничения
			chatMember.Rights = tele.Rights{
				CanSendMessages: false,
			}
		}

		if originalStatus != "banned" {
			if err := chatMessageHandler.Bot.Restrict(c.Message().Chat, chatMember); err != nil {
				log.Printf("Failed to restrict user %d with status %s: %v", joinedUser.ID, originalStatus, err)
			}
		}

		// НЕ устанавливаем состояние "new_user" и НЕ показываем кнопку
		log.Printf("User %d rejoined with status %s, not showing unmute button", joinedUser.ID, originalStatus)
		return nil
	}
}

// HandleCallback обрабатывает колбэки от инлайн-кнопок
func HandleCallback(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	callback := c.Callback()
	if callback == nil {
		return fmt.Errorf("callback is nil")
	}

	log.Printf("Received callback: '%s' from user %d", callback.Data, callback.Sender.ID)

	callbackData := strings.TrimSpace(callback.Data)

	// Обработка колбэка для подтверждения, что пользователь не бот
	switch callbackData {
	case "join":
		userID := callback.Sender.ID

		// Проверяем, что пользователь имеет состояние "new_user"
		if chatMessageHandler.GetUserState(userID) != "new_user" {
			return c.Respond(&tele.CallbackResponse{
				Text:      "Эта кнопка не для тебя!",
				ShowAlert: false,
			})
		}

		// Получаем данные пользователя
		userData, err := chatMessageHandler.Rep.GetUser(userID)
		if err != nil {
			log.Printf("Failed to get user data for unmute: %v", err)
			return c.Respond(&tele.CallbackResponse{
				Text:      "Ошибка при размуте. Обратитесь к админу.",
				ShowAlert: true,
			})
		}

		// Размучиваем пользователя
		userData.Status = "active"
		if err := chatMessageHandler.Rep.SaveUser(&userData); err != nil {
			log.Printf("Failed to save active status for user %d: %v", userID, err)
			return c.Respond(&tele.CallbackResponse{
				Text:      "Ошибка при сохранении статуса. Обратитесь к админу.",
				ShowAlert: true,
			})
		}

		// Восстанавливаем права пользователя в чате
		chatMember := &tele.ChatMember{
			User: callback.Sender,
			Role: tele.Member,
			Rights: tele.Rights{
				CanSendMessages:  true,
				CanSendMedia:     true,
				CanSendAudios:    true,
				CanSendVideos:    true,
				CanSendPhotos:    true,
				CanSendDocuments: true,
				CanSendOther:     true,
			},
		}
		if err := chatMessageHandler.Bot.Restrict(c.Chat(), chatMember); err != nil {
			log.Printf("Failed to unrestrict user %d: %v", userID, err)
		}

		// Удаляем состояние пользователя
		if chatMessageHandler.UserStates != nil {
			delete(chatMessageHandler.UserStates, userID)
		}

		// Удаляем кнопку из сообщения
		if callback.Message != nil {
			chatMessageHandler.Bot.EditReplyMarkup(callback.Message, nil)
		}

		return c.Respond(&tele.CallbackResponse{
			Text:      "Добро пожаловать! Теперь ты можешь писать в чат.",
			ShowAlert: false,
		})

	case "set_birthday":
		userID := callback.Sender.ID
		chatMessageHandler.SetUserState(userID, "set_birthday")
		return handleBirthdayCallback(c)

	case "show_muted":
		return handleMutedCallback(c, chatMessageHandler)

	case "show_restricted":
		return handleRestrictedCallback(c, chatMessageHandler)
	}

	return nil
}

func autokick(bot *tele.Bot, chat *tele.Chat, user *tele.ChatMember, db *database.PostgresRepository) error {
	time.Sleep(5 * time.Minute)
	userData, err := db.GetUser(user.User.ID)
	if err != nil {
		return fmt.Errorf("failed to get user %d: %w", user.User.ID, err)
	}
	if userData.Status == "new_user" {
		err = admins.KickUser(bot, chat, user)
		if err != nil {
			return fmt.Errorf("failed to kick user %d: %w", userData.UserID, err)
		}
	}
	return nil
}

func ManageRunningQuiz(c tele.Context, chatMessageHandler *ChatMessageHandler) {
	todayQuiz, quizRunning, _, _, _, _ := chatMessageHandler.QuizManager.GetState()
	log.Printf("Quiz running: %v", quizRunning)
	log.Println(c.Message().Text)
	log.Println(todayQuiz.SongName)
	if strings.EqualFold(c.Message().Text, todayQuiz.SongName) {
		if slices.Contains(chatMessageHandler.AdminsList, c.Message().Sender.ID) || chatMessageHandler.ChatMessage.chatAdmin {
			messages.ReplyMessage(c, "Ты и так уже админ, дружок-пирожок. Дай выиграть тем, кто пока ещё нет", c.Message().ThreadID)
			return
		}
		chatMessageHandler.QuizManager.SetQuizRunning(false)
		chatMessageHandler.QuizManager.SetQuizAlreadyWas(true)
		chatMessageHandler.Rep.SetQuizAlreadyWas()
		winnerTitle := textcases.GetRandomTitle()
		messages.ReplyMessage(c, fmt.Sprintf("Правильно! Песня: %s", todayQuiz.SongName), c.Message().ThreadID)
		time.Sleep(100 * time.Millisecond)
		messages.ReplyMessage(c, fmt.Sprintf("Поздравляем, %s! Ты победил и получил титул %s до следующего квиза!", chatMessageHandler.ChatMessage.appeal, winnerTitle), c.Message().ThreadID)
		chatMember := &tele.ChatMember{User: c.Message().Sender, Role: tele.Member}
		admins.SetPref(chatMessageHandler.Bot, c.Chat(), chatMember, winnerTitle)
		quiz, err := chatMessageHandler.Rep.GetLastCompletedQuiz()
		if err != nil {
			log.Printf("failed to get last completed quiz: %v", err)
		} else if quiz != nil {
			err = chatMessageHandler.Rep.SetQuizWinner(quiz.ID, c.Message().Sender.ID)
			if err != nil {
				log.Printf("failed to set user %d as a quiz winner %v", c.Message().Sender.ID, err)
			}
		}
		// Обновляем isLastQuizClip после завершения квиза для правильного чередования
		chatMessageHandler.QuizManager.SetIsLastQuizClip(todayQuiz.IsClip)
	}
}
