package handlers

import (
	"fmt"
	"log"
	"saxbot/messages"
	"slices"
	"strings"

	tele "gopkg.in/telebot.v4"
)

func HandleChatMessage(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	log.Printf("Received message: '%s' from user %d in chat %d", c.Message().Text, c.Message().Sender.ID, c.Message().Chat.ID)

	if !slices.Contains(chatMessageHandler.AllowedChats, c.Message().Chat.ID) {
		log.Printf("Получил сообщение в чат %d. Ожидаются чаты %v", c.Message().Chat.ID, chatMessageHandler.AllowedChats)
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

	// Проверяем статус пользователя
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

	// TODO: Реализовать логику обработки личных сообщений
	// Например, можно добавить команды для работы с ботом в личке,
	// обработку обращений пользователей, статистику и т.д.

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

	appeal := "@" + joinedUser.Username
	if appeal == "@" {
		appeal = joinedUser.FirstName
	}

	return messages.ReplyMessage(c, fmt.Sprintf(`Добро пожаловать, %s! Ты присоединился к чатику братства нежити. Напиши команду "Инфа", чтобы узнать, как тут все устроено`, appeal), c.Message().ThreadID)
}

// HandleCallback обрабатывает колбэки от инлайн-кнопок
func HandleCallback(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	callback := c.Callback()
	if callback == nil {
		return fmt.Errorf("callback is nil")
	}

	log.Printf("Received callback: '%s' from user %d", callback.Data, callback.Sender.ID)

	callbackData := strings.TrimSpace(callback.Data)
	// Обработка колбэка для установки даты рождения
	if callbackData == "set_birthday" {
		userID := callback.Sender.ID
		chatMessageHandler.SetUserState(userID, "set_birthday")
		return handleBirthdayCallback(c)
	}

	return nil
}
