package handlers

import (
	"errors"
	"fmt"
	"log"
	"saxbot/admins"
	"saxbot/database"
	"saxbot/messages"
	textcases "saxbot/text_cases"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"
)

// isBirthdayFormat проверяет, соответствует ли строка формату даты DD.MM.YYYY
func isBirthdayFormat(text string) bool {
	if len(text) != 10 {
		return false
	}
	// Простая проверка формата: DD.MM.YYYY (10 символов)
	// Проверяем, что на позициях 2 и 5 стоят точки
	if text[2] != '.' || text[5] != '.' {
		return false
	}
	// Пытаемся распарсить дату
	_, err := time.Parse("02.01.2006", text)
	return err == nil
}

func handleWarn(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	if !chatMsg.IsReply() {
		return messages.ReplyMessage(c, "Ты кого предупреждаешь?", chatMsg.ThreadID())
	}

	replyToID := chatMsg.ReplyToID()

	// Проверяем, является ли ReplyTo каналом
	if chatMsg.ReplyToIsChannel() {
		// Для каналов используем UpdateChannelWarns
		if err := chatMessageHandler.Rep.UpdateChannelWarns(replyToID, 1); err != nil {
			log.Printf("Failed to save warns increase for channel %d: %v", replyToID, err)
		}
	} else {
		// Для пользователей используем UpdateUserWarns
		if err := chatMessageHandler.Rep.UpdateUserWarns(replyToID, 1); err != nil {
			log.Printf("Failed to save warns increase for user %d: %v", replyToID, err)
		} else {
			replyToUserData := chatMsg.ReplyToUserData()
			if replyToUserData != nil {
				replyToUserData.Warns++
			}
		}
	}

	var text string
	replyTo := chatMsg.ReplyTo()
	if replyTo != nil {
		text = textcases.GetWarnCase(chatMsg.ReplyToAppeal())
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

	// Проверяем, является ли ReplyTo каналом
	if chatMsg.ReplyToIsChannel() {
		// Для каналов только меняем статус в БД
		channelID := chatMsg.ReplyToChannel().ID
		channelData, err := chatMessageHandler.Rep.GetChannel(channelID)
		if err != nil {
			return fmt.Errorf("failed to get channel data for channel %d: %w", channelID, err)
		}
		channelData.Status = "banned"
		if err := chatMessageHandler.Rep.SaveChannel(&channelData); err != nil {
			return fmt.Errorf("failed to ban channel %d: %w", channelID, err)
		}
		chatMessageHandler.Bot.Delete(chatMsg.ReplyTo())
		return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
	}

	// Обработка бана пользователя
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

func handleUnban(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	if !chatMsg.IsReply() {
		return messages.ReplyMessage(c, "С кого бан снять?", chatMsg.ThreadID())
	}

	// Проверяем, является ли ReplyTo каналом
	if chatMsg.ReplyToIsChannel() {
		// Для каналов только меняем статус в БД
		channelID := chatMsg.ReplyToChannel().ID
		channelData, err := chatMessageHandler.Rep.GetChannel(channelID)
		if err != nil {
			return fmt.Errorf("failed to get channel data for channel %d: %w", channelID, err)
		}
		channelData.Status = "active"
		if err := chatMessageHandler.Rep.SaveChannel(&channelData); err != nil {
			return fmt.Errorf("failed to unban channel %d: %w", channelID, err)
		}
		return messages.ReplyMessage(c, fmt.Sprintf("%s помилован. Больше не шали!", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
	}

	// Обработка разбана пользователя
	replyTo := chatMsg.ReplyTo()
	if replyTo == nil || replyTo.Sender == nil {
		return fmt.Errorf("reply message or sender is nil")
	}

	user := replyTo.Sender
	admins.UnbanUser(chatMessageHandler.Bot, c.Message().Chat, user, chatMessageHandler.Rep)
	return messages.ReplyMessage(c, fmt.Sprintf("%s помилован. Больше не шали!", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
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

	// Проверяем, является ли ReplyTo каналом
	if chatMsg.ReplyToIsChannel() {
		// Для каналов только меняем статус в БД
		channelID := chatMsg.ReplyToChannel().ID
		channelData, err := chatMessageHandler.Rep.GetChannel(channelID)
		if err != nil {
			return fmt.Errorf("failed to get channel data for channel %d: %w", channelID, err)
		}
		channelData.Status = "restricted"
		if err := chatMessageHandler.Rep.SaveChannel(&channelData); err != nil {
			return fmt.Errorf("failed to restrict channel %d: %w", channelID, err)
		}
		return messages.ReplyMessage(c, fmt.Sprintf("%s рестрикнут. Даже я словил кринж. А я бот ваще-то", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
	}

	// Обработка рестрикта пользователя
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

	// Проверяем, является ли ReplyTo каналом
	if chatMsg.ReplyToIsChannel() {
		// Для каналов только меняем статус в БД
		channelID := chatMsg.ReplyToChannel().ID
		channelData, err := chatMessageHandler.Rep.GetChannel(channelID)
		if err != nil {
			return fmt.Errorf("failed to get channel data for channel %d: %w", channelID, err)
		}
		channelData.Status = "active"
		if err := chatMessageHandler.Rep.SaveChannel(&channelData); err != nil {
			return fmt.Errorf("failed to unmute channel %d: %w", channelID, err)
		}
		return messages.ReplyMessage(c, fmt.Sprintf("%s размучен. А то че как воды в рот набрал", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
	}

	// Обработка размута пользователя
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

	// Проверяем, является ли ReplyTo каналом
	if chatMsg.ReplyToIsChannel() {
		// Для каналов только меняем статус в БД и запускаем горутину для автоматического размута
		channelID := chatMsg.ReplyToChannel().ID
		channelData, err := chatMessageHandler.Rep.GetChannel(channelID)
		if err != nil {
			return fmt.Errorf("failed to get channel data for channel %d: %w", channelID, err)
		}
		now := time.Now().In(database.MoscowTZ)
		channelData.Status = "muted"
		channelData.MutedUntil = now.Add(time.Duration(durationMinutes) * time.Minute)
		if err := chatMessageHandler.Rep.SaveChannel(&channelData); err != nil {
			return fmt.Errorf("failed to mute channel %d: %w", channelID, err)
		}

		return messages.ReplyMessage(c, fmt.Sprintf("%s помолчит %d минут и подумает о своем поведении", chatMsg.ReplyToAppeal(), durationMinutes), chatMsg.ThreadID())
	}

	// Обработка мута пользователя
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

	// Проверяем, является ли ReplyTo каналом
	if chatMsg.ReplyToIsChannel() {
		// Для каналов только меняем статус в БД
		channelID := chatMsg.ReplyToChannel().ID
		channelData, err := chatMessageHandler.Rep.GetChannel(channelID)
		if err != nil {
			return fmt.Errorf("failed to get channel data for channel %d: %w", channelID, err)
		}
		messages.ReplyToOriginalMessage(c, fmt.Sprintf("%s, скажи ауфидерзейн своим нацистским яйцам!", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
		time.Sleep(1 * time.Second)
		channelData.Status = "banned"
		if err := chatMessageHandler.Rep.SaveChannel(&channelData); err != nil {
			return fmt.Errorf("failed to ban channel %d: %w", channelID, err)
		}
		chatMessageHandler.Bot.Delete(chatMsg.ReplyTo())
		return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
	}

	// Обработка бана пользователя
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

	// Проверяем, является ли ReplyTo каналом
	if chatMsg.ReplyToIsChannel() {
		// Для каналов только меняем статус в БД
		channelID := chatMsg.ReplyToChannel().ID
		channelData, err := chatMessageHandler.Rep.GetChannel(channelID)
		if err != nil {
			return fmt.Errorf("failed to get channel data for channel %d: %w", channelID, err)
		}
		messages.ReplyToOriginalMessage(c, "ОБЕЗГЛАВИТЬ ОБОССАТЬ И СЖЕЧЬ!!!", chatMsg.ThreadID())
		time.Sleep(1 * time.Second)
		channelData.Status = "banned"
		if err := chatMessageHandler.Rep.SaveChannel(&channelData); err != nil {
			return fmt.Errorf("failed to ban channel %d: %w", channelID, err)
		}
		chatMessageHandler.Bot.Delete(chatMsg.ReplyTo())
		return messages.ReplyMessage(c, fmt.Sprintf("%s идет нахуй из чатика. АВЕ АВЕ ПИРОМАН!", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
	}

	// Обработка бана пользователя
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

	var senderID int64
	if chatMsg.IsFromChannel() {
		senderID = chatMsg.Channel().ID
	} else {
		userData := chatMsg.UserData()
		if userData == nil {
			return fmt.Errorf("user data is nil")
		}
		senderID = userData.UserID
	}
	log.Printf("Got an admin command from %d", senderID)

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

	var warns int
	if chatMsg.IsFromChannel() {
		channelData := chatMsg.ChannelData()
		if channelData == nil {
			return fmt.Errorf("channel data is nil")
		}
		warns = channelData.Warns
	} else {
		userData := chatMsg.UserData()
		if userData == nil {
			return fmt.Errorf("user data is nil")
		}
		warns = userData.Warns
	}

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

func handleNotEnoughRights(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	return messages.ReplyMessage(c, "У тебя недостаточно прав для выполнения этой команды.", chatMsg.ThreadID())
}

func handleKick(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	if !chatMsg.IsReply() {
		return messages.ReplyMessage(c, "Кого мне кикать?", chatMsg.ThreadID())
	}

	if chatMsg.ReplyToAdmin() {
		return messages.ReplyMessage(c, "Ты не можешь кикать других админов, соси писос", chatMsg.ThreadID())
	}

	// Проверяем, является ли ReplyTo каналом
	if chatMsg.ReplyToIsChannel() {
		// Для каналов кик не имеет смысла, так как канал нельзя кикнуть из чата
		// Вместо этого баним канал
		channelID := chatMsg.ReplyToChannel().ID
		channelData, err := chatMessageHandler.Rep.GetChannel(channelID)
		if err != nil {
			return fmt.Errorf("failed to get channel data for channel %d: %w", channelID, err)
		}
		channelData.Status = "banned"
		if err := chatMessageHandler.Rep.SaveChannel(&channelData); err != nil {
			return fmt.Errorf("failed to ban channel %d: %w", channelID, err)
		}
		return messages.ReplyMessage(c, fmt.Sprintf("%s покидает нас", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
	}

	// Обработка кика пользователя
	replyTo := chatMsg.ReplyTo()
	if replyTo == nil || replyTo.Sender == nil {
		return fmt.Errorf("reply message or sender is nil")
	}

	user := replyTo.Sender
	chatMember := &tele.ChatMember{User: user, Role: tele.Member}
	err := admins.KickUser(chatMessageHandler.Bot, chatMsg.Chat(), chatMember)
	if err != nil {
		return fmt.Errorf("can't kick user %d: %w", user.ID, err)
	}
	return messages.ReplyMessage(c, fmt.Sprintf("%s покидает нас", chatMsg.ReplyToAppeal()), chatMsg.ThreadID())
}

// Обработка команды "Предупредить всех" (просто прикольное сообщение в чат)
func handleWarnAll(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	return messages.ReplyFormattedHTML(c, "Неужели мало сегодняшних жертв? Надо, чтобы в чатике продолжали сраться и страдать тысячи людей? <b>Астанавитесь!</b> Всем предупреждение!", chatMsg.ThreadID())
}

// Обработка команды показать информацию по сегодняшнему квизу (для админов)
func handleShowQuizInfo(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	time := chatMessageHandler.QuizManager.TodayQuiz.QuizTime.In(database.MoscowTZ).Format("15:04")
	text := fmt.Sprintf("Информация о сегодняшнем квизе:\nВремя проведения: %s\n", time)
	if chatMessageHandler.QuizManager.QuizAlreadyWas {
		winnerID := chatMessageHandler.QuizManager.WinnerID
		winner, err := chatMessageHandler.Rep.GetUser(winnerID)
		if err != nil {
			return c.Send("Внутренняя ошибка базы данных. Попробуй еще раз")
		}
		text = text + fmt.Sprintf("Квиз сегодня уже был проведен\nПобедитель: @%s (ID %d)\n", winner.Username, winner.UserID)
	} else {
		text = text + "Квиза сегодня ещё не было\n"
	}
	answer := chatMessageHandler.QuizManager.TodayQuiz.SongName
	if chatMessageHandler.QuizManager.TodayQuiz.IsClip {
		text = text + fmt.Sprintf("Сегодня кадр из клипа\nОтвет: %s", answer)
	} else {
		text = text + fmt.Sprintf("Сегодня цитата из песни\nОтвет: %s", answer)
	}
	return c.Send(text)
}

func handleUnwarn(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	chatMsg := chatMessageHandler.ChatMessage
	if chatMsg == nil {
		return fmt.Errorf("chat message is nil")
	}
	if !chatMsg.IsReply() {
		return messages.ReplyMessage(c, "Кого лишить предупреждения?", chatMsg.ThreadID())
	}

	replyToID := chatMsg.ReplyToID()

	// Проверяем, является ли ReplyTo каналом
	if chatMsg.ReplyToIsChannel() {
		// Для каналов используем UpdateChannelWarns
		if err := chatMessageHandler.Rep.UpdateChannelWarns(replyToID, -1); err != nil {
			log.Printf("Failed to save warns increase for channel %d: %v", replyToID, err)
		}
	} else {
		// Для пользователей используем UpdateUserWarns
		if err := chatMessageHandler.Rep.UpdateUserWarns(replyToID, -1); err != nil {
			log.Printf("Failed to save warns increase for user %d: %v", replyToID, err)
		} else {
			replyToUserData := chatMsg.ReplyToUserData()
			if replyToUserData != nil {
				replyToUserData.Warns++
			}
		}
	}

	text := fmt.Sprintf("%s лишается нажитого непосильным трудом предупреждения. Это надо было серьезно разозлить админа!", chatMsg.ReplyToAppeal())
	return messages.ReplyToOriginalMessage(c, text, chatMsg.ThreadID())
}

func handleCondemn(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	text := textcases.GetCondemnMessage(chatMessageHandler.ChatMessage.ReplyToAppeal())
	return messages.ReplyToOriginalMessage(c, text, chatMessageHandler.ChatMessage.ThreadID())
}

func handlePromoteAdmin(c tele.Context, chatMessageHandler *ChatMessageHandler) error {
	msg := chatMessageHandler.ChatMessage
	if msg == nil {
		return errors.New("chat message is nil")
	}
	parts := strings.Split(msg.Text(), " ")
	if len(parts) != 2 {
		c.Reply("Неправильный формат команды (len)")
		return fmt.Errorf("got unexpected len %d, expected 2", len(parts))
	}
	idStr := parts[1]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.Reply("Неправильный формат команды (Atoi)")
		return fmt.Errorf("couldn't convert ID to int, %w", err)
	}
	id64 := int64(id)
	err = chatMessageHandler.Rep.PromoteAdmin(id64, "+")
	if err != nil {
		c.Reply("Внутренняя ошибка базы данных")
		return fmt.Errorf("couldn't promote admin %d: %w", id64, err)
	}
	return c.Reply(fmt.Sprintf("Успешно продвинули админа %d", id64))
}
