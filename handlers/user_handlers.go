package handlers

import (
	"fmt"
	"saxbot/activities"
	"saxbot/messages"
	"strings"

	tele "gopkg.in/telebot.v4"
)

func handleUserChatMessage(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	text := strings.ToLower(chatMsg.Text())

	// Обработка команд пользователей
	switch text {
	case "инфа", "/info":
		return handleInfo(c, chatMessageHandler)
	case "админ", "/report":
		return handleReport(c, chatMessageHandler)
	case "преды", "/warns":
		return handleWarns(c, chatMessageHandler)
	}

	// Проверка на формат даты рождения (DD.MM.YYYY)
	if isBirthdayFormat(chatMsg.Text()) {
		return handleSaveBirthday(c, chatMessageHandler)
	}

	// Если квиз запущен, обрабатываем ответы на квиз
	if chatMessageHandler.QuizManager.IsRunning() {
		activities.ManageRunningQuiz(chatMessageHandler.Rep, chatMessageHandler.Bot, chatMessageHandler.QuizManager, c, chatMsg.Appeal())
	}

	return nil
}

func handleUserPrivateMessage(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	text := strings.ToLower(chatMsg.Text())

	// Обработка команд в личных сообщениях
	switch text {
	case "/start", "меню", "/menu":
		return handleShowBirthdayMenu(c)
	case "/state":
		return messages.ReplyMessage(c, fmt.Sprintf("Текущее состояние: %s", chatMessageHandler.CurrentState), chatMsg.ThreadID())
	}

	// Проверка на формат даты рождения (DD.MM.YYYY)
	if chatMessageHandler.CurrentState == "set_birthday" {
		if isBirthdayFormat(chatMsg.Text()) {
			return handleSaveBirthday(c, chatMessageHandler)
		} else {
			return messages.ReplyMessage(c, "Неверный формат даты. Пожалуйста, введите дату рождения в формате DD.MM.YYYY (например, 15.03.1990)", chatMsg.ThreadID())
		}
	}

	return nil
}
