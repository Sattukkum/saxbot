package handlers

import (
	"fmt"
	"saxbot/activities"
	"saxbot/messages"
	"strconv"
	"strings"

	tele "gopkg.in/telebot.v4"
)

func handleAdminChatMessage(c tele.Context, chatMessageHandler *ChatMessageHandler, isWinnerOnly bool) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	text := strings.ToLower(chatMsg.Text())

	switch text {
	case "инфа", "/info":
		return handleInfo(c, chatMessageHandler)
	case "админ", "/report":
		return handleReport(c, chatMessageHandler)
	case "преды", "/warns":
		return handleWarns(c, chatMessageHandler)
	}

	// Победитель (не админ) может использовать только "предупреждение" и "извинись"
	if isWinnerOnly {
		switch text {
		case "предупреждение":
			return handleWarn(c, chatMessageHandler)
		case "извинись":
			return handleApologize(c, chatMessageHandler)
		default:
			// Для победителя другие команды недоступны, обрабатываем как обычное сообщение
			if chatMessageHandler.QuizManager.IsRunning() {
				activities.ManageRunningQuiz(chatMessageHandler.Rep, chatMessageHandler.Bot, chatMessageHandler.QuizManager, c, chatMsg.Appeal())
			}
			return nil
		}
	}

	// Обработка команд админов
	switch text {
	case "предупреждение":
		return handleWarn(c, chatMessageHandler)
	case "извинись":
		return handleApologize(c, chatMessageHandler)
	case "пошел нахуй", "пошла нахуй", "пошёл нахуй", "иди нахуй", "в бан", "/ban":
		// Только сеньоры могут банить
		if chatMsg.AdminRole() == "senior" {
			return handleBan(c, chatMessageHandler)
		}
	case "рестрикт", "кринж", "/restrict":
		// Только сеньоры могут рестриктить
		if chatMsg.AdminRole() == "senior" {
			return handleRestrict(c, chatMessageHandler)
		}
	case "размут", "/unmute":
		// Джуниоры и сеньоры могут размучивать
		adminRole := chatMsg.AdminRole()
		if adminRole == "junior" || adminRole == "senior" {
			return handleUnmute(c, chatMessageHandler)
		}
	case "нацик":
		// Только сеньоры могут использовать эту команду
		if chatMsg.AdminRole() == "senior" {
			return handleNazik(c, chatMessageHandler)
		}
	case "обезглавить", "обоссать", "сжечь":
		// Только сеньоры могут использовать эту команду
		if chatMsg.AdminRole() == "senior" {
			return handleDecapitate(c, chatMessageHandler)
		}
	}

	// Обработка команды мута (может содержать число)
	parts := strings.Fields(text)
	if len(parts) > 0 {
		prefix := parts[0]
		if prefix == "мут" || prefix == "ебало" || prefix == "/mute" {
			// Джуниоры и сеньоры могут мутить
			adminRole := chatMsg.AdminRole()
			if adminRole == "junior" || adminRole == "senior" {
				var durationMinutes uint = 30 // стандартное значение
				if len(parts) > 1 {
					lastPart := parts[len(parts)-1]
					lastPart = strings.Replace(lastPart, "-", "", 1)
					if mins, err := strconv.Atoi(lastPart); err == nil && mins > 0 {
						durationMinutes = uint(mins)
					} else {
						messages.ReplyMessage(c, "Нихрена не понял, на сколько мутить. Я фигану 30 минуток на всякий, в следующий раз выражайся понятнее", chatMsg.ThreadID())
					}
				}
				return handleMute(c, chatMessageHandler, durationMinutes)
			}
		}
	}

	// Если квиз запущен, обрабатываем ответы на квиз
	if chatMessageHandler.QuizManager.IsRunning() {
		activities.ManageRunningQuiz(chatMessageHandler.Rep, chatMessageHandler.Bot, chatMessageHandler.QuizManager, c, chatMsg.Appeal())
	}

	return nil
}

func handleAdminPrivateMessage(c tele.Context, chatMessageHandler *ChatMessageHandler, isWinnerOnly bool) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	text := strings.ToLower(chatMsg.Text())
	userID := chatMsg.UserData().UserID

	// Обработка команд в личных сообщениях
	switch text {
	case "/start", "меню", "/menu":
		return handleShowBirthdayMenu(c)
	case "/state":
		currentState := chatMessageHandler.GetUserState(userID)
		return messages.ReplyMessage(c, fmt.Sprintf("Текущее состояние: %s", currentState), chatMsg.ThreadID())
	}

	// Проверка на формат даты рождения (DD.MM.YYYY)
	if chatMessageHandler.GetUserState(userID) == "set_birthday" {
		if isBirthdayFormat(chatMsg.Text()) {
			return handleSaveBirthday(c, chatMessageHandler)
		} else {
			return messages.ReplyMessage(c, "Неверный формат даты. Пожалуйста, введите дату рождения в формате DD.MM.YYYY (например, 15.03.1990)", chatMsg.ThreadID())
		}
	}

	return nil
}
