package handlers

import (
	"fmt"
	"saxbot/activities"
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

	// Если квиз запущен, обрабатываем ответы на квиз
	if chatMessageHandler.QuizManager.IsRunning() {
		activities.ManageRunningQuiz(chatMessageHandler.Rep, chatMessageHandler.Bot, chatMessageHandler.QuizManager, c, chatMsg.Appeal())
	}

	return nil
}
