package telegram

import tele "gopkg.in/telebot.v4"

// IsChatMember проверяет, состоит ли пользователь в чате.
// Ошибка API возвращается отдельно от отсутствия пользователя в чате.
func IsChatMember(bot *tele.Bot, chatID, userID int64) (bool, error) {
	member, err := bot.ChatMemberOf(tele.ChatID(chatID), &tele.User{ID: userID})
	if err != nil {
		return false, err
	}

	switch member.Role {
	case tele.Creator, tele.Administrator, tele.Member:
		return true, nil
	case tele.Restricted:
		return member.Member, nil
	default:
		return false, nil
	}
}
