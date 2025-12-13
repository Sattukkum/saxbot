package handlers

import (
	"fmt"
	"log"
	"saxbot/admins"
	"saxbot/messages"
	textcases "saxbot/text_cases"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"
)

func handleWarn(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	if !chatMsg.IsReply() {
		return messages.ReplyMessage(c, "Ты кого предупреждаешь?", chatMsg.ThreadID())
	}

	replyToID := chatMsg.ReplyToID()
	if err := chatMessageHandler.Rep.UpdateUserWarns(replyToID, 1); err != nil {
		log.Printf("Failed to save warns increase for user %d: %v", replyToID, err)
	} else {
		replyToUserData := chatMsg.ReplyToUserData()
		if replyToUserData != nil {
			replyToUserData.Warns++
		}
	}

	var text string
	replyTo := chatMsg.ReplyTo()
	if replyTo != nil {
		replyText := strings.ToLower(replyTo.Text)
		if strings.EqualFold(replyText, "лена") || strings.EqualFold(replyText, "елена") || strings.EqualFold(replyText, "елена вячеславовна") {
			text = textcases.GetWarnCase(chatMsg.ReplyToAppeal(), true)
		} else {
			text = textcases.GetWarnCase(chatMsg.ReplyToAppeal(), false)
		}
	}
	return messages.ReplyToOriginalMessage(c, text, chatMsg.ThreadID())
}

func handleApologize(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	if !chatMsg.IsReply() {
		return nil
	}
	return messages.ReplyToOriginalMessage(c, "Извинись дон. Скажи, что ты был не прав дон. Или имей в виду — на всю оставшуюся жизнь у нас с тобой вражда", chatMsg.ThreadID())
}

func handleBan(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	if !chatMsg.IsReply() {
		return messages.ReplyMessage(c, "Банхаммер готов. Кого послать нахуй?", chatMsg.ThreadID())
	}

	if chatMsg.ReplyToAdmin() {
		return messages.ReplyMessage(c, "Ты не можешь банить других админов, соси писос", chatMsg.ThreadID())
	}

	replyTo := chatMsg.ReplyTo()
	if replyTo == nil || replyTo.Sender == nil {
		return fmt.Errorf("reply message or sender is nil")
	}

	user := replyTo.Sender
	chatMember := &tele.ChatMember{User: user, Role: tele.Member}
	admins.BanUser(chatMessageHandler.Bot, c.Message().Chat, chatMember, chatMessageHandler.Rep)
	chatMessageHandler.Bot.Delete(replyTo)
	return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
}

