package handlers

import (
	"fmt"
	"log"
	"saxbot/activities"
	"saxbot/admins"
	"saxbot/database"
	"saxbot/messages"
	textcases "saxbot/text_cases"
	"slices"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"
)

type ChatMessageHandler struct {
	AllowedChats    []int64
	AdminsList      []int64
	AdminsUsernames []string
	QuizManager     *activities.QuizManager
	Rep             *database.PostgresRepository
	Bot             *tele.Bot
}

func HandleChatMessage(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	log.Printf("Received message: '%s' from user %d in chat %d", c.Message().Text, c.Message().Sender.ID, c.Message().Chat.ID)

	if !slices.Contains(chatMessageHandler.AllowedChats, c.Message().Chat.ID) {
		log.Printf("Получил сообщение в чат %d. Ожидаются чаты %v", c.Message().Chat.ID, chatMessageHandler.AllowedChats)
		return nil
	}

	messageThreadID := c.Message().ThreadID

	userID := c.Message().Sender.ID
	var chatID int64
	var chatAdmin = false
	if c.Message().SenderChat != nil {
		chatID = c.Message().SenderChat.ID
		log.Printf("!!! Сообщение от канала %d", chatID)
		if slices.Contains(chatMessageHandler.AdminsList, chatID) {
			chatAdmin = true
		}
	}
	isReply := c.Message().IsReply()
	appeal := "@" + c.Message().Sender.Username
	if appeal == "@" {
		appeal = c.Message().Sender.FirstName
	}
	var replyToID int64
	var replyToUserData database.User
	var replyToAppeal string

	if isReply {
		replyToID = c.Message().ReplyTo.Sender.ID
		replyToAppeal = "@" + c.Message().ReplyTo.Sender.Username
		if replyToAppeal == "@" {
			replyToAppeal = c.Message().ReplyTo.Sender.FirstName
		}
	}

	userData, err := chatMessageHandler.Rep.GetUser(userID)
	if err != nil {
		log.Printf("Failed to get user data: %v", err)
		return nil
	}
	if userData.Username != c.Message().Sender.Username || userData.FirstName != c.Message().Sender.FirstName {
		userData.Username = c.Message().Sender.Username
		userData.FirstName = c.Message().Sender.FirstName
		if err := chatMessageHandler.Rep.SaveUser(&userData); err != nil {
			log.Printf("Failed to save persistent username update for user %d: %v", userID, err)
		}
	}

	if userData.Status == "muted" {
		chatMessageHandler.Bot.Delete(c.Message())
		return nil
	}

	if userData.Status == "banned" {
		if c.Message().OriginalSender != nil || c.Message().OriginalChat != nil {
			log.Printf("Получено пересланное сообщение от забаненного пользователя %d, автоматический разбан не выполняется", userID)
			return nil
		}

		userData.Status = "active"
		if err := chatMessageHandler.Rep.SaveUser(&userData); err != nil {
			log.Printf("Failed to save persistent status update for user %d: %v", userID, err)
		}
		messages.ReplyMessage(c, fmt.Sprintf("%s, тебя разбанили, но это можно исправить. Веди себя хорошо", appeal), messageThreadID)
	}

	chatMessageHandler.Rep.UpdateUserMessageCount(userData.UserID, 1)

	if isReply {
		replyToUserData, err = chatMessageHandler.Rep.GetUser(replyToID)
		if err != nil {
			log.Printf("Failed to get reply to user data: %v", err)
			return nil
		}
		if replyToUserData.Username != c.Message().ReplyTo.Sender.Username {
			replyToUserData.Username = c.Message().ReplyTo.Sender.Username
			if err := chatMessageHandler.Rep.SaveUser(&replyToUserData); err != nil {
				log.Printf("Failed to save persistent username update for reply user %d: %v", replyToID, err)
			}
		}
	}

	replyToAdmin := false
	if isReply {
		replyToAdmin = chatMessageHandler.Rep.IsAdmin(replyToUserData.UserID)
	}
	var adminRole = "junior"
	isAdmin := chatMessageHandler.Rep.IsAdmin(userData.UserID)
	if isAdmin {
		adminRole, err = chatMessageHandler.Rep.GetAdminRole(userData.UserID)
		if err != nil {
			log.Printf("failed to get admin role, consider it junior")
			adminRole = "junior"
		}
	}
	if chatAdmin {
		adminRole = "senior"
	}

	isWinner := userData.UserID == chatMessageHandler.QuizManager.Winner()

	text := strings.ToLower(c.Message().Text)

	if isAdmin || isWinner || chatAdmin {
		switch text {
		case "предупреждение":
			if isReply {
				if err := chatMessageHandler.Rep.UpdateUserWarns(replyToID, 1); err != nil {
					log.Printf("Failed to save warns increase for user %d: %v", replyToID, err)
				} else {
					replyToUserData.Warns++
				}
				var text string
				if strings.EqualFold(c.Message().ReplyTo.Text, "Лена") || strings.EqualFold(c.Message().ReplyTo.Text, "Елена") || strings.EqualFold(c.Message().ReplyTo.Text, "Елена Вячеславовна") {
					text = textcases.GetWarnCase(replyToAppeal, true)
				} else {
					text = textcases.GetWarnCase(replyToAppeal, false)
				}
				return messages.ReplyToOriginalMessage(c, text, messageThreadID)
			} else {
				return messages.ReplyMessage(c, "Ты кого предупреждаешь?", messageThreadID)
			}
		case "извинись":
			if isReply {
				return messages.ReplyToOriginalMessage(c, "Извинись дон. Скажи, что ты был не прав дон. Или имей в виду — на всю оставшуюся жизнь у нас с тобой вражда", messageThreadID)
			}
		case "пошел нахуй", "пошла нахуй", "пошёл нахуй", "иди нахуй", "в бан", "/ban":
			if adminRole == "senior" {
				if isReply {
					if replyToAdmin {
						return messages.ReplyMessage(c, "Ты не можешь банить других админов, соси писос", messageThreadID)
					}
					user := c.Message().ReplyTo.Sender
					chatMember := &tele.ChatMember{User: user, Role: tele.Member}
					admins.BanUser(chatMessageHandler.Bot, c.Message().Chat, chatMember, chatMessageHandler.Rep)
					chatMessageHandler.Bot.Delete(c.Message().ReplyTo)
					return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика", replyToAppeal), messageThreadID)
				} else {
					return messages.ReplyMessage(c, "Банхаммер готов. Кого послать нахуй?", messageThreadID)
				}
			}
		case "рестрикт", "кринж", "/restrict":
			if adminRole == "senior" {
				if isReply {
					if replyToAdmin {
						return messages.ReplyMessage(c, "Ты не можешь рестриктить других админов, соси писос", messageThreadID)
					}
					user := c.Message().ReplyTo.Sender
					chatMember := &tele.ChatMember{User: user, Role: tele.Member}
					admins.RestrictUser(chatMessageHandler.Bot, c.Message().Chat, chatMember, chatMessageHandler.Rep)
					return messages.ReplyMessage(c, fmt.Sprintf("%s рестрикнут. Даже я словил кринж. А я бот ваще-то", replyToAppeal), messageThreadID)
				}
				return messages.ReplyMessage(c, "Кого рестриктить?", messageThreadID)
			}
		case "размут", "/unmute":
			if isReply {
				chatMember := &tele.ChatMember{User: c.Message().ReplyTo.Sender, Role: tele.Member, Rights: tele.Rights{
					CanSendMessages: true,
				}}
				admins.UnmuteUser(chatMessageHandler.Bot, c.Chat(), chatMember, chatMessageHandler.Rep)
				return messages.ReplyMessage(c, fmt.Sprintf("%s размучен. А то че как воды в рот набрал", replyToAppeal), messageThreadID)
			} else {
				return messages.ReplyMessage(c, "Кого размутить?", messageThreadID)
			}
		case "нацик":
			if adminRole == "senior" {
				if isReply {
					if replyToAdmin {
						return messages.ReplyMessage(c, "Ты не можешь банить других админов, соси писос", messageThreadID)
					}
					user := c.Message().ReplyTo.Sender
					messages.ReplyToOriginalMessage(c, fmt.Sprintf("%s, скажи ауфидерзейн своим нацистским яйцам!", replyToAppeal), messageThreadID)
					time.Sleep(1 * time.Second)
					chatMember := &tele.ChatMember{User: user, Role: tele.Member}
					admins.BanUser(chatMessageHandler.Bot, c.Message().Chat, chatMember, chatMessageHandler.Rep)
					chatMessageHandler.Bot.Delete(c.Message().ReplyTo)
					return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика", replyToAppeal), messageThreadID)
				} else {
					return messages.ReplyMessage(c, "Кому яйца жмут?", messageThreadID)
				}
			}
		case "обезглавить", "обоссать", "сжечь":
			if adminRole == "senior" {
				if isReply {
					if replyToAdmin {
						return messages.ReplyMessage(c, "Ты не можешь банить других админов, соси писос", messageThreadID)
					}
					user := c.Message().ReplyTo.Sender
					messages.ReplyToOriginalMessage(c, "ОБЕЗГЛАВИТЬ ОБОССАТЬ И СЖЕЧЬ!!!", messageThreadID)
					time.Sleep(1 * time.Second)
					chatMember := &tele.ChatMember{User: user, Role: tele.Member}
					admins.BanUser(chatMessageHandler.Bot, c.Message().Chat, chatMember, chatMessageHandler.Rep)
					chatMessageHandler.Bot.Delete(c.Message().ReplyTo)
					return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика. АВЕ АВЕ ПИРОМАН!", replyToAppeal), messageThreadID)
				} else {
					return messages.ReplyMessage(c, "Пироман готов!", messageThreadID)
				}
			}
		}
	}
	switch text {
	case "инфа", "/info":
		text := textcases.GetInfo()
		return messages.ReplyFormattedHTML(c, text, messageThreadID)
	case "админ", "/report":
		log.Printf("Got an admin command from %d", userData.UserID)
		if isReply {
			return messages.ReplyToOriginalMessage(c, textcases.GetAdminsCommand(appeal, chatMessageHandler.AdminsUsernames), messageThreadID)
		} else {
			return messages.ReplyMessage(c, textcases.GetAdminsCommand(appeal, chatMessageHandler.AdminsUsernames), messageThreadID)
		}
	case "преды", "/warns":
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

	var durationMinutes uint = 30 // стандартное значение
	var isMuteCommand bool = false
	parts := strings.Fields(text)

	if len(parts) > 0 {
		prefix := parts[0]

		if prefix == "мут" || prefix == "ебало" || prefix == "/mute" {
			// Проверяем, есть ли число в конце
			if len(parts) > 1 {
				lastPart := parts[len(parts)-1]
				lastPart = strings.Replace(lastPart, "-", "", 1)
				if mins, err := strconv.Atoi(lastPart); err == nil && mins > 0 {
					durationMinutes = uint(mins)
					isMuteCommand = true
				}
			}
		}
	}

	if isMuteCommand {
		if isReply && (isAdmin || chatAdmin) {
			if replyToAdmin {
				return messages.ReplyMessage(c, "Ты не можешь мутить других админов, соси писос", messageThreadID)
			}

			user := c.Message().ReplyTo.Sender
			chatMember := &tele.ChatMember{
				User: user,
				Role: tele.Member,
				Rights: tele.Rights{
					CanSendMessages: false,
				},
			}

			admins.MuteUser(chatMessageHandler.Bot, c.Chat(), chatMember, chatMessageHandler.Rep, durationMinutes)
			return messages.ReplyMessage(c, fmt.Sprintf("%s помолчит %d минут и подумает о своем поведении", replyToAppeal, durationMinutes), messageThreadID)
		} else {
			return messages.ReplyMessage(c, "Кого мутить?", messageThreadID)
		}
	}

	if chatMessageHandler.QuizManager.IsRunning() {
		activities.ManageRunningQuiz(chatMessageHandler.Rep, chatMessageHandler.Bot, chatMessageHandler.QuizManager, c, appeal)
	}
	return nil
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