func handleRestrict(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	if !chatMsg.IsReply() {
		return messages.ReplyMessage(c, "Кого рестриктить?", chatMsg.ThreadID())
	}

	if chatMsg.ReplyToAdmin() {
		return messages.ReplyMessage(c, "Ты не можешь рестриктить других админов, соси писос", chatMsg.ThreadID())
	}

	replyTo := chatMsg.ReplyTo()
	if replyTo == nil || replyTo.Sender == nil {
		return fmt.Errorf("reply message or sender is nil")
	}

	user := replyTo.Sender
	chatMember := &tele.ChatMember{User: user, Role: tele.Member}
	if err := admins.RestrictUser(chatMessageHandler.Bot, c.Message().Chat, chatMember, chatMessageHandler.Rep); err != nil {
		log.Printf("Failed to restrict user: %v", err)
		return messages.ReplyMessage(c, "Не удалось рестриктить пользователя", chatMsg.ThreadID())
	}
	return messages.ReplyMessage(c, fmt.Sprintf("%s рестрикнут. Даже я словил кринж. А я бот ваще-то", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
}

func handleUnmute(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	if !chatMsg.IsReply() {
		return messages.ReplyMessage(c, "Кого размутить?", chatMsg.ThreadID())
	}

	replyTo := chatMsg.ReplyTo()
	if replyTo == nil || replyTo.Sender == nil {
		return fmt.Errorf("reply message or sender is nil")
	}

	chatMember := &tele.ChatMember{
		User: replyTo.Sender,
		Role: tele.Member,
		Rights: tele.Rights{
			CanSendMessages: true,
		},
	}
	admins.UnmuteUser(chatMessageHandler.Bot, c.Chat(), chatMember, chatMessageHandler.Rep)
	return messages.ReplyMessage(c, fmt.Sprintf("%s размучен. А то че как воды в рот набрал", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
}

func handleMute(c tele.Context, chatMessageHandler *ChatMessageHandler, durationMinutes uint) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	if !chatMsg.IsReply() {
		return messages.ReplyMessage(c, "Кого мутить?", chatMsg.ThreadID())
	}

	if chatMsg.ReplyToAdmin() {
		return messages.ReplyMessage(c, "Ты не можешь мутить других админов, соси писос", chatMsg.ThreadID())
	}

	replyTo := chatMsg.ReplyTo()
	if replyTo == nil || replyTo.Sender == nil {
		return fmt.Errorf("reply message or sender is nil")
	}

	user := replyTo.Sender
	chatMember := &tele.ChatMember{
		User: user,
		Role: tele.Member,
		Rights: tele.Rights{
			CanSendMessages: false,
		},
	}

	admins.MuteUser(chatMessageHandler.Bot, c.Chat(), chatMember, chatMessageHandler.Rep, durationMinutes)
	return messages.ReplyMessage(c, fmt.Sprintf("%s помолчит %d минут и подумает о своем поведении", chatMsg.ReplyToAppeal(), durationMinutes), chatMsg.ThreadID())
}

func handleNazik(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	if !chatMsg.IsReply() {
		return messages.ReplyMessage(c, "Кому яйца жмут?", chatMsg.ThreadID())
	}

	if chatMsg.ReplyToAdmin() {
		return messages.ReplyMessage(c, "Ты не можешь банить других админов, соси писос", chatMsg.ThreadID())
	}

	replyTo := chatMsg.ReplyTo()
	if replyTo == nil || replyTo.Sender == nil {
		return fmt.Errorf("reply message or sender is nil")
	}

	user := replyTo.Sender
	messages.ReplyToOriginalMessage(c, fmt.Sprintf("%s, скажи ауфидерзейн своим нацистским яйцам!", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
	time.Sleep(1 * time.Second)
	chatMember := &tele.ChatMember{User: user, Role: tele.Member}
	admins.BanUser(chatMessageHandler.Bot, c.Message().Chat, chatMember, chatMessageHandler.Rep)
	chatMessageHandler.Bot.Delete(replyTo)
	return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
}

func handleDecapitate(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	if !chatMsg.IsReply() {
		return messages.ReplyMessage(c, "Пироман готов!", chatMsg.ThreadID())
	}

	if chatMsg.ReplyToAdmin() {
		return messages.ReplyMessage(c, "Ты не можешь банить других админов, соси писос", chatMsg.ThreadID())
	}

	replyTo := chatMsg.ReplyTo()
	if replyTo == nil || replyTo.Sender == nil {
		return fmt.Errorf("reply message or sender is nil")
	}

	user := replyTo.Sender
	messages.ReplyToOriginalMessage(c, "ОБЕЗГЛАВИТЬ ОБОССАТЬ И СЖЕЧЬ!!!", chatMsg.ThreadID())
	time.Sleep(1 * time.Second)
	chatMember := &tele.ChatMember{User: user, Role: tele.Member}
	admins.BanUser(chatMessageHandler.Bot, c.Message().Chat, chatMember, chatMessageHandler.Rep)
	chatMessageHandler.Bot.Delete(replyTo)
	return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика. АВЕ АВЕ ПИРОМАН!", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
}

func handleInfo(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	text := textcases.GetInfo()
	return messages.ReplyFormattedHTML(c, text, chatMsg.ThreadID())
}

func handleReport(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	userData := chatMsg.UserData()
	if userData == nil {
		return fmt.Errorf("user data is nil")
	}
	log.Printf("Got an admin command from %d", userData.UserID)

	text := textcases.GetAdminsCommand(chatMsg.Appeal(), chatMessageHandler.AdminsUsernames)
	if chatMsg.IsReply() {
		return messages.ReplyToOriginalMessage(c, text, chatMsg.ThreadID())
	} else {
		return messages.ReplyMessage(c, text, chatMsg.ThreadID())
	}
}

func handleWarns(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	userData := chatMsg.UserData()
	if userData == nil {
		return fmt.Errorf("user data is nil")
	}
	warns := userData.Warns

	switch {
	case warns == 0:
		return messages.ReplyMessage(c, "Тебя ещё не предупреждали? Срочно предупредите его!", chatMsg.ThreadID())
	case warns > 0 && warns < 10:
		return messages.ReplyMessage(c, fmt.Sprintf("У тебя %d предупреждений. Помни, предупрежден — значит предупрежден", warns), chatMsg.ThreadID())
	case warns >= 10 && warns < 100:
		return messages.ReplyMessage(c, fmt.Sprintf("У тебя %d предупреждений. Этот парень совсем слов не понимает?", warns), chatMsg.ThreadID())
	case warns >= 100 && warns < 1000:
		return messages.ReplyMessage(c, fmt.Sprintf("У тебя %d предупреждений. Я от тебя в светлом ахуе. Ты когда-нибудь перестанешь?", warns), chatMsg.ThreadID())
	case warns >= 1000:
		return messages.ReplyMessage(c, fmt.Sprintf("У тебя %d предупреждений. Ты постиг нирвану и вышел за пределы сознания. Тебя больше ничто не остановит", warns), chatMsg.ThreadID())
	}

	return nil
}
